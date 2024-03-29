package main

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/persistence"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/utils"
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

var globalStrategies = make(map[int]*Strategy)
var gGrids = newTrackedGrids()
var tradingBlock = time.Now()
var bundle *StrategiesBundle
var statesOnGridOpen = make(map[int]*StateOnGridOpen)

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

const (
	SDRaw          = "SDRaw"
	SDFiltered     = "SDFiltered"
	SDPairSpecific = "SDPairSpecific"
)

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
			s.lastNHrNoDip = NoDip(s.Rois, time.Duration(config.TheConfig.LastNHoursNoDips)*time.Hour)
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
	discord.Infof("### Strategies")
	discord.Infof("* Raw: " + bundle.Raw.String())
	discord.Infof("* Open: " + bundle.FilteredSortedBySD.String())
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
	log.Infof("Strategy with Symbol Specifics: %v", symbols)
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
	discord.Infof("* SDSpecific: %v", bundle.SDCountPairSpecific)
	return nil
}

func tick() error {
	utils.ResetTime()
	sdk.ClearSessionSymbolPrice()
	discord.Infof("## Run: %v", time.Now().Format("2006-01-02 15:04:05"))
	usdt, err := sdk.GetFutureUSDT()
	if err != nil {
		return err
	}
	err = updateTopStrategiesWithRoi()
	if err != nil {
		return err
	}
	utils.Time("Fetch strategies")
	sdk.ClearSessionSymbolPrice()
	discord.Infof("### Current Grids:")
	err = updateOpenGrids(true)
	if err != nil {
		return err
	}

	utils.Time("Fetch grids")
	count := 0
	for _, grid := range gGrids.gridsByGid {
		discord.Infof(display(nil, grid, "", count+1, len(gGrids.gridsByGid)))
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
			if bl, _ := blacklist.SymbolDirectionBlacklisted(grid.Symbol, d); !bl {
				existsNonBlacklistedOpposite = true
				break
			}
		}
		if symbolDifferentDirectionsHigherRanking >= 2 && existsNonBlacklistedOpposite {
			toCancel.AddGridToCancel(grid, 0,
				fmt.Sprintf("opposite directions at top: %d", symbolDifferentDirectionsHigherRanking))
			blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, 10*time.Minute, "opposite directions at top")
		}

		currentSDCount, sdCountWhenOpen, ratio := gridSDCount(grid.GID, grid.Symbol, grid.Direction, SDRaw)
		if ratio < config.TheConfig.CancelSymbolDirectionShrink && sdCountWhenOpen-currentSDCount >= config.TheConfig.CancelSymbolDirectionShrinkMinConstant {
			reason := fmt.Sprintf("direction shrink: %.2f", ratio)
			blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), reason)
			toCancel.AddGridToCancel(grid, 0, reason)
			if ratio < config.TheConfig.CancelWithLossSymbolDirectionShrink {
				toCancel.AddGridToCancel(grid, config.TheConfig.MaxLossWithSymbolDirectionShrink,
					fmt.Sprintf("shrink below %f, Accept Loss: %f",
						config.TheConfig.CancelWithLossSymbolDirectionShrink, config.TheConfig.MaxLossWithSymbolDirectionShrink))
			}
		}

		if !bundle.Raw.exists(grid.SID) {
			toCancel.AddGridToCancel(grid, config.TheConfig.MaxCancelLossStrategyDeleted, "strategy not found")
			if grid.lastRoi < 0 {
				blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), "strategy not found, lastRoi loss")
			}
		} else if !bundle.FilteredSortedBySD.exists(grid.SID) {
			toCancel.AddGridToCancel(grid, 0, "strategy not picked")
		}

		if time.Since(grid.tracking.timeLastChange) > time.Duration(config.TheConfig.CancelNoChangeMinutes)*time.Minute {
			reason := fmt.Sprintf("no change, %s", utils.ShortDur(time.Since(grid.tracking.timeLastChange).Round(time.Second)))
			blacklist.AddSID(grid.SID, 10*time.Minute, reason)
			toCancel.AddGridToCancel(grid, 0, reason)
		}

		for c, gpMax := range config.TheConfig.TakeProfits {
			if grid.lastRoi >= gpMax {
				gpLookBack := time.Duration(config.TheConfig.TakeProfitsMaxLookbackMinutes[c]) * time.Minute
				gpBlock := time.Duration(config.TheConfig.TakeProfitsBlockMinutes[c]) * time.Minute
				if time.Since(grid.tracking.timeHighestRoi) > gpLookBack {
					reason := fmt.Sprintf("max gain %.2f%%/%.2f%%, reached %s ago",
						grid.lastRoi*100, grid.tracking.highestRoi*100,
						time.Since(grid.tracking.timeHighestRoi).Round(time.Second))
					toCancel.AddGridToCancel(grid, gpMax, reason)
					if gpBlock < 0 {
						blacklist.AddSymbol(grid.Symbol, utils.TillNextRefresh(), reason)
					} else {
						blacklist.AddSymbol(grid.Symbol, gpBlock, reason)
					}
				}
			}
		}
	}
	if !toCancel.Empty() {
		discord.Infof("### Expired Strategies: %s", toCancel)
		toCancel.CancelAll()
	}

	if toCancel.hasCancelled() && !config.TheConfig.Paper {
		discord.Infof("Cleared expired grids - Skip current run")
		return nil
	}

	gridsOpen := len(gGrids.gridsByGid)
	if config.TheConfig.MaxChunks-gridsOpen <= 0 && !config.TheConfig.Paper {
		discord.Infof("Max Chunks reached, No cancel - Skip current run")
		return nil
	}
	if mapset.NewSetFromMapKeys(bundle.FilteredSortedBySD.symbolCount).Difference(gGrids.existingSymbols).Cardinality() == 0 && !config.TheConfig.Paper {
		discord.Infof("All symbols exists in open grids, Skip")
		return nil
	}
	if time.Now().Before(tradingBlock) && !config.TheConfig.Paper {
		discord.Infof("Trading Block, Skip")
		return nil
	}
	discord.Infof("### Opening new grids:")
	chunksInt := config.TheConfig.MaxChunks - gridsOpen
	chunks := float64(config.TheConfig.MaxChunks - gridsOpen)
	invChunk := (usdt - config.TheConfig.LeavingAsset) / chunks
	idealInvChunk := (usdt + gGrids.totalGridPnl + gGrids.totalGridInitial) / float64(config.TheConfig.MaxChunks)
	log.Infof("Ideal Investment: %f, allowed Investment: %f, missing %f chunks", idealInvChunk, invChunk, chunks)
	if invChunk > idealInvChunk {
		invChunk = idealInvChunk
	}
	sessionSymbols := gGrids.existingSymbols.Clone()
	for c, s := range bundle.FilteredSortedBySD.strategies {
		discord.Infof(display(s, nil, "New", c+1, len(bundle.FilteredSortedBySD.strategies)))
		if gGrids.existingSIDs.Contains(s.SID) {
			discord.Infof("Strategy exists in open grids, Skip")
			continue
		}
		if sessionSymbols.Contains(s.Symbol) {
			discord.Infof("Symbol exists in open grids, Skip")
			continue
		}
		if bl, till := blacklist.SIDBlacklisted(s.SID); bl {
			discord.Infof("Strategy blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}
		if bl, till := blacklist.SymbolDirectionBlacklisted(s.Symbol, DirectionMap[s.Direction]); bl {
			discord.Infof("Symbol Direction blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}
		if bl, till := blacklist.SymbolBlacklisted(s.Symbol); bl {
			discord.Infof("Symbol blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}

		if s.StrategyParams.TriggerPrice != nil {
			triggerPrice, _ := strconv.ParseFloat(*s.StrategyParams.TriggerPrice, 64)
			marketPrice, _ := sdk.GetSessionSymbolPrice(s.Symbol)
			diff := math.Abs((triggerPrice - marketPrice) / marketPrice)
			if diff > 0.08 {
				discord.Infof("Trigger Price difference too high, Skip, Trigger: %f, Market: %f, Diff: %f",
					triggerPrice, marketPrice, diff)
				continue
			}
		}

		switch s.Direction {
		case LONG:
			if config.TheConfig.MaxLongs >= 0 && gGrids.longs.Cardinality() >= config.TheConfig.MaxLongs {
				discord.Infof("Max Longs reached, Skip")
				continue
			}
		case NEUTRAL:
			if config.TheConfig.MaxNeutrals >= 0 && gGrids.shorts.Cardinality() >= config.TheConfig.MaxNeutrals {
				discord.Infof("Max Neutrals not reached, Skip")
				continue
			}
		}
		errr := placeGrid(*s, invChunk)
		if config.TheConfig.Paper {

		} else if errr != nil {
			discord.Infof("**Error placing grid: %v**", errr)
			if strings.Contains(errr.Error(), "Create grid too frequently") {
				discord.Infof("**Too Frequent Error, Skip Current Run**")
				break
			}
		} else {
			discord.Info(display(s, nil, "**Opened Grid**", c+1, len(bundle.FilteredSortedBySD.strategies)), discord.ActionWebhook, discord.DefaultWebhook)
			chunksInt -= 1
			sessionSymbols.Add(s.Symbol)
			if chunksInt <= 0 {
				break
			}
		}
	}

	utils.Time("Place/Cancel done")
	discord.Infof("### New Grids:")
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
	config.Init()
	log.Infof("Public IP: %s", utils.GetPublicIP())
	discord.Init()
	switch config.TheConfig.Mode {
	case "trading":
		if config.TheConfig.Paper {
			discord.Infof("Paper Trading")
		} else {
			discord.Infof("Real Trading")
		}
		sdk.Init()
		err := persistence.Load(&statesOnGridOpen, persistence.GridStatesFileName)
		if err != nil {
			log.Fatalf("Error loading state on grid open: %v", err)
		}
		blacklist.Init()
		_, err = scheduler.SingletonMode().Every(config.TheConfig.TickEverySeconds).Seconds().Do(
			func() {
				t := time.Now()
				err := tick()
				if err != nil {
					log.Errorf("Error: %v", err)
				}
				discord.Infof("*Run took: %v*", time.Since(t))
			},
		)
		if err != nil {
			log.Errorf("Error: %v", err)
			return
		}
	}
	scheduler.StartBlocking()
}
