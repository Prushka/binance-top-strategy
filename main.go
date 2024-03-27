package main

import (
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())

var globalStrategies = make(map[int]*Strategy) // StrategyOriginalID -> Strategy
var gGrids = newTrackedGrids()
var sessionSymbolPrice = make(map[string]float64)

func getSessionSymbolPrice(symbol string) (float64, error) {
	if _, ok := sessionSymbolPrice[symbol]; !ok {
		marketPrice, err := fetchMarketPrice(symbol)
		if err != nil {
			return 0, err
		}
		sessionSymbolPrice[symbol] = marketPrice
	}
	return sessionSymbolPrice[symbol], nil
}

type StrategiesBundle struct {
	Raw      *TrackedStrategies
	Filtered *TrackedStrategies
}

func getTopStrategiesWithRoi() (*StrategiesBundle, error) {
	strategies, err := getTopStrategies(FUTURE)
	if err != nil {
		return nil, err
	}
	filtered := make(Strategies, 0)
	for c, s := range strategies.strategies {
		id := s.SID
		roi, err := RoisCache.Get(fmt.Sprintf("%d-%d", id, s.UserID))
		if err != nil {
			return nil, err
		}
		s.Rois = roi
		s.roi, _ = strconv.ParseFloat(s.Roi, 64)
		s.roi /= 100

		lower, _ := strconv.ParseFloat(s.StrategyParams.LowerLimit, 64)
		upper, _ := strconv.ParseFloat(s.StrategyParams.UpperLimit, 64)
		s.priceDifference = upper/lower - 1

		if len(s.Rois) > 1 {
			s.roi = s.Rois[0].Roi
			s.lastDayRoiChange = GetRoiChange(s.Rois, 24*time.Hour)
			s.last3HrRoiChange = GetRoiChange(s.Rois, 3*time.Hour)
			s.last2HrRoiChange = GetRoiChange(s.Rois, 2*time.Hour)
			s.lastHrRoiChange = GetRoiChange(s.Rois, 1*time.Hour)
			s.lastDayRoiPerHr = GetRoiPerHr(s.Rois, 24*time.Hour)
			s.last12HrRoiPerHr = GetRoiPerHr(s.Rois, 12*time.Hour)
			s.last6HrNoDip = NoDip(s.Rois, 6*time.Hour)
			s.roiPerHour = (s.roi - s.Rois[len(s.Rois)-1].Roi) / float64(s.RunningTime/3600)
			prefix := ""
			if s.lastDayRoiChange > 0.1 &&
				s.last3HrRoiChange > 0.03 &&
				s.lastHrRoiChange > 0.01 &&
				s.last2HrRoiChange > s.lastHrRoiChange &&
				s.lastDayRoiPerHr > 0.01 &&
				s.last12HrRoiPerHr > 0.014 &&
				s.priceDifference > 0.05 &&
				s.last6HrNoDip {
				filtered = append(filtered, s)
				prefix += "Open"
			}
			log.Info(prefix + display(s, nil, "Found", c+1, len(strategies.strategiesById)))
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		I := filtered[i]
		J := filtered[j]
		return I.last12HrRoiPerHr > J.last12HrRoiPerHr
	})
	bundle := &StrategiesBundle{Raw: strategies, Filtered: filtered.toTrackedStrategies()}
	DiscordWebhook("### Strategies")
	DiscordWebhook("Raw: " + bundle.Raw.String())
	DiscordWebhook("Open: " + bundle.Filtered.String())
	return bundle, nil
}

func GetRoiChange(roi StrategyRoi, t time.Duration) float64 {
	latestTimestamp := roi[0].Time
	latestRoi := roi[0].Roi
	l := latestTimestamp - int64(t.Seconds())
	for _, r := range roi {
		if r.Time <= l {
			return latestRoi - r.Roi
		}
	}
	return latestRoi - roi[len(roi)-1].Roi
}

func GetRoiPerHr(roi StrategyRoi, t time.Duration) float64 {
	latestTimestamp := roi[0].Time
	latestRoi := roi[0].Roi
	l := latestTimestamp - int64(t.Seconds())
	hrs := float64(t.Seconds()) / 3600
	for _, r := range roi {
		if r.Time <= l {
			return (latestRoi - r.Roi) / hrs
		}
	}
	return (latestRoi - roi[len(roi)-1].Roi) / (float64(roi[0].Time-roi[len(roi)-1].Time) / 3600)
}

