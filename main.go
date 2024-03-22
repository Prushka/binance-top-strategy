package main

import (
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"sort"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())

var globalStrategies = make(map[int]*Strategy)
var globalGrids = make(map[int]*TrackedGrid)

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

		if len(s.Rois) > 1 {
			s.LastDayRoiChange = GetRoiChange(s.Rois, 24*time.Hour)
			s.Last3HrRoiChange = GetRoiChange(s.Rois, 3*time.Hour)
			s.Last2HrRoiChange = GetRoiChange(s.Rois, 2*time.Hour)
			s.LastHrRoiChange = GetRoiChange(s.Rois, 1*time.Hour)
			prefix := ""
			if s.LastDayRoiChange > 0.1 && s.Last3HrRoiChange > 0.05 && s.Last2HrRoiChange > 0 && s.LastHrRoiChange > -0.05 {
				allowKeep = append(allowKeep, s)
				prefix += "Keep "
				if s.Last2HrRoiChange > s.LastHrRoiChange && s.LastHrRoiChange > 0 {
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
		iWeight := I.Last3HrRoiChange*TheConfig.Last3HrWeight + I.Last2HrRoiChange*TheConfig.Last2HrWeight + I.LastHrRoiChange*TheConfig.LastHrWeight
		jWeight := J.Last3HrRoiChange*TheConfig.Last3HrWeight + J.Last2HrRoiChange*TheConfig.Last2HrWeight + J.LastHrRoiChange*TheConfig.LastHrWeight
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
	openGrids, err := getOpenGrids()
	if err != nil {
		return err
	}
	for _, grid := range openGrids.Data {
		grid.track()
	}
	for c, grid := range openGrids.Data {
		id := grid.CopiedStrategyID
		DiscordWebhook(display(globalStrategies[id], grid,
			fmt.Sprintf("%d, %d, %d", bundle.Raw.findStrategyRanking(id), bundle.AllowKeep.findStrategyRanking(id), bundle.AllowOpen.findStrategyRanking(id)),
			c+1, len(openGrids.Data)))
	}
	expiredCopiedIds := openGrids.existingIds.Difference(bundle.AllowKeep.ids)
	if expiredCopiedIds.Cardinality() > 0 {
		DiscordWebhook(fmt.Sprintf("### Expired Strategies: %v", expiredCopiedIds))
	}
	closedIds := mapset.NewSet[int]()
	for c, id := range expiredCopiedIds.ToSlice() {
		reason := ""
		att, ok := globalStrategies[id]
		if !bundle.Raw.exists(id) {
			reason += "Strategy not found"
		} else if ok && !bundle.AllowKeep.exists(id) {
			reason += "Strategy not picked"
		}
		log.Infof("Closing Grid: %d", id)
		tracked, ok := globalGrids[id]
		if ok && tracked.LastRoi < -0.03 { // attempting to close loss
			reason += " too much loss"
			DiscordWebhook(display(att, tracked.grid, "Skip Cancel "+reason, c+1, expiredCopiedIds.Cardinality()))
			continue
		}
		err := closeGridConv(id, openGrids)
		if err != nil {
			return err
		}
		closedIds.Add(id)
		DiscordWebhook(display(att, tracked.grid, "Cancelled "+reason, c+1, expiredCopiedIds.Cardinality()))
	}

	for _, grid := range openGrids.Data {
		t, ok := globalGrids[grid.CopiedStrategyID]
		if ok && t.ContinuousRoiNoChange > 3 && grid.profit >= 0 {
			err := closeGridConv(grid.CopiedStrategyID, openGrids)
			if err != nil {
				return err
			}
			closedIds.Add(grid.CopiedStrategyID)
			DiscordWebhook(display(globalStrategies[grid.CopiedStrategyID], t.grid, "Cancelled No Change", 0, 0))
		}
	}

	if closedIds.Cardinality() > 0 && !TheConfig.Paper {
		DiscordWebhook("Cleared expired grids - Skip current run")
		for _, id := range closedIds.ToSlice() {
			delete(globalGrids, id)
		}
		return nil
	}

	if TheConfig.MaxChunks-len(openGrids.Data) <= 0 && !TheConfig.Paper {
		DiscordWebhook("Max Chunks reached, No cancel - Skip current run")
		return nil
	}
	DiscordWebhook("### Opening new grids:")
	chunksInt := TheConfig.MaxChunks - len(openGrids.Data)
	chunks := float64(TheConfig.MaxChunks - len(openGrids.Data))
	invChunk := (usdt - chunks*0.8) / chunks
	idealInvChunk := (usdt + openGrids.totalGridPnl + openGrids.totalGridInitial) / float64(TheConfig.MaxChunks)
	log.Infof("Ideal Investment: %f, allowed Investment: %f, missing %f chunks", idealInvChunk, invChunk, chunks)
	if invChunk > idealInvChunk {
		invChunk = idealInvChunk
	}
	for c, s := range bundle.AllowOpen.strategies {
		DiscordWebhook(display(s, nil, "New", c+1, len(bundle.AllowOpen.strategies)))
		if openGrids.existingIds.Contains(s.StrategyID) {
			DiscordWebhook("Strategy exists in open grids, Skip")
			continue
		}
		if openGrids.existingPairs.Contains(s.Symbol) {
			DiscordWebhook("Symbol exists in open grids, Skip")
			continue
		}
		switch s.Direction {
		case LONG:
			if TheConfig.MaxLongs >= 0 && openGrids.totalLongs >= TheConfig.MaxLongs {
				DiscordWebhook("Max Longs reached, Skip")
				continue
			}
		case NEUTRAL:
			if TheConfig.MaxNeutrals >= 0 && openGrids.totalNeutrals >= TheConfig.MaxNeutrals {
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
			openGrids.existingPairs.Add(s.Symbol)
			if chunksInt <= 0 {
				break
			}
		}
	}

	DiscordWebhook("### Opened Grids:")
	newOpenGrids, err := getOpenGrids()
	if err != nil {
		return err
	}
	for _, newId := range newOpenGrids.existingIds.Difference(openGrids.existingIds).ToSlice() {
		DiscordWebhook(display(globalStrategies[newId], nil, "Opened", 0, 0))
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
