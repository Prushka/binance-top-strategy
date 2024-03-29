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
	Raw                    *TrackedStrategies
	FilteredSortedBySD     *TrackedStrategies
	FilteredSortedByMetric *TrackedStrategies
	SDCountPairSpecific    map[string]int
}

type StateOnGridOpen struct {
	SDCountRaw          map[string]int
	SDCountFiltered     map[string]int
	SDCountPairSpecific map[string]int
}

const (
	SDRaw          = "SDRaw"
	SDFiltered     = "SDFiltered"
	SDPairSpecific = "SDPairSpecific"
)

var statesOnGridOpen = make(map[int]*StateOnGridOpen)

func gridSDCount(gid int, symbol, direction string, setType string) (int, int, float64) {
	sd := symbol + direction
	var currentSDCount int
	var sdCountWhenOpen int
	switch setType {
	case SDRaw:
		currentSDCount = bundle.Raw.symbolDirectionCount[sd]
		sdCountWhenOpen = statesOnGridOpen[gid].SDCountRaw[sd]
	case SDFiltered:
		currentSDCount = bundle.FilteredSortedBySD.symbolDirectionCount[sd]
		sdCountWhenOpen = statesOnGridOpen[gid].SDCountFiltered[sd]
	case SDPairSpecific:
		currentSDCount = bundle.SDCountPairSpecific[sd]
		sdCountWhenOpen = statesOnGridOpen[gid].SDCountPairSpecific[sd]
	}
	ratio := float64(currentSDCount) / float64(sdCountWhenOpen)
	return currentSDCount, sdCountWhenOpen, ratio
}

