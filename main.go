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
var tradingBlock = time.Now()
var bundle *StrategiesBundle

type SDCountPair struct {
	SymbolDirection string
	Count           int
	MaxMetric       float64
}

type StrategiesBundle struct {
	Raw      *TrackedStrategies
	Filtered *TrackedStrategies
}

type StateOnGridOpen struct {
	SymbolDirectionCount map[string]int
}

var statesOnGridOpen = make(map[int]*StateOnGridOpen)

func gridSDCount(gid int, symbol, direction string) (int, int, float64) {
	sd := symbol + direction
	currentSDCount := bundle.Raw.symbolDirectionCount[sd]
	sdCountWhenOpen := statesOnGridOpen[gid].SymbolDirectionCount[sd]
	ratio := float64(currentSDCount) / float64(sdCountWhenOpen)
	return currentSDCount, sdCountWhenOpen, ratio
}

func persistStateOnGridOpen(gid int) {
	if _, ok := statesOnGridOpen[gid]; !ok {
		statesOnGridOpen[gid] = &StateOnGridOpen{SymbolDirectionCount: bundle.Raw.symbolDirectionCount}
		err := save(statesOnGridOpen, GridStatesFileName)
		if err != nil {
			Discordf("Error saving state on grid open: %v", err)
		}
	}
}

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
	filteredBySymbolDirection := make(map[string]Strategies)
	for _, s := range filtered {
		sd := s.Symbol + DirectionMap[s.Direction]
		if _, ok := filteredBySymbolDirection[sd]; !ok {
			filteredBySymbolDirection[sd] = make(Strategies, 0)
		}
		filteredBySymbolDirection[sd] = append(filteredBySymbolDirection[sd], s)
	}
	sdLengths := make([]SDCountPair, 0)
	for sd, s := range filteredBySymbolDirection {
		sdLengths = append(sdLengths, SDCountPair{SymbolDirection: sd, Count: len(s), MaxMetric: s[0].last12HrRoiPerHr})
	}
	sort.Slice(sdLengths, func(i, j int) bool {
		if sdLengths[i].Count == sdLengths[j].Count {
			return sdLengths[i].MaxMetric > sdLengths[j].MaxMetric
		}
		return sdLengths[i].Count > sdLengths[j].Count
	})
	sortedBySDCount := make(Strategies, 0)
	for _, sd := range sdLengths {
		sortedBySDCount = append(sortedBySDCount, filteredBySymbolDirection[sd.SymbolDirection]...)
	}
	bundle := &StrategiesBundle{Raw: strategies, Filtered: sortedBySDCount.toTrackedStrategies()}
	Discordf("### Strategies")
	Discordf("Raw: " + bundle.Raw.String())
	Discordf("Open: " + bundle.Filtered.String())
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
	Discordf("## Run: %v", time.Now().Format("2006-01-02 15:04:05"))
	usdt, err := getFutureUSDT()
	if err != nil {
		return err
	}
	bundle, err = getTopStrategiesWithRoi()
	if err != nil {
		return err
	}
	Time("Fetch strategies")
	clear(sessionSymbolPrice)
	Discordf("### Current Grids:")
	err = updateOpenGrids(true)
	if err != nil {
		return err
	}

	Time("Fetch grids")
	count := 0
	for _, grid := range gGrids.gridsByUid {
		sid := grid.SID
		Discordf(display(globalStrategies[sid], grid,
			fmt.Sprintf("%d, %d", bundle.Raw.findStrategyRanking(sid), bundle.Filtered.findStrategyRanking(sid)),
			count+1, len(gGrids.gridsByUid)))
		count++
	}
	expiredCopiedIds := gGrids.existingIds.Difference(bundle.Filtered.ids)
	for _, grid := range gGrids.gridsByUid {
		if !expiredCopiedIds.Contains(grid.SID) {
			// exit signal: outdated direction
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
				Discordf(display(globalStrategies[grid.SID], grid,
					fmt.Sprintf("**Opposite directions at top: %d**", symbolDifferentDirectionsHigherRanking),
					0, 0))
			}

			// exit signal: symbol direction shrunk in raw strategies
			_, _, ratio := gridSDCount(grid.GID, grid.Symbol, grid.Direction)
			if ratio < TheConfig.CancelSymbolDirectionShrink {
				expiredCopiedIds.Add(grid.SID)
				minutesTillNextHour := 60 - time.Now().Minute()
				blockDuration := 75 * time.Minute
				if minutesTillNextHour < 30 {
					blockDuration = time.Duration(minutesTillNextHour+75) * time.Minute
				}
				addSymbolDirectionToBlacklist(grid.Symbol, grid.Direction, blockDuration)
				Discordf(display(globalStrategies[grid.SID], grid,
					fmt.Sprintf("**Direction shrink: %.2f**", ratio),
					0, 0))
			}
		}
	}
	if expiredCopiedIds.Cardinality() > 0 {
		Discordf("### Expired Strategies: %v", expiredCopiedIds)
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
		if grid.lastRoi >= 0 && time.Since(grid.tracking.timeLastChange) > time.Duration(TheConfig.CancelNoChangeMinutes)*time.Minute {
			err := closeGrid(grid.GID)
			if err != nil {
				return err
			}
			closedIds.Add(grid.GID)
			DiscordWebhookS(display(globalStrategies[grid.SID], grid,
				fmt.Sprintf("**Cancelled No Change - %s**", time.Since(grid.tracking.timeLastChange).Round(time.Second)),
				0, 0), ActionWebhook, DefaultWebhook)
			addSIDToBlacklist(grid.SID, 10*time.Minute)
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
					fmt.Sprintf("**Cancelled, max gain %.2f%%/%.2f%%, reached %s ago**",
						grid.lastRoi*100, grid.tracking.highestRoi*100,
						time.Since(grid.tracking.timeHighestRoi).Round(time.Second)),
					0, 0), ActionWebhook, DefaultWebhook)
				currentMinute := time.Now().Minute()
				if currentMinute < 15 || currentMinute > 40 { // new data is available

				}
			}
		}
	}

	if closedIds.Cardinality() > 0 && !TheConfig.Paper {
		Discordf("Cleared expired grids - Skip current run - Block trading for %d minutes", TheConfig.TradingBlockMinutesAfterCancel)
		tradingBlock = time.Now().Add(time.Duration(TheConfig.TradingBlockMinutesAfterCancel) * time.Minute)
		return nil
	}

	gridsOpen := len(gGrids.gridsByUid)
	if TheConfig.MaxChunks-gridsOpen <= 0 && !TheConfig.Paper {
		Discordf("Max Chunks reached, No cancel - Skip current run")
		return nil
	}
	if mapset.NewSetFromMapKeys(bundle.Filtered.symbolCount).Difference(gGrids.existingSymbols).Cardinality() == 0 && !TheConfig.Paper {
		Discordf("All symbols exists in open grids, Skip")
		return nil
	}
	if time.Now().Before(tradingBlock) && !TheConfig.Paper {
		Discordf("Trading Block, Skip")
		return nil
	}
	Discordf("### Opening new grids:")
	chunksInt := TheConfig.MaxChunks - gridsOpen
	chunks := float64(TheConfig.MaxChunks - gridsOpen)
	invChunk := (usdt - TheConfig.LeavingAsset) / chunks
	idealInvChunk := (usdt + gGrids.totalGridPnl + gGrids.totalGridInitial) / float64(TheConfig.MaxChunks)
	log.Infof("Ideal Investment: %f, allowed Investment: %f, missing %f chunks", idealInvChunk, invChunk, chunks)
	if invChunk > idealInvChunk {
		invChunk = idealInvChunk
	}
	sessionSymbols := gGrids.existingSymbols.Clone()
	for c, s := range bundle.Filtered.strategies {
		Discordf(display(s, nil, "New", c+1, len(bundle.Filtered.strategies)))
		if gGrids.existingIds.Contains(s.SID) {
			Discordf("Strategy exists in open grids, Skip")
			continue
		}
		if sessionSymbols.Contains(s.Symbol) {
			Discordf("Symbol exists in open grids, Skip")
			continue
		}
		if bl, till := SIDBlacklisted(s.SID); bl {
			Discordf("Strategy blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}
		if bl, till := SymbolDirectionBlacklisted(s.Symbol, DirectionMap[s.Direction]); bl {
			Discordf("Symbol Direction blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}
		if bl, till := SymbolBlacklisted(s.Symbol); bl {
			Discordf("Symbol blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}

		if s.StrategyParams.TriggerPrice != nil {
			triggerPrice, _ := strconv.ParseFloat(*s.StrategyParams.TriggerPrice, 64)
			marketPrice, _ := getSessionSymbolPrice(s.Symbol)
			diff := math.Abs((triggerPrice - marketPrice) / marketPrice)
			if diff > 0.08 {
				Discordf("Trigger Price difference too high, Skip, Trigger: %f, Market: %f, Diff: %f",
					triggerPrice, marketPrice, diff)
				continue
			}
		}

		switch s.Direction {
		case LONG:
			if TheConfig.MaxLongs >= 0 && gGrids.longs.Cardinality() >= TheConfig.MaxLongs {
				Discordf("Max Longs reached, Skip")
				continue
			}
		case NEUTRAL:
			if TheConfig.MaxNeutrals >= 0 && gGrids.shorts.Cardinality() >= TheConfig.MaxNeutrals {
				Discordf("Max Neutrals not reached, Skip")
				continue
			}
		}
		errr := placeGrid(*s, invChunk)
		if TheConfig.Paper {

		} else if errr != nil {
			Discordf("**Error placing grid: %v**", errr)
			if strings.Contains(errr.Error(), "Create grid too frequently") {
				Discordf("**Too Frequent Error, Skip Current Run**")
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
	Discordf("### New Grids:")
	err = updateOpenGrids(false)
	if err != nil {
		return err
	}
	return nil
}

// percentage of direction in same symbol group in filtered
// use it to cancel

// TODO: cancel when above n%, then cooldown?
// perform last 20 min roi (latest - last 20 OR if max roi was reached more than 20 min ago), if not positive and stop gain, cancel then block symbolpairdirection until next hr

func main() {
	configure()
	log.Infof("Public IP: %s", getPublicIP())
	DiscordService()
	switch TheConfig.Mode {
	case "trading":
		if TheConfig.Paper {
			Discordf("Paper Trading")
		} else {
			Discordf("Real Trading")
		}
		sdk()
		err := load(&statesOnGridOpen, GridStatesFileName)
		if err != nil {
			log.Fatalf("Error loading state on grid open: %v", err)
		}
		_, err = scheduler.SingletonMode().Every(TheConfig.TickEveryMinutes).Minutes().Do(
			func() {
				t := time.Now()
				err := tick()
				if err != nil {
					log.Errorf("Error: %v", err)
				}
				Discordf("*Run took: %v*", time.Since(t))
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
