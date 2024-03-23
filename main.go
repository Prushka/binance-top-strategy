package main

import (
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"sort"
	"strconv"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())

var globalStrategies = make(map[int]*Strategy) // StrategyOriginalID -> Strategy
var gGrids = newTrackedGrids()

type StrategiesBundle struct {
	Raw       *TrackedStrategies
	AllowOpen *TrackedStrategies
	AllowKeep *TrackedStrategies
}

func getTopStrategiesWithRoi() (*StrategiesBundle, error) {
	strategies, err := getTopStrategies(FUTURE, time.Duration(TheConfig.RuntimeMinHours)*time.Hour, time.Duration(TheConfig.RuntimeMaxHours)*time.Hour)
	if err != nil {
		return nil, err
	}
	allowKeep := make(Strategies, 0)
	allowOpen := make(Strategies, 0)
	for c, s := range strategies.strategies {
		id := s.StrategyID
		roi, err := getStrategyRois(id, s.UserID)
		if err != nil {
			return nil, err
		}
		for _, r := range roi {
			r.Time = r.Time / 1000
		}
		sort.Slice(roi, func(i, j int) bool {
			return roi[i].Time > roi[j].Time
		})
		s.Rois = roi
		s.roi, _ = strconv.ParseFloat(s.Roi, 64)
		s.roi /= 100

		if len(s.Rois) > 1 {
			s.lastDayRoiChange = GetRoiChange(s.Rois, 24*time.Hour)
			s.last3HrRoiChange = GetRoiChange(s.Rois, 3*time.Hour)
			s.last2HrRoiChange = GetRoiChange(s.Rois, 2*time.Hour)
			s.lastHrRoiChange = GetRoiChange(s.Rois, 1*time.Hour)
			s.roiPerHour = (s.roi - s.Rois[len(s.Rois)-1].Roi) / float64(s.RunningTime/3600)
			prefix := ""
			if s.lastDayRoiChange > 0.1 && s.last3HrRoiChange > 0.05 && s.last2HrRoiChange > 0 && s.lastHrRoiChange > -0.05 {
				allowKeep = append(allowKeep, s)
				prefix += "Keep "
				if s.last2HrRoiChange > s.lastHrRoiChange && s.lastHrRoiChange > 0.01 {
					allowOpen = append(allowOpen, s)
					prefix += " Open "
				}
			}
			log.Info(prefix + display(s, nil, "Found", c+1, len(strategies.strategiesById)))
		}
	}
	sort.Slice(allowOpen, func(i, j int) bool {
		I := allowOpen[i]
		J := allowOpen[j]
		iWeight := I.last3HrRoiChange*TheConfig.Last3HrWeight + I.last2HrRoiChange*TheConfig.Last2HrWeight + I.lastHrRoiChange*TheConfig.LastHrWeight
		jWeight := J.last3HrRoiChange*TheConfig.Last3HrWeight + J.last2HrRoiChange*TheConfig.Last2HrWeight + J.lastHrRoiChange*TheConfig.LastHrWeight
		return iWeight > jWeight
	})
	bundle := &StrategiesBundle{Raw: strategies, AllowOpen: allowOpen.toTrackedStrategies(), AllowKeep: allowKeep.toTrackedStrategies()}
	DiscordWebhook("### Strategies")
	DiscordWebhook("Raw: " + bundle.Raw.String())
	DiscordWebhook("Keep: " + bundle.AllowKeep.String())
	DiscordWebhook("Open: " + bundle.AllowOpen.String())
	return bundle, nil
}

func GetRoiChange(roi StrategyRoi, t time.Duration) float64 {
	latestTimestamp := roi[0].Time
	latestRoi := roi[0].Roi
	for _, r := range roi {
		l := latestTimestamp - int64(t.Seconds())
		if r.Time <= l {
			return latestRoi - r.Roi
		}
	}
	return latestRoi - roi[len(roi)-1].Roi
}