func persistStateOnGridOpen(gid int) {
	if _, ok := statesOnGridOpen[gid]; !ok {
		statesOnGridOpen[gid] = &StateOnGridOpen{SDCountRaw: bundle.Raw.symbolDirectionCount,
			SDCountFiltered:     bundle.FilteredSortedBySD.symbolCount,
			SDCountPairSpecific: bundle.SDCountPairSpecific}
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

func updateTopStrategiesWithRoi() error {
	strategies, err := getTopStrategies(FUTURE, "")
	if err != nil {
		return err
	}
	filtered := make(Strategies, 0)
	for c, s := range strategies.strategies {
		id := s.SID
		roi, err := RoisCache.Get(fmt.Sprintf("%d-%d", id, s.UserID))
		if err != nil {
			return err
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
			s.last6HrRoiPerHr = GetRoiPerHr(s.Rois, 6*time.Hour)
			s.lastNHrNoDip = NoDip(s.Rois, time.Duration(TheConfig.LastNHoursNoDips)*time.Hour)
			s.roiPerHour = (s.roi - s.Rois[len(s.Rois)-1].Roi) / float64(s.RunningTime/3600)
			prefix := ""
			if s.lastDayRoiChange > 0.1 &&
				s.last3HrRoiChange > 0.03 &&
				s.lastHrRoiChange > 0.01 &&
				s.last2HrRoiChange > s.lastHrRoiChange &&
				s.lastDayRoiPerHr > 0.01 &&
				s.last12HrRoiPerHr > 0.014 &&
				s.last6HrRoiPerHr > 0.014 &&
				s.priceDifference > 0.05 &&
				// TODO: price difference can shrink with trailing, e.g., 5.xx% -> 4.xx%
				s.lastNHrNoDip {
				filtered = append(filtered, s)
				prefix += "Open"
			}
			log.Info(prefix + display(s, nil, "Found", c+1, len(strategies.strategiesById)))
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		I := filtered[i]
		J := filtered[j]
		return I.last6HrRoiPerHr > J.last6HrRoiPerHr
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
		sdLengths = append(sdLengths, SDCountPair{SymbolDirection: sd, Count: len(s), MaxMetric: s[0].last6HrRoiPerHr})
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
	bundle = &StrategiesBundle{Raw: strategies,
		FilteredSortedBySD:     sortedBySDCount.toTrackedStrategies(),
		FilteredSortedByMetric: filtered.toTrackedStrategies(),
		SDCountPairSpecific:    make(map[string]int)}
	Discordf("### Strategies")
	Discordf("* Raw: " + bundle.Raw.String())
	Discordf("* Open: " + bundle.FilteredSortedBySD.String())
	filteredSymbols := mapset.NewSetFromMapKeys(bundle.FilteredSortedBySD.symbolCount)
	var gridSymbols mapset.Set[string]
	if gGrids.existingSymbols.Cardinality() > 0 {
		gridSymbols = gGrids.existingSymbols
	} else {
		gridSymbols, err = getGridSymbols()
		if err != nil {
			return err
		}
	}
	err = updateSDCountPairSpecific(filteredSymbols.Union(gridSymbols))
	if err != nil {
		return err
	}
	return nil
}

func updateSDCountPairSpecific(symbols mapset.Set[string]) error {
	for _, symbol := range symbols.ToSlice() {
		strategies, err := getTopStrategies(FUTURE, symbol)
		if err != nil {
			return err
		}
		for sd, count := range strategies.symbolDirectionCount {
			if _, ok := bundle.SDCountPairSpecific[sd]; !ok {
				bundle.SDCountPairSpecific[sd] = count
			}
		}
	}
	Discordf("* SDSpecific: %v", bundle.SDCountPairSpecific)
	return nil
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
	err = updateTopStrategiesWithRoi()
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
	for _, grid := range gGrids.gridsByGid {
		Discordf(display(nil, grid, "", count+1, len(gGrids.gridsByGid)))
		count++
	}
	toCancel := make(GridsToCancel)
	for _, grid := range gGrids.gridsByGid {
		symbolDifferentDirectionsHigherRanking := 0
		possibleDirections := mapset.NewSet[string]()
		for _, s := range bundle.FilteredSortedByMetric.strategies {
			if s.Symbol == grid.Symbol {
				if DirectionMap[s.Direction] != grid.Direction {
					symbolDifferentDirectionsHigherRanking++
					possibleDirections.Add(DirectionMap[s.Direction])
				} else {
					break
				}
			}
		}
		existsNonBlacklistedOpposite := false
		for _, d := range possibleDirections.ToSlice() {
			if bl, _ := SymbolDirectionBlacklisted(grid.Symbol, d); !bl {
				existsNonBlacklistedOpposite = true
				break
			}
		}
		if symbolDifferentDirectionsHigherRanking >= 2 && existsNonBlacklistedOpposite {
			toCancel.AddGridToCancel(grid, 0,
				fmt.Sprintf("opposite directions at top: %d", symbolDifferentDirectionsHigherRanking))
			addSymbolDirectionToBlacklist(grid.Symbol, grid.Direction, 10*time.Minute, "opposite directions at top")
		}

		currentSDCount, sdCountWhenOpen, ratio := gridSDCount(grid.GID, grid.Symbol, grid.Direction, SDRaw)
		if ratio < TheConfig.CancelSymbolDirectionShrink && sdCountWhenOpen-currentSDCount >= TheConfig.CancelSymbolDirectionShrinkMinConstant {
			reason := fmt.Sprintf("direction shrink: %.2f", ratio)
			addSymbolDirectionToBlacklist(grid.Symbol, grid.Direction, TillNextRefresh(), reason)
			toCancel.AddGridToCancel(grid, 0, reason)
			if ratio < TheConfig.CancelWithLossSymbolDirectionShrink {
				toCancel.AddGridToCancel(grid, TheConfig.MaxLossWithSymbolDirectionShrink,
					fmt.Sprintf("shrink below %f, Accept Loss: %f",
						TheConfig.CancelWithLossSymbolDirectionShrink, TheConfig.MaxLossWithSymbolDirectionShrink))
			}
		}

		if !bundle.Raw.exists(grid.SID) {
			toCancel.AddGridToCancel(grid, TheConfig.MaxCancelLossStrategyDeleted, "strategy not found")
			if grid.lastRoi < 0 {
				addSymbolDirectionToBlacklist(grid.Symbol, grid.Direction, TillNextRefresh(), "strategy not found, lastRoi loss")
			}
		} else if !bundle.FilteredSortedBySD.exists(grid.SID) {
			toCancel.AddGridToCancel(grid, 0, "strategy not picked")
		}

		if time.Since(grid.tracking.timeLastChange) > time.Duration(TheConfig.CancelNoChangeMinutes)*time.Minute {
			reason := fmt.Sprintf("no change, %s", shortDur(time.Since(grid.tracking.timeLastChange).Round(time.Second)))
			addSIDToBlacklist(grid.SID, 10*time.Minute, reason)
			toCancel.AddGridToCancel(grid, 0, reason)
		}

		for c, gpMax := range TheConfig.TakeProfits {
			if grid.lastRoi >= gpMax {
				gpLookBack := time.Duration(TheConfig.TakeProfitsMaxLookbackMinutes[c]) * time.Minute
				gpBlock := time.Duration(TheConfig.TakeProfitsBlockMinutes[c]) * time.Minute
				if time.Since(grid.tracking.timeHighestRoi) > gpLookBack {
					reason := fmt.Sprintf("max gain %.2f%%/%.2f%%, reached %s ago",
						grid.lastRoi*100, grid.tracking.highestRoi*100,
						time.Since(grid.tracking.timeHighestRoi).Round(time.Second))
					toCancel.AddGridToCancel(grid, gpMax, reason)
					if gpBlock < 0 {
						addSymbolToBlacklist(grid.Symbol, TillNextRefresh(), reason)
					} else {
						addSymbolToBlacklist(grid.Symbol, gpBlock, reason)
					}
				}
			}
		}
	}
	if !toCancel.Empty() {
		Discordf("### Expired Strategies: %s", toCancel)
		toCancel.CancelAll()
	}

	if toCancel.hasCancelled() && !TheConfig.Paper {
		Discordf("Cleared expired grids - Skip current run")
		return nil
	}

	gridsOpen := len(gGrids.gridsByGid)
	if TheConfig.MaxChunks-gridsOpen <= 0 && !TheConfig.Paper {
		Discordf("Max Chunks reached, No cancel - Skip current run")
		return nil
	}
	if mapset.NewSetFromMapKeys(bundle.FilteredSortedBySD.symbolCount).Difference(gGrids.existingSymbols).Cardinality() == 0 && !TheConfig.Paper {
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
	for c, s := range bundle.FilteredSortedBySD.strategies {
		Discordf(display(s, nil, "New", c+1, len(bundle.FilteredSortedBySD.strategies)))
		if gGrids.existingSIDs.Contains(s.SID) {
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
			DiscordWebhookS(display(s, nil, "**Opened Grid**", c+1, len(bundle.FilteredSortedBySD.strategies)), ActionWebhook, DefaultWebhook)
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

// strict stop gain then block

// Detect move out of range

// strict stop loss or other conditions then block

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
		err = load(blacklist, BlacklistFileName)
		if err != nil {
			log.Fatalf("Error loading blacklist: %v", err)
		}
		_, err = scheduler.SingletonMode().Every(TheConfig.TickEverySeconds).Seconds().Do(
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

	case "extract-cookies":

	}

	scheduler.StartBlocking()
}
