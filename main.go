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
	"strconv"
	"strings"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())

func tick() error {
	utils.ResetTime()
	sdk.ClearSessionSymbolPrice()
	discord.Infof("## Run: %v", time.Now().Format("2006-01-02 15:04:05"))
	usdt, err := sdk.GetFutureUSDT()
	if err != nil {
		return err
	}
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

	utils.Time("Fetch grids")
	count := 0
	for _, grid := range gsp.GGrids.GridsByGid {
		discord.Infof(gsp.Display(nil, grid, "", count+1, len(gsp.GGrids.GridsByGid)))
		count++
	}
	toCancel := make(gsp.GridsToCancel)
	for _, grid := range gsp.GGrids.GridsByGid {
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
		if symbolDifferentDirectionsHigherRanking >= 2 && existsNonBlacklistedOpposite {
			toCancel.AddGridToCancel(grid, 0,
				fmt.Sprintf("opposite directions at top: %d", symbolDifferentDirectionsHigherRanking))
			blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, 10*time.Minute, "opposite directions at top")
		}

		currentSDCount, sdCountWhenOpen, ratio := gsp.GridSDCount(grid.GID, grid.Symbol, grid.Direction, gsp.SDRaw)
		if ratio < config.TheConfig.CancelSymbolDirectionShrink && sdCountWhenOpen-currentSDCount >= config.TheConfig.CancelSymbolDirectionShrinkMinConstant {
			reason := fmt.Sprintf("direction shrink: %.2f", ratio)
			blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), reason)
			toCancel.AddGridToCancel(grid, 0, reason)
			if ratio < config.TheConfig.CancelWithLossSymbolDirectionShrink {
				toCancel.AddGridToCancel(grid, config.TheConfig.MaxLossWithSymbolDirectionShrink,
					fmt.Sprintf("shrink below %f, accept loss: %f",
						config.TheConfig.CancelWithLossSymbolDirectionShrink, config.TheConfig.MaxLossWithSymbolDirectionShrink))
			}
		}

		if !gsp.Bundle.Raw.Exists(grid.SID) {
			toCancel.AddGridToCancel(grid, config.TheConfig.MaxCancelLossStrategyDeleted, "strategy not found")
			if grid.LastRoi < 0 {
				blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), "strategy not found, lastRoi loss")
			}
		} else if !gsp.GetPool().Exists(grid.SID) {
			toCancel.AddGridToCancel(grid, 0, "strategy not picked")
		}

		if time.Since(grid.Tracking.TimeLastChange) > time.Duration(config.TheConfig.CancelNoChangeMinutes)*time.Minute {
			reason := fmt.Sprintf("no change, %s", utils.ShortDur(time.Since(grid.Tracking.TimeLastChange).Round(time.Second)))
			blacklist.AddSID(grid.SID, 10*time.Minute, reason)
			toCancel.AddGridToCancel(grid, 0, reason)
		}

		for c, gpMax := range config.TheConfig.TakeProfits {
			if grid.LastRoi >= gpMax {
				gpLookBack := time.Duration(config.TheConfig.TakeProfitsMaxLookbackMinutes[c]) * time.Minute
				gpBlock := time.Duration(config.TheConfig.TakeProfitsBlockMinutes[c]) * time.Minute
				if time.Since(grid.Tracking.TimeHighestRoi) > gpLookBack {
					reason := fmt.Sprintf("max gain %.2f%%/%.2f%%, reached %s ago",
						grid.LastRoi*100, grid.Tracking.HighestRoi*100,
						time.Since(grid.Tracking.TimeHighestRoi).Round(time.Second))
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
	if !toCancel.IsEmpty() {
		discord.Infof("### Expired Strategies: %s", toCancel)
		toCancel.CancelAll()
	}

	if toCancel.HasCancelled() && !config.TheConfig.Paper {
		discord.Infof("Cleared expired grids - Skip current run")
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
	discord.Infof("### Opening new grids:")
	chunksInt := config.TheConfig.MaxChunks - gridsOpen
	chunks := float64(config.TheConfig.MaxChunks - gridsOpen)
	invChunk := (usdt - config.TheConfig.LeavingAsset) / chunks
	idealInvChunk := (usdt + gsp.GGrids.TotalGridPnl + gsp.GGrids.TotalGridInitial) / float64(config.TheConfig.MaxChunks)
	log.Infof("Ideal Investment: %f, allowed Investment: %f, missing %f chunks", idealInvChunk, invChunk, chunks)
	if invChunk > idealInvChunk {
		invChunk = idealInvChunk
	}
	sessionSymbols := gsp.GGrids.ExistingSymbols.Clone()
	for c, s := range gsp.GetPool().Strategies {
		discord.Infof(gsp.Display(s, nil, "New", c+1, len(gsp.GetPool().Strategies)))
		if gsp.GGrids.ExistingSIDs.Contains(s.SID) {
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
		if bl, till := blacklist.SymbolDirectionBlacklisted(s.Symbol, gsp.DirectionMap[s.Direction]); bl {
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

		errr := gsp.PlaceGrid(*s, invChunk)
		if config.TheConfig.Paper {

		} else if errr != nil {
			discord.Infof("**Error placing grid: %v**", errr)
			if strings.Contains(errr.Error(), "Create grid too frequently") {
				discord.Infof("**Too Frequent Error, Skip Current Run**")
				break
			}
		} else {
			discord.Info(gsp.Display(s, nil, "**Opened Grid**", c+1, len(gsp.GetPool().Strategies)), discord.ActionWebhook, discord.DefaultWebhook)
			chunksInt -= 1
			sessionSymbols.Add(s.Symbol)
			if chunksInt <= 0 {
				break
			}
		}
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
			discord.Infof("Paper Trading")
		} else {
			discord.Infof("Real Trading")
		}
		sdk.Init()
		gsp.Init()
		blacklist.Init()
		_, err := scheduler.SingletonMode().Every(config.TheConfig.TickEverySeconds).Seconds().Do(
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
		scheduler.StartBlocking()
	case "playground":
		sdk.Init()
		timeAgo := 47 * time.Hour
		res, err := sdk.FuturesClient.NewMarkPriceKlinesService().
			Symbol("BCHUSDT").Interval("1m").StartTime(time.Now().Add(-timeAgo).Unix() * 1000).
			EndTime(time.Now().Add(-timeAgo+time.Minute).Unix() * 1000).Do(context.Background())
		if err != nil {
			log.Errorf("Error: %v", err)
			return
		}
		log.Info(utils.AsJson(res))
	}
}
