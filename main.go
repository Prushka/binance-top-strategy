package main

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/cleanup"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/gsp"
	"BinanceTopStrategies/notional"
	"BinanceTopStrategies/persistence"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/sql"
	"BinanceTopStrategies/utils"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"math"
	"os"
	"reflect"
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

func checkTakeProfits(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	for c, gpMax := range config.TheConfig.TakeProfits {
		gpMax = config.GetScaledProfits(gpMax, grid.InitialLeverage)
		if grid.LastRoi >= gpMax {
			gpLookBack := time.Duration(config.TheConfig.TakeProfitsMaxLookbackMinutes[c]) * time.Minute
			gpBlock := time.Duration(config.TheConfig.TakeProfitsBlockMinutes[c]) * time.Minute
			gridTracking := grid.GetTracking()
			lowerBound, _ := gridTracking.GetLocalWithin(gpLookBack)
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

func checkStopLoss(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	for c, sl := range config.TheConfig.StopLossMarkForRemoval {
		slAt := config.TheConfig.StopLossMarkForRemovalSLAt[c]
		if grid.LastRoi < sl {
			gsp.GridMarkForRemoval(grid.GID, slAt)
			discord.Infof(fmt.Sprintf("**stop loss marked for removal**: %.2f%%", (slAt)*100))
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
	poolDB := make([]*gsp.ChosenStrategyDB, 0)
	err = sql.GetDB().Scan(&poolDB, `SELECT * FROM bts.ThePool`)
	if err != nil {
		return err
	}
	utils.Time("Fetch the chosen")
	users := mapset.NewSet[int64]()
	for _, u := range poolDB {
		users.Add(u.UserID)
	}
	discord.Infof("Found %d strategies and %d users", len(poolDB), users.Cardinality())

	gsp.AddToPool(gsp.ToStrategies(poolDB))

	discord.Infof("### Current Grids:")
	sdk.ClearSessionSymbolPrice()
	err = gsp.UpdateOpenGrids(true)
	if err != nil {
		return err
	}
	gsp.SessionCancelledGIDs.Clear()
	toCancel := make(gsp.GridsToCancel)

	utils.Time("Fetch grids")
	count := 0
	grids := utils.MapValues(gsp.GGrids.GridsByGid)
	sort.Slice(grids, func(i, j int) bool {
		return grids[i].GID < grids[j].GID
	})
	for _, grid := range grids {
		discord.Infof(gsp.Display(gsp.GetPool().StrategiesBySID[grid.SID], grid, "", count+1, len(gsp.GGrids.GridsByGid)))
		isRunning, err := gsp.IsGridOriStrategyRunning(grid)
		if err != nil {
			return err
		}
		count++
		if !isRunning {
			toCancel.AddGridToCancel(grid, -999, "strategy not running")
			blacklist.AddSymbolDirection(grid.Symbol, grid.Direction, utils.TillNextRefresh(), "strategy sd not running")
		}
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
	if config.TheConfig.MaxPerChunk != -1 {
		invChunk = math.Min(usdt/chunks, config.TheConfig.MaxPerChunk)
	}
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
	if time.Now().Minute() < 19 {
		discord.Infof("Only trade after min 19, Skip")
		return nil
	}
	discord.Infof("### Opening new grids:")
	sessionSymbols := gsp.GGrids.ExistingSymbols.Clone()
	blacklistedInPool := mapset.NewSet[string]()
out:
	for c, s := range gsp.GetPool().Strategies {
		if s.RunningTime > 60*config.TheConfig.MaxLookBackBookingMinutes {
			log.Infof("Strategy running for more than %d hours, Skip", config.TheConfig.MaxLookBackBookingMinutes)
			// first pass, will run a second pass with strategy fetched to local
			continue
		}
		if strings.Contains(s.Symbol, "USDC") {
			discord.Infof("USDC symbol, Skip")
			continue
		}
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

		userStrategies := gsp.GetPool().StrategiesByUserId[s.UserID]
		for _, us := range userStrategies {
			if us.Symbol == s.Symbol && us.Direction != s.Direction {
				discord.Infof("Same symbol hedging, Skip")
				continue out
			}
		}

		userWl, err := gsp.UserWLCache.Get(fmt.Sprintf("%d", s.UserID))
		if err != nil {
			return err
		}
		userWlRatio := float64(userWl.Win) / float64(userWl.Total)
		shortRunningRatio := float64(userWl.ShortRunning) / float64(userWl.Total)
		if userWlRatio < 0.84 || (shortRunningRatio > 0.18 && userWlRatio != 1.0) {
			discord.Infof("User %d Win Loss Ratio %d/%d (%.2f), Short running ratio %.2f", s.UserID, userWl.Win, userWl.Total, userWlRatio, shortRunningRatio)
			continue
		}
		sInPool := s
		s, err := gsp.DiscoverGridRootStrategy(s.SID, s.Symbol, s.Direction, time.Duration(s.RunningTime)*time.Second)
		if err != nil {
			return err
		}
		if s == nil {
			discord.Errorf("Strategy candidate %d %s not running", sInPool.SID, sInPool.Symbol)
			continue
		}
		err = s.PopulateRois()
		if err != nil {
			return err
		}
		if s.RunningTime > 60*config.TheConfig.MaxLookBackBookingMinutes {
			discord.Infof("Strategy running for more than %d minutes, Skip", config.TheConfig.MaxLookBackBookingMinutes)
			continue
		}

		marketPrice, _ := sdk.GetSessionSymbolPrice(s.Symbol)
		minInvestment, _ := strconv.ParseFloat(s.MinInvestment, 64)
		notionalLeverage := notional.GetLeverage(s.Symbol, invChunk)
		leverage := utils.IntMin(notionalLeverage, config.TheConfig.PreferredLeverage)
		gap := s.StrategyParams.UpperLimit - s.StrategyParams.LowerLimit
		if s.PriceDifference < 0.1 {
			discord.Infof("Price difference too low, Skip")
			continue
		}
		switch s.Direction {
		case gsp.LONG:
			if marketPrice > s.StrategyParams.UpperLimit-gap*config.TheConfig.LongRangeDiff {
				discord.Infof("Market Price too high for long, Skip")
				continue
			}
		case gsp.NEUTRAL:
			minInvestPerLeverage := minInvestment * float64(s.StrategyParams.Leverage)
			minLeverage := int(math.Ceil(minInvestPerLeverage / invChunk))
			if minLeverage > config.TheConfig.MaxLeverage {
				discord.Infof("Investment too low %f, Min leverage %d, Skip", invChunk, minLeverage)
				continue
			} else if minLeverage > leverage {
				leverage = minLeverage
			}
			if marketPrice < s.StrategyParams.LowerLimit+gap*config.TheConfig.NeutralRangeDiff {
				discord.Infof("Market Price too low for neutral, Skip")
				continue
			}
			if marketPrice > s.StrategyParams.UpperLimit-gap*config.TheConfig.NeutralRangeDiff {
				discord.Infof("Market Price too high for neutral, Skip")
				continue
			}
		case gsp.SHORT:
			if marketPrice < s.StrategyParams.LowerLimit+gap*config.TheConfig.ShortRangeDiff {
				discord.Infof("Market Price too low for short, Skip")
				continue
			}
		}

		if s.StrategyParams.TriggerPrice != nil {
			triggerPrice, _ := strconv.ParseFloat(*s.StrategyParams.TriggerPrice, 64)
			marketPrice, _ := sdk.GetSessionSymbolPrice(s.Symbol)
			diff := math.Abs((triggerPrice - marketPrice) / marketPrice)
			if diff > config.TheConfig.TriggerRangeDiff {
				discord.Infof("Trigger Price difference too high, Skip, Trigger: %f, Market: %f, Diff: %f",
					triggerPrice, marketPrice, diff)
				continue
			}
		}

		if !s.MarketPriceWithinRange() {
			discord.Infof("Market Price not within range, Skip")
			continue
		}

		discord.Infof(gsp.Display(s, nil, "New", c+1, len(gsp.GetPool().Strategies)))
		errr := gsp.PlaceGrid(*s, invChunk, leverage)
		if !config.TheConfig.Paper {
			if errr != nil {
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

func configPop() {
	err := sql.Init()
	if err != nil {
		panic(err)
	}
	dbFields := make(map[string]reflect.StructField)
	for i := 0; i < reflect.ValueOf(config.TheConfig).Elem().NumField(); i++ {
		field := reflect.ValueOf(config.TheConfig).Elem().Field(i)
		tag := reflect.TypeOf(config.TheConfig).Elem().Field(i).Tag.Get("db")
		if tag == "" {
			continue
		}
		if field.Kind() == reflect.String {
			dbFields[tag] = reflect.TypeOf(config.TheConfig).Elem().Field(i)
		}
	}
	for k, v := range dbFields {
		var s string
		err := sql.GetDB().ScanOne(&s, `SELECT value FROM bts.config WHERE key = $1`, k)
		if err == nil {
			s = strings.ReplaceAll(s, "\n", "")
			reflect.ValueOf(config.TheConfig).Elem().FieldByName(v.Name).SetString(s)
		}
	}
}

func main() {
	config.Init()
	configPop()
	blocking := make(chan bool, 1)
	cleanup.InitSignalCallback(blocking)
	cleanup.AddOnStopFunc(func(_ os.Signal) {
		scheduler.Stop()
	})
	discord.Init()
	sdk.Init()
	switch config.TheConfig.Mode {
	case "trading":
		if config.TheConfig.Paper {
			discord.Errorf("Paper Trading")
		} else {
			discord.Errorf("Real Trading")
		}
		persistence.Init()
		panicOnErrorSec(scheduler.SingletonMode().Every(config.TheConfig.TickEverySeconds).Seconds().Do(
			func() {
				utils.ResetTime()
				t := time.Now()
				err := tick()
				if err != nil {
					discord.Errorf("Error: %v", err)
				}
				discord.Infof("*Run took: %v*", time.Since(t))
			},
		))
	case "SQL":
		panicOnErrorSec(scheduler.SingletonMode().Every(1).Minutes().Do(func() {
			t := time.Now()
			discord.Infof("### Prices: %v", time.Now().Format("2006-01-02 15:04:05"))
			err := gsp.PopulatePrices()
			if err != nil {
				discord.Errorf("Prices: %v", err)
			}
			discord.Infof("*Prices run took: %v*", time.Since(t))
		}))
		scheduler.StartAsync()
		for {
			t := time.Now()
			discord.Infof("### Strategies: %v", time.Now().Format("2006-01-02 15:04:05"))
			err := gsp.Scrape()
			if err != nil {
				discord.Errorf("Strategies: %v", err)
			}
			discord.Infof("*Strategies run took: %v*", time.Since(t))
			time.Sleep(5 * time.Minute)

			t = time.Now()
			discord.Infof("### Roi: %v", time.Now().Format("2006-01-02 15:04:05"))
			err = gsp.PopulateRoi()
			if err != nil {
				discord.Errorf("Roi: %v", err)
			}
			discord.Infof("*Roi run took: %v*", time.Since(t))
			time.Sleep(5 * time.Minute)
		}
	case "playground":
		utils.ResetTime()
		sdk.Init()
	}
	scheduler.StartAsync()
	<-blocking
}

func panicOnErrorSec(a interface{}, err error) {
	if err != nil {
		panic(err)
	}
}

func panicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
