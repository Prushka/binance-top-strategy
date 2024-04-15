package main

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/gsp"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/utils"
	"context"
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

func checkOppositeDirections(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	symbolDifferentDirectionsHigherRanking := 0
	possibleDirections := mapset.NewSet[string]()
	for _, s := range gsp.GetPool().Strategies {
		if s.Symbol == grid.Symbol {
			if gsp.DirectionMap[s.Direction] != grid.Direction {
				symbolDifferentDirectionsHigherRanking++
				possibleDirections.Add(gsp.DirectionMap[s.Direction])
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
	if symbolDifferentDirectionsHigherRanking >= config.TheConfig.MinOppositeDirectionHigherRanking &&
		existsNonBlacklistedOpposite {
		toCancel.AddGridToCancel(grid, 0,
			fmt.Sprintf("opposite directions at top: %d", symbolDifferentDirectionsHigherRanking))
	}
}

var directionShrinkPool = []string{
	gsp.SDRaw, gsp.SDPairSpecific,
}

func checkDirectionShrink(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	for _, sdPool := range directionShrinkPool {
		currentSDCount, sdCountWhenOpen, ratio := gsp.GridSDCount(grid.GID, grid.Symbol, grid.Direction, sdPool)
		diff := sdCountWhenOpen - currentSDCount
		for c, ratioCutoff := range config.TheConfig.SymbolDirectionShrink {
			if ratio < ratioCutoff && diff >= config.TheConfig.SymbolDirectionShrinkMinConstant {
				maxLoss := config.TheConfig.SymbolDirectionShrinkLoss[c]
				reason := fmt.Sprintf("direction shrink: %.2f, accept loss: %f", ratio, maxLoss)
				blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), reason)
				toCancel.AddGridToCancel(grid, maxLoss, reason)
			}
		}
	}
}

func checkTakeProfits(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	for c, gpMax := range config.TheConfig.TakeProfits {
		gpMax = config.GetScaledProfits(gpMax, grid.InitialLeverage)
		if grid.LastRoi >= gpMax {
			gpLookBack := time.Duration(config.TheConfig.TakeProfitsMaxLookbackMinutes[c]) * time.Minute
			gpBlock := time.Duration(config.TheConfig.TakeProfitsBlockMinutes[c]) * time.Minute
			gridTracking := grid.GetTracking()
			lowerBound := gridTracking.GetLowestWithin(gpLookBack)
			if time.Since(gridTracking.TimeHighestRoi) > gpLookBack && lowerBound >= gpMax {
				reason := fmt.Sprintf("max gain %.2f%%/%.2f%%, reached %s ago",
					grid.LastRoi*100, gridTracking.HighestRoi*100,
					time.Since(gridTracking.TimeHighestRoi).Round(time.Second))
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

func checkStopLossNotPicked(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	for c, slHr := range config.TheConfig.StopLossNotPickedHrs {
		slDuration := time.Duration(slHr) * time.Hour
		maxLoss := config.TheConfig.StopLossNotPicked[c]
		notPickedDuration := gsp.GridNotPickedDuration(grid.GID)
		if *notPickedDuration > slDuration {
			toCancel.AddGridToCancel(grid, maxLoss, fmt.Sprintf("not picked for %s, accept loss: %f",
				notPickedDuration.Round(time.Second), maxLoss))
		}
	}
}

func checkStopLoss(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	for c, sl := range config.TheConfig.StopLossMarkForRemoval {
		slack := config.TheConfig.StopLossMarkForRemovalSlack[c]
		if grid.LastRoi < sl {
			gsp.GridMarkForRemoval(grid.GID, sl+slack)
			discord.Infof(fmt.Sprintf("**stop loss marked for removal**: %.2f%%", (sl+slack)*100))
		}
	}
	maxLoss := gsp.GetMaxLoss(grid.GID)
	if maxLoss != nil && grid.LastRoi > *maxLoss {
		reason := fmt.Sprintf("**stop loss reached**: %.2f%%", *maxLoss*100)
		toCancel.AddGridToCancel(grid, *maxLoss, reason)
		blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), reason)
	}
}

func tick() error {
	utils.ResetTime()
	sdk.ClearSessionSymbolPrice()
	discord.Infof("## Run: %v", time.Now().Format("2006-01-02 15:04:05"))
	usdt, err := sdk.GetFutureUSDT()
	if err != nil {
		return err
	}
	usdt -= config.TheConfig.LeavingAsset
	err = gsp.UpdateTopStrategiesWithRoi()
	if err != nil {
		return err
	}
	utils.Time("Fetch strategies")
	discord.Infof("### Current Grids:")
	sdk.ClearSessionSymbolPrice()
	err = gsp.UpdateOpenGrids(true)
	if err != nil {
		return err
	}
	gsp.SessionCancelledGIDs.Clear()

	utils.Time("Fetch grids")
	toCancel := make(gsp.GridsToCancel)
	count := 0
	grids := utils.MapValues(gsp.GGrids.GridsByGid)
	sort.Slice(grids, func(i, j int) bool {
		return grids[i].GID < grids[j].GID
	})
	for _, grid := range grids {
		discord.Infof(gsp.Display(nil, grid, "", count+1, len(gsp.GGrids.GridsByGid)))
		count++
		if !gsp.Bundle.Raw.Exists(grid.SID) {
			toCancel.AddGridToCancel(grid, config.TheConfig.MaxCancelLossStrategyDeleted, "strategy not found")
			if grid.LastRoi < 0 {
				blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), "strategy not found, lastRoi loss")
			}
			checkStopLossNotPicked(grid, toCancel)
		} else if !gsp.GetPool().Exists(grid.SID) {
			toCancel.AddGridToCancel(grid, 0, "strategy not picked")
			checkStopLossNotPicked(grid, toCancel)
		}
		gridTracking := grid.GetTracking()
		if time.Since(gridTracking.TimeLastChange) > time.Duration(config.TheConfig.CancelNoChangeMinutes)*time.Minute {
			reason := fmt.Sprintf("no change, %s", utils.ShortDur(time.Since(gridTracking.TimeLastChange).Round(time.Second)))
			blacklist.AddSID(grid.SID, 10*time.Minute, reason)
			toCancel.AddGridToCancel(grid, 0, reason)
		}

		checkTakeProfits(grid, toCancel)
		checkDirectionShrink(grid, toCancel)
		checkOppositeDirections(grid, toCancel)
		checkStopLoss(grid, toCancel)
	}
	if !toCancel.IsEmpty() {
		discord.Infof("### Expired Strategies: %s", toCancel)
		toCancel.CancelAll()
	}

	if toCancel.HasCancelled() && !config.TheConfig.Paper {
		discord.Infof("Cancelled expired grids - Skip current run")
		gsp.SessionCancelledGIDs = toCancel.CancelledGIDs()
		return nil
	}

	gridsOpen := len(gsp.GGrids.GridsByGid)
	if config.TheConfig.MaxChunks-gridsOpen <= 0 && !config.TheConfig.Paper {
		discord.Infof("Max Chunks reached, No cancel - Skip current run")
		return nil
	}
	if mapset.NewSetFromMapKeys(gsp.GetPool().SymbolCount).Difference(gsp.GGrids.ExistingSymbols).Cardinality() == 0 && !config.TheConfig.Paper {
		discord.Infof("All symbols exists in open grids, Skip")
		return nil
	}
	if bl, _ := blacklist.IsTradingBlocked(); bl && !config.TheConfig.Paper {
		discord.Infof("Trading Block, Skip")
		return nil
	}
	chunksInt := config.TheConfig.MaxChunks - gridsOpen
	chunks := float64(config.TheConfig.MaxChunks - gridsOpen)
	invChunk := usdt / chunks
	idealInvChunk := (usdt + gsp.GGrids.TotalGridPnl + gsp.GGrids.TotalGridInitial) / float64(config.TheConfig.MaxChunks)
	log.Infof("Ideal Investment: %f, allowed Investment: %f, missing %f chunks", idealInvChunk, invChunk, chunks)
	if invChunk > idealInvChunk {
		invChunk = idealInvChunk
	}
	if invChunk < config.TheConfig.MinInvestmentPerChunk && !config.TheConfig.Paper {
		discord.Infof("Investment too low (%f), Skip", invChunk)
		return nil
	}
	invChunk = float64(int(invChunk))
	discord.Infof("### Opening new grids:")
	sessionSymbols := gsp.GGrids.ExistingSymbols.Clone()
	blacklistedInPool := mapset.NewSet[string]()
	for c, s := range gsp.GetPool().Strategies {
		if gsp.GGrids.ExistingSIDs.Contains(s.SID) {
			discord.Infof("* Strategy %d - %s exists in open grids, Skip", s.SID, s.SD())
			continue
		}
		if sessionSymbols.Contains(s.Symbol) {
			log.Infof("Symbol exists in open grids, Skip")
			continue
		}

		if bl, till := blacklist.SIDBlacklisted(s.SID); bl {
			blacklistedInPool.Add(fmt.Sprintf("%d", s.SID))
			log.Infof("Strategy blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}
		if bl, till := blacklist.SymbolDirectionBlacklisted(s.Symbol, gsp.DirectionMap[s.Direction]); bl {
			blacklistedInPool.Add(s.SD())
			log.Infof("Symbol Direction blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}
		if bl, till := blacklist.SymbolBlacklisted(s.Symbol); bl {
			blacklistedInPool.Add(s.Symbol)
			log.Infof("Symbol blacklisted till %s, Skip", till.Format("2006-01-02 15:04:05"))
			continue
		}

		discord.Infof(gsp.Display(s, nil, "New", c+1, len(gsp.GetPool().Strategies)))

		minInvestment, _ := strconv.ParseFloat(s.MinInvestment, 64)
		leverage := s.MaxLeverage(invChunk)
		switch s.Direction {
		case gsp.LONG:

		case gsp.NEUTRAL:
			minInvestPerLeverage := minInvestment * float64(s.StrategyParams.Leverage)
			realMinInvestment := minInvestPerLeverage / float64(leverage)
			if invChunk < realMinInvestment {
				discord.Infof("Investment too low (%f/%f), Skip", invChunk, realMinInvestment)
				continue
			}
		case gsp.SHORT:

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

		if !s.MarketPriceWithinRange() {
			discord.Infof("Market Price not within range, Skip")
			continue
		}

		errr := gsp.PlaceGrid(*s, invChunk, leverage)
		if config.TheConfig.Paper {

		} else if errr != nil {
			discord.Infof("**Error placing grid: %v**", errr)
			if strings.Contains(errr.Error(), "Create grid too frequently") {
				discord.Infof("**Too Frequent Error, Skip Current Run**")
				break
			}
		} else {
			discord.Actionf(gsp.Display(s, nil, "**Opened Grid**", c+1, len(gsp.GetPool().Strategies)))
			chunksInt -= 1
			sessionSymbols.Add(s.Symbol)
			if chunksInt <= 0 {
				break
			}
		}
	}
	if blacklistedInPool.Cardinality() > 0 {
		discord.Infof("Blacklisted in pool: %s", blacklistedInPool)
	}

	utils.Time("Place/Cancel done")
	discord.Infof("### New Grids:")
	err = gsp.UpdateOpenGrids(false)
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

// Use filtered SD ratio to cancel
// Use total SD ratio of the pair to cancel

// neutral: either trail down or check if range is in the middle before placing
// TODO: cancel when above n%, then cooldown?
// perform last 20 min roi (latest - last 20 OR if max roi was reached more than 20 min ago), if not positive and stop gain, cancel then block symbolpairdirection until next hr

func main() {
	config.Init()
	discord.Init()
	switch config.TheConfig.Mode {
	case "trading":
		if config.TheConfig.Paper {
			discord.Errorf("Paper Trading")
		} else {
			discord.Errorf("Real Trading")
		}
		sdk.Init()
		gsp.Init()
		blacklist.Init()
		_, err := scheduler.SingletonMode().Every(config.TheConfig.TickEverySeconds).Seconds().Do(
			func() {
				t := time.Now()
				err := tick()
				if err != nil {
					discord.Errorf("Error: %v", err)
				}
				discord.Infof("*Run took: %v*", time.Since(t))
			},
		)
		if err != nil {
			discord.Errorf("Error: %v", err)
			return
		}
		scheduler.StartBlocking()
	case "playground":
		sdk.Init()
		timeAgo := 47 * time.Hour
		res, err := sdk.FuturesClient.NewMarkPriceKlinesService().
			Symbol("BCHUSDT").Interval("1m").StartTime(time.Now().Add(-timeAgo).Unix() * 1000).
			EndTime(time.Now().Add(-timeAgo+time.Minute).Unix() * 1000).Do(context.Background())
		if err != nil {
			discord.Errorf("Error: %v", err)
			return
		}
		log.Info(utils.AsJson(res))
	}
}