func NoDip(roi StrategyRoi, t time.Duration) bool {
	latestTimestamp := roi[0].Time
	l := latestTimestamp - int64(t.Seconds())
	for c, r := range roi {
		if r.Time < l {
			return true
		}
		if c > 0 && roi[c-1].Roi-r.Roi < 0 {
			return false
		}
	}
	return true
}

func tick() error {
	ResetTime()
	clear(sessionSymbolPrice)
	DiscordWebhook(fmt.Sprintf("## Run: %v", time.Now().Format("2006-01-02 15:04:05")))
	usdt, err := getFutureUSDT()
	if err != nil {
		return err
	}
	bundle, err := getTopStrategiesWithRoi()
	if err != nil {
		return err
	}

	Time("Fetch strategies")
	clear(sessionSymbolPrice)
	DiscordWebhook("### Current Grids:")
	err = updateOpenGrids(true)
	if err != nil {
		return err
	}

	Time("Fetch grids")
	count := 0
	for _, grid := range gGrids.gridsByUid {
		sid := grid.SID
		DiscordWebhook(display(globalStrategies[sid], grid,
			fmt.Sprintf("%d, %d", bundle.Raw.findStrategyRanking(sid), bundle.Filtered.findStrategyRanking(sid)),
			count+1, len(gGrids.gridsByUid)))
		count++
	}
	expiredCopiedIds := gGrids.existingIds.Difference(bundle.Filtered.ids)
	for _, grid := range gGrids.gridsByUid {
		if !expiredCopiedIds.Contains(grid.SID) {
			symbolDifferentDirectionsHigherRanking := 0
			for _, s := range bundle.Filtered.strategies {
				if s.Symbol == grid.Symbol {
					if DirectionMap[s.Direction] != grid.Direction {
						symbolDifferentDirectionsHigherRanking++
					} else {
						break
					}
				}
			}
			if symbolDifferentDirectionsHigherRanking >= 2 {
				expiredCopiedIds.Add(grid.SID)
				DiscordWebhook(display(globalStrategies[grid.SID], grid,
					fmt.Sprintf("**Opposite directions at top: %d**", symbolDifferentDirectionsHigherRanking),
					0, 0))
			}
		}
	}
	if expiredCopiedIds.Cardinality() > 0 {
		DiscordWebhook(fmt.Sprintf("### Expired Strategies: %v", expiredCopiedIds))
	}
	closedIds := mapset.NewSet[int]()
	expiredGridIds := gGrids.findGridIdsByStrategyId(expiredCopiedIds.ToSlice()...)
	for c, id := range expiredGridIds.ToSlice() {
		reason := ""
		grid := gGrids.gridsByUid[id]
		strategyId := grid.SID
		att, ok := globalStrategies[strategyId]
		maxCancelLoss := TheConfig.MaxCancelLoss
		if !bundle.Raw.exists(strategyId) {
			reason += "Strategy not found"
			maxCancelLoss = TheConfig.MaxCancelLostStrategyDeleted
		} else if ok && !bundle.Filtered.exists(strategyId) {
			reason += "Strategy not picked"
		}
		if grid.lastRoi < maxCancelLoss {
			reason += " too much loss"
			DiscordWebhookS(display(att, grid, "**Skip Cancel "+reason+"**", c+1, expiredCopiedIds.Cardinality()), ActionWebhook, DefaultWebhook)
			continue
		}
		err := closeGrid(id)
		if err != nil {
			return err
		}
		closedIds.Add(id)
		DiscordWebhookS(display(att, grid, "**Cancelled "+reason+"**", c+1, expiredCopiedIds.Cardinality()), ActionWebhook, DefaultWebhook)
	}

	for _, grid := range gGrids.gridsByUid {
		if grid.tracking.continuousRoiNoChange > 3 && grid.lastRoi >= 0 {
			err := closeGrid(grid.GID)
			if err != nil {
				return err
			}
			closedIds.Add(grid.GID)
			DiscordWebhookS(display(globalStrategies[grid.SID], grid, "**Cancelled No Change**",
				0, 0), ActionWebhook, DefaultWebhook)
		}
	}

	for _, grid := range gGrids.gridsByUid {
		if grid.lastRoi >= TheConfig.GainExitNotGoingUp {
			if time.Since(grid.tracking.timeHighestRoi) > time.Duration(TheConfig.GainExitNotGoingUpMaxLookBackMinutes)*time.Minute {
				err := closeGrid(grid.GID)
				if err != nil {
					return err
				}
				closedIds.Add(grid.GID)
				DiscordWebhookS(display(globalStrategies[grid.SID], grid,
					fmt.Sprintf("Cancelled, max gain %.2f%%/%.2f%%, reached %s ago",
						grid.lastRoi*100, grid.tracking.highestRoi*100,
						time.Since(grid.tracking.timeHighestRoi).Round(time.Second)),
					0, 0), ActionWebhook, DefaultWebhook)
			}
		}
	}

	if closedIds.Cardinality() > 0 && !TheConfig.Paper {
		DiscordWebhook("Cleared expired grids - Skip current run")
		return nil
	}

	gridsOpen := len(gGrids.gridsByUid)
	if TheConfig.MaxChunks-gridsOpen <= 0 && !TheConfig.Paper {
		DiscordWebhook("Max Chunks reached, No cancel - Skip current run")
		return nil
	}
	if mapset.NewSetFromMapKeys(bundle.Filtered.symbolCount).Difference(gGrids.existingSymbols).Cardinality() == 0 {
		DiscordWebhook("All symbols exists in open grids, Skip")
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
	sessionSymbols := gGrids.existingSymbols.Clone()
	for c, s := range bundle.Filtered.strategies {
		DiscordWebhook(display(s, nil, "New", c+1, len(bundle.Filtered.strategies)))
		if gGrids.existingIds.Contains(s.SID) {
			DiscordWebhook("Strategy exists in open grids, Skip")
			continue
		}
		if sessionSymbols.Contains(s.Symbol) {
			DiscordWebhook("Symbol exists in open grids, Skip")
			continue
		}

		if s.StrategyParams.TriggerPrice != nil {
			triggerPrice, _ := strconv.ParseFloat(*s.StrategyParams.TriggerPrice, 64)
			marketPrice, _ := getSessionSymbolPrice(s.Symbol)
			diff := math.Abs((triggerPrice - marketPrice) / marketPrice)
			if diff > 0.08 {
				DiscordWebhook(fmt.Sprintf("Trigger Price difference too high, Skip, Trigger: %f, Market: %f, Diff: %f",
					triggerPrice, marketPrice, diff))
				continue
			}
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
			if strings.Contains(errr.Error(), "Create grid too frequently") {
				DiscordWebhook("**Too Frequent Error, Skip Current Run**")
				break
			}
		} else {
			DiscordWebhookS(display(s, nil, "**Opened Grid**", c+1, len(bundle.Filtered.strategies)), ActionWebhook, DefaultWebhook)
			chunksInt -= 1
			sessionSymbols.Add(s.Symbol)
			if chunksInt <= 0 {
				break
			}
		}
	}

	Time("Place/Cancel done")
	DiscordWebhook("### New Grids:")
	err = updateOpenGrids(false)
	if err != nil {
		return err
	}
	return nil
}

// TODO: cancel when symbolpair count decreases
//by 2 more
//2 and more
//then avoid that sp for how long?
//if increases again cancel blacklist

// cancel when above n%, then cooldown?
// perform last 20 min roi (latest - last 20 OR if max roi was reached more than 20 min ago), if not positive and stop gain, cancel then block symbolpairdirection until next hr
// sort by symbol pair in filtered

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
		_, err := scheduler.SingletonMode().Every(TheConfig.TickEveryMinutes).Minutes().Do(
			func() {
				t := time.Now()
				err := tick()
				if err != nil {
					log.Errorf("Error: %v", err)
				}
				DiscordWebhook(fmt.Sprintf("*Run took: %v*", time.Since(t)))
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