func tick() error {
	DiscordWebhook(fmt.Sprintf("## Run: %v", time.Now().Format("2006-01-02 15:04:05")))
	usdt, err := getFutureUSDT()
	if err != nil {
		return err
	}
	log.Infof("USDT: %f", usdt)
	bundle, err := getTopStrategiesWithRoi()
	if err != nil {
		return err
	}
	DiscordWebhook("### Current Grids:")
	err = updateOpenGrids(true)
	if err != nil {
		return err
	}
	count := 0
	for _, grid := range gGrids.gridsByUid {
		id := grid.CopiedStrategyID
		DiscordWebhook(display(globalStrategies[id], grid,
			fmt.Sprintf("%d, %d", bundle.Raw.findStrategyRanking(id), bundle.AllowOpen.findStrategyRanking(id)),
			count+1, len(gGrids.gridsByUid)))
		count++
	}
	expiredCopiedIds := gGrids.existingIds.Difference(bundle.AllowOpen.ids)
	if expiredCopiedIds.Cardinality() > 0 {
		DiscordWebhook(fmt.Sprintf("### Expired Strategies: %v", expiredCopiedIds))
	}
	closedIds := mapset.NewSet[int]()
	expiredGridIds := gGrids.findGridIdsByStrategyId(expiredCopiedIds.ToSlice()...)
	for c, id := range expiredGridIds.ToSlice() {
		reason := ""
		grid := gGrids.gridsByUid[id]
		att, ok := globalStrategies[id]
		if !bundle.Raw.exists(id) {
			reason += "Strategy not found"
		} else if ok && !bundle.AllowOpen.exists(id) {
			reason += "Strategy not picked"
		}
		log.Infof("Closing Grid with Strategy Id: %d", id)
		if grid.lastRoi < TheConfig.MaxCancelLoss {
			reason += " too much loss"
			DiscordWebhook(display(att, grid, "Skip Cancel "+reason, c+1, expiredCopiedIds.Cardinality()))
			continue
		}
		err := closeGrid(id)
		if err != nil {
			return err
		}
		closedIds.Add(id)
		DiscordWebhook(display(att, grid, "Cancelled "+reason, c+1, expiredCopiedIds.Cardinality()))
	}

	for _, grid := range gGrids.gridsByUid {
		if grid.continuousRoiNoChange > 3 && grid.lastRoi >= 0 {
			err := closeGrid(grid.StrategyID)
			if err != nil {
				return err
			}
			closedIds.Add(grid.StrategyID)
			DiscordWebhook(display(globalStrategies[grid.CopiedStrategyID], grid, "Cancelled No Change", 0, 0))
		}
	}

	if closedIds.Cardinality() > 0 && !TheConfig.Paper {
		DiscordWebhook("Cleared expired grids - Skip current run")
		for _, id := range closedIds.ToSlice() {
			gGrids.Remove(id)
		}
		return nil
	}

	gridsOpen := len(gGrids.gridsByUid)
	if TheConfig.MaxChunks-gridsOpen <= 0 && !TheConfig.Paper {
		DiscordWebhook("Max Chunks reached, No cancel - Skip current run")
		return nil
	}
	DiscordWebhook("### Opening new grids:")
	chunksInt := TheConfig.MaxChunks - gridsOpen
	chunks := float64(TheConfig.MaxChunks - gridsOpen)
	invChunk := (usdt - chunks*0.8) / chunks
	idealInvChunk := (usdt + gGrids.totalGridPnl + gGrids.totalGridInitial) / float64(TheConfig.MaxChunks)
	log.Infof("Ideal Investment: %f, allowed Investment: %f, missing %f chunks", idealInvChunk, invChunk, chunks)
	if invChunk > idealInvChunk {
		invChunk = idealInvChunk
	}
	for c, s := range bundle.AllowOpen.strategies {
		DiscordWebhook(display(s, nil, "New", c+1, len(bundle.AllowOpen.strategies)))
		if gGrids.existingIds.Contains(s.StrategyID) {
			DiscordWebhook("Strategy exists in open grids, Skip")
			continue
		}
		if gGrids.existingPairs.Contains(s.Symbol) {
			DiscordWebhook("Symbol exists in open grids, Skip")
			continue
		}
		switch s.Direction {
		case LONG:
			if TheConfig.MaxLongs >= 0 && gGrids.longs.Cardinality() >= TheConfig.MaxLongs {
				DiscordWebhook("Max Longs reached, Skip")
				continue
			}
		case NEUTRAL:
			if TheConfig.MaxNeutrals >= 0 && gGrids.shorts.Cardinality() >= TheConfig.MaxNeutrals {
				DiscordWebhook("Max Neutrals not reached, Skip")
				continue
			}
		}
		errr := placeGrid(*s, invChunk)
		if TheConfig.Paper {

		} else if errr != nil {
			DiscordWebhook(fmt.Sprintf("**Error placing grid: %v**", errr))
		} else {
			DiscordWebhook(fmt.Sprintf("**Placed grid**"))
			chunksInt -= 1
			gGrids.existingPairs.Add(s.Symbol)
			if chunksInt <= 0 {
				break
			}
		}
	}

	DiscordWebhook("### New Grids:")
	err = updateOpenGrids(false)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	configure()
	log.Infof("Public IP: %s", getPublicIP())
	DiscordService()
	switch TheConfig.Mode {
	case "trading":
		if TheConfig.Paper {
			DiscordWebhook("Paper Trading")
		} else {
			DiscordWebhook("Real Trading")
		}
		sdk()
		_, err := scheduler.Every(TheConfig.TickEveryMinutes).Minutes().Do(
			func() {
				err := tick()
				if err != nil {
					log.Errorf("Error: %v", err)
				}
			},
		)
		if err != nil {
			log.Errorf("Error: %v", err)
			return
		}
		scheduler.StartBlocking()

	case "extract-cookies":

	}
}
