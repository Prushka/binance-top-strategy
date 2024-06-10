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

func checkTakeProfits(grid *gsp.Grid, toCancel gsp.GridsToCancel) {
	for c, gpMax := range config.TheConfig.TakeProfits {
		gpMax = config.GetNormalized(gpMax, grid.InitialLeverage)
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
		if grid.LastRoi < config.GetNormalized(sl, grid.InitialLeverage) {
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
	discord.Infof("Days since cookie: %.2f", time.Since(config.TheConfig.CookieTimeParsed).Hours()/24)
	usdt, err := sdk.GetFuture("USDT")
	if err != nil {
		return err
	}
	usdc, err := sdk.GetFuture("USDC")
	if err != nil {
		return err
	}
	poolDB := make([]*gsp.ChosenStrategyDB, 0)
	err = sql.GetDB().Scan(&poolDB, `SELECT * FROM bts.ThePool`)
	if err != nil {
		return err
	}
	utils.Time("Fetched the pool")
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
		isRunning, err := gsp.IsGridOriStrategyRunning(grid)
		if err != nil {
			return err
		}
		count++
		oriStrategy := gsp.GetPool().StrategiesBySID[grid.SID]
		if isRunning != nil {
			if oriStrategy != nil {
				isRunning.UserMetricsDB = oriStrategy.UserMetricsDB
			}
			oriStrategy = isRunning
		}
		discord.Infof(gsp.Display(oriStrategy, grid, "", count, len(gsp.GGrids.GridsByGid)))
		if isRunning == nil {
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

	gridsOpen := gsp.GGrids.GridsByGid
	usdtChunks := gridsOpen.GetChunks("USDT")
	usdcChunks := gridsOpen.GetChunks("USDC")
	if config.TheConfig.MaxUSDTChunks-usdtChunks <= 0 &&
		config.TheConfig.MaxUSDCChunks-usdcChunks <= 0 && !config.TheConfig.Paper {
		discord.Infof("Max Chunks reached (%d/%d, %d/%d), No cancel - Skip current run", usdtChunks,
			config.TheConfig.MaxUSDTChunks, usdcChunks, config.TheConfig.MaxUSDCChunks)
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
	blacklistedInPool := mapset.NewSet[string]()
	if time.Now().Minute() < 19 {
		discord.Infof("Only trade after min 19, Skip")
		return nil
	}
	sessionSymbols := gsp.GGrids.ExistingSymbols.Clone()
	sessionSIDs := gsp.GGrids.ExistingSIDs.Clone()
	sessionNeutrals := gsp.GGrids.Neutrals.Cardinality()
	sortedStrategies := make(gsp.Strategies, 0)
	filteredUsers := mapset.NewSet[int]()
	for _, s := range gsp.GetPool().Strategies {
		userWl, err := gsp.UserWLCache.Get(fmt.Sprintf("%d", s.UserID))
		if err != nil {
			return err
		}
		if userWl.WinRatio < 0.819 || (userWl.ShortRunningRatio > 0.245 && userWl.WinRatio < 0.979) {
			log.Debugf("Skipped, %v", userWl)
			continue
		}
		sortedStrategies = append(sortedStrategies, s)
		filteredUsers.Add(s.UserID)
	}
	sort.Slice(sortedStrategies, func(i, j int) bool {
		iWL, _ := gsp.UserWLCache.Get(fmt.Sprintf("%d", sortedStrategies[i].UserID))
		jWL, _ := gsp.UserWLCache.Get(fmt.Sprintf("%d", sortedStrategies[j].UserID))
		return iWL.WinRatio > jWL.WinRatio
	})
	longs, shorts, neutrals := sortedStrategies.GetLSN()
	discord.Infof("Filtered strategies by WL: %d, %d users | L/S/N: %d, %d, %d", len(sortedStrategies), filteredUsers.Cardinality(), longs, shorts, neutrals)
	var place func(maxChunks, existingChunks int, currency, overwriteQuote string, balance float64) error
	place = func(maxChunks, existingChunks int, currency, overwriteQuote string, balance float64) error {
		total := balance + gsp.GGrids.TotalGridPnl[currency] + gsp.GGrids.TotalGridInitial[currency]
		total *= 1 - config.TheConfig.Reserved
		chunksInt := maxChunks - existingChunks
		chunks := float64(chunksInt)
		if chunksInt == 0 {
			discord.Infof("Max Chunks reached for %s %s, Skip", currency, overwriteQuote)
			return nil
		}
		invChunk := balance / chunks
		if config.TheConfig.MaxPerChunk != -1 {
			invChunk = math.Min(balance/chunks, config.TheConfig.MaxPerChunk)
		}
		idealInvChunk := total / float64(maxChunks)
		discord.Infof("### Opening %d chunks for %s %s (%.2f, %.2f):", chunksInt, currency, overwriteQuote, idealInvChunk, invChunk)
		invChunk = math.Min(invChunk, idealInvChunk)
		if invChunk < config.TheConfig.MinInvestmentPerChunk && !config.TheConfig.Paper {
			adjusted := int(balance/config.TheConfig.MinInvestmentPerChunk) + existingChunks
			discord.Infof("Investment too low (%f), Adjusting max chunks to %d", invChunk, adjusted)
			return place(adjusted, existingChunks, currency, overwriteQuote, balance)
		}
		invChunk = float64(int(invChunk))
	out:
		for c, s := range sortedStrategies {
			if s.RunningTime > 60*config.TheConfig.MaxLookBackBookingMinutes {
				log.Infof("Strategy running for more than %d minutes, Skip", config.TheConfig.MaxLookBackBookingMinutes)
				continue
			}

			userStrategies := gsp.GetPool().StrategiesByUserId[s.UserID]
			for _, us := range userStrategies {
				if us.Symbol == s.Symbol && us.Direction != s.Direction {
					discord.Infof("Same symbol hedging, Skip")
					continue out
				}
			}
			strategyQuote := s.Symbol[len(s.Symbol)-4:]
			if strategyQuote != currency {
				log.Infof("wrong quote (%s, %s), Skip", currency, strategyQuote)
				continue
			}

			if sessionSIDs.Contains(s.SID) {
				discord.Infof("* Strategy %d - %s exists in open grids, Skip", s.SID, s.SD())
				continue
			}
			if sessionSymbols.Contains(s.Symbol) ||
				sessionSymbols.Contains(utils.OverwriteQuote(s.Symbol, "USDT", 4)) ||
				sessionSymbols.Contains(utils.OverwriteQuote(s.Symbol, "USDC", 4)) {
				log.Infof("Symbol exists in open grids, Skip")
				continue
			}

			if sessionNeutrals >= config.TheConfig.MaxNeutrals && s.Direction == gsp.NEUTRAL {
				discord.Infof("Max Neutrals reached (%d/%d), Skip", sessionNeutrals, config.TheConfig.MaxNeutrals)
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
			userWl, err := gsp.UserWLCache.Get(fmt.Sprintf("%d", s.UserID))
			if err != nil {
				return err
			}
			discord.Infof(userWl.String())
			sInPool := s
			s, err := gsp.DiscoverRootStrategy(s.SID, s.Symbol, s.Direction, time.Duration(s.RunningTime)*time.Second)
			if err != nil {
				return err
			}
			if s == nil {
				discord.Infof("Strategy candidate %d %s not running", sInPool.SID, sInPool.Symbol)
				continue
			}
			if s.RunningTime > 60*config.TheConfig.MaxLookBackBookingMinutes {
				discord.Infof("Strategy %d running for more than %d minutes, Skip", s.SID, config.TheConfig.MaxLookBackBookingMinutes)
				continue
			}
			err = s.PopulateRois()
			if err != nil {
				return err
			}

			marketPrice, _ := sdk.GetSessionSymbolPrice(s.Symbol)
			minInvestment, _ := strconv.ParseFloat(s.MinInvestment, 64)
			notionalLeverage := notional.GetLeverage(s.Symbol, invChunk)
			leverage := utils.IntMin(notionalLeverage, config.TheConfig.PreferredLeverage)
			gap := s.StrategyParams.UpperLimit - s.StrategyParams.LowerLimit
			priceDiff := s.StrategyParams.UpperLimit/s.StrategyParams.LowerLimit - 1
			minPriceDiff := 0.0
			minWinRatio := 0.819
			notionalMax := notional.MaxLeverage(s.Symbol)
			switch s.Direction {
			case gsp.LONG:
				if marketPrice > s.StrategyParams.UpperLimit-gap*config.TheConfig.LongRangeDiff {
					discord.Infof("Market Price too high for long, Skip")
					continue
				}
				minPriceDiff = 0.02
			case gsp.NEUTRAL:
				minInvestPerLeverage := minInvestment * float64(s.StrategyParams.Leverage)
				minLeverage := int(math.Ceil(minInvestPerLeverage / invChunk))
				if minLeverage > config.TheConfig.MaxLeverage || minLeverage > notionalMax {
					discord.Infof("%s Investment too low %f, Min leverage %d, Notional Max %d, Skip", s.Symbol, invChunk, minLeverage, notionalMax)
					continue
				} else if minLeverage > leverage {
					leverage = minLeverage
				}
				minPriceDiff = 0.08
				minWinRatio = 0.839
			case gsp.SHORT:
				if marketPrice < s.StrategyParams.LowerLimit+gap*config.TheConfig.ShortRangeDiff {
					discord.Infof("Market Price too low for short, Skip")
					continue
				}
				minPriceDiff = 0.02
			}
			if priceDiff < minPriceDiff {
				discord.Infof("Price difference too low, Skip")
				continue
			}
			if userWl.WinRatio < minWinRatio {
				log.Infof("Win Ratio too low for long, Skip")
				continue
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

			if overwriteQuote != "" {
				s.Symbol = utils.OverwriteQuote(s.Symbol, overwriteQuote, len(currency))
			}
		place:
			discord.Infof(gsp.Display(s, nil, "New", c+1, len(sortedStrategies)))
			errr := gsp.PlaceGrid(*s, invChunk, leverage)
			if !config.TheConfig.Paper {
				if errr != nil {
					discord.Infof("**Error placing grid: %v**", errr)
					if strings.Contains(errr.Error(), "Create grid too frequently") {
						discord.Infof("**Too Frequent Error, Skip Current Run**")
						break
					}
					if (strings.Contains(errr.Error(), "notional") || strings.Contains(errr.Error(), "margin is below minimum")) &&
						s.Direction != gsp.NEUTRAL && leverage < config.TheConfig.MaxLeverage && leverage < notionalMax {
						leverage += 3
						if leverage > config.TheConfig.MaxLeverage {
							leverage = config.TheConfig.MaxLeverage
						}
						discord.Infof("Increase leverage to %d", leverage)
						goto place
					}
				} else {
					discord.Actionf(gsp.Display(s, nil, "**Opened Grid**", c+1, len(sortedStrategies)))
					chunksInt -= 1
					sessionSymbols.Add(s.Symbol)
					sessionSIDs.Add(s.SID)
					if s.Direction == gsp.NEUTRAL {
						sessionNeutrals++
					}
					if chunksInt <= 0 {
						break
					}
				}
			}
		}
		return nil
	}
	err = place(config.TheConfig.MaxUSDCChunks, usdcChunks, "USDT", "USDC", usdc)
	if err != nil {
		return err
	}
	err = place(config.TheConfig.MaxUSDTChunks, usdtChunks, "USDT", "", usdt)
	if err != nil {
		return err
	}
	err = place(config.TheConfig.MaxUSDCChunks, usdcChunks, "USDC", "", usdc)
	if err != nil {
		return err
	}

	if blacklistedInPool.Cardinality() > 0 {
		discord.Infof("Blacklisted in pool: %s", blacklistedInPool)
	}
	utils.Time("Place/Cancel done")
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
	if config.TheConfig.CookieTime != "" {
		i, err := strconv.ParseInt(config.TheConfig.CookieTime, 10, 64)
		if err != nil {
			panic(err)
		}
		config.TheConfig.CookieTimeParsed = time.Unix(i, 0)
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
		panicOnErrorSec(scheduler.SingletonMode().Every(3).Minutes().Do(func() {
			t := time.Now()
			discord.Infof("### Prices: %v", time.Now().Format("2006-01-02 15:04:05"))
			err := gsp.PopulatePrices()
			if err != nil {
				discord.Errorf("Prices: %v", err)
			}
			discord.Infof("*Prices run took: %v*", time.Since(t))
		}))
		panicOnErrorSec(scheduler.SingletonMode().Every(30).Minutes().Do(func() {
			t := time.Now()
			discord.Infof("### Refresh TheChosen: %v", time.Now().Format("2006-01-02 15:04:05"))
			err := gsp.RefreshTheChosen()
			if err != nil {
				discord.Errorf("TheChosen: %v", err)
			}
			discord.Infof("*TheChosen run took: %v*", time.Since(t))
		}))
		scheduler.StartAsync()
		for {
			t := time.Now()
			discord.Infof("### Roi: %v", time.Now().Format("2006-01-02 15:04:05"))
			err := gsp.PopulateRoi()
			if err != nil {
				discord.Errorf("Roi: %v", err)
			}
			discord.Infof("*Roi run took: %v*", time.Since(t))

			_ = gsp.Scrape(gsp.FUTURE, "FUTURE")
			time.Sleep(60 * time.Second)
			_ = gsp.Scrape(gsp.SPOT, "SPOT")
			time.Sleep(60 * time.Second)
		}
	case "playground":
		utils.ResetTime()
		poolDB := make([]*gsp.ChosenStrategyDB, 0)
		err := sql.GetDB().Scan(&poolDB, `SELECT * FROM bts.ThePool`)
		if err != nil {
			panic(err)
		}
		utils.Time("Fetched the pool")
		users := mapset.NewSet[int64]()
		for _, u := range poolDB {
			users.Add(u.UserID)
		}
		for _, user := range users.ToSlice() {
			_, err := gsp.UserWLCache.Get(fmt.Sprintf("%d", user))
			if err != nil {
				panic(err)
			}
		}
		LWUsers := mapset.NewSet[int64]()
		for _, user := range users.ToSlice() {
			wl, err := gsp.UserWLCache.Get(fmt.Sprintf("%d", user))
			if err != nil {
				panic(err)
			}
			if wl.WinRatio > 0.8 {
				log.Info(wl)
				LWUsers.Add(user)
			}
		}
		for _, s := range poolDB {
			if LWUsers.Contains(s.UserID) && s.Direction == gsp.SHORT {
				log.Infof(utils.AsJson(s))
			}
		}
	}
	scheduler.StartAsync()
	<-blocking
}

func timeNowHourPrecision() time.Time {
	t := time.Now()
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.Local)
}

func getTestStrategy(id int) *gsp.Strategy {
	s := gsp.ChosenStrategyDB{}
	err := sql.GetDB().ScanOne(&s, `SELECT * FROM bts.strategy WHERE strategy_id = $1`, id)
	if err != nil {
		panic(err)
	}
	ss := gsp.ToStrategies([]*gsp.ChosenStrategyDB{&s})
	return ss[0]
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
