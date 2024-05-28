package gsp

import (
	"BinanceTopStrategies/sdk"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"math"
	"strconv"
	"time"
)

var TheGridEnv = make(map[int]*GridEnv)

type Profit struct {
	Profit float64
	Time   time.Time
}

type Profits []Profit

type GridEnv struct {
	Tracking *GridTracking
}

type GridTracking struct {
	LowestRoi             float64
	HighestRoi            float64
	TimeHighestRoi        time.Time
	TimeLowestRoi         time.Time
	TimeLastChange        time.Time
	LocalProfits          Profits
	ContinuousRoiGrowth   int
	ContinuousRoiLoss     int
	ContinuousRoiNoChange int
}

func (tracking *GridTracking) GetLocalWithin(duration time.Duration) (float64, float64) {
	earliest := time.Now().Add(-duration)
	lowest := 10000000.0
	highest := -10000000.0
	for i := len(tracking.LocalProfits) - 1; i >= 0; i-- {
		if tracking.LocalProfits[i].Time.Before(earliest) {
			break
		}
		lowest = math.Min(lowest, tracking.LocalProfits[i].Profit)
		highest = math.Max(highest, tracking.LocalProfits[i].Profit)
	}
	return lowest, highest
}

type TrackedGrids struct {
	TotalGridInitial map[string]float64
	TotalGridPnl     map[string]float64
	Shorts           mapset.Set[int]
	Longs            mapset.Set[int]
	Neutrals         mapset.Set[int]
	ExistingSIDs     mapset.Set[int]
	ExistingSymbols  mapset.Set[string]
	GridsByGid       GridsMap
}

type GridsMap map[int]*Grid

func (grid *Grid) IsQuote(quote string) bool {
	token := grid.Symbol[len(grid.Symbol)-len(quote):]
	return token == quote
}

func (gm GridsMap) GetChunks(quote string) int {
	chunks := 0
	for _, g := range gm {
		if g.IsQuote(quote) {
			chunks++
		}
	}
	return chunks
}

type Grid struct {
	TotalPnl               float64
	InitialValue           float64
	LastRoi                float64
	BelowLowerLimit        bool
	AboveUpperLimit        bool
	GID                    int    `json:"strategyId"`
	RootUserID             int    `json:"rootUserId"`
	StrategyUserID         int    `json:"strategyUserId"`
	StrategyAccountID      int    `json:"strategyAccountId"`
	Symbol                 string `json:"symbol"`
	StrategyStatus         string `json:"strategyStatus"`
	BookTime               int64  `json:"bookTime"`
	TriggerTime            int64  `json:"triggerTime"`
	UpdateTime             int64  `json:"updateTime"`
	GridInitialValue       string `json:"gridInitialValue"`
	InitialLeverage        int    `json:"initialLeverage"`
	GridProfit             string `json:"gridProfit"`
	Direction              string `json:"direction"`
	MatchedPnl             string `json:"matchedPnl"`
	UnmatchedAvgPrice      string `json:"unmatchedAvgPrice"`
	UnmatchedQty           string `json:"unmatchedQty"`
	UnmatchedFee           string `json:"unmatchedFee"`
	GridEntryPrice         string `json:"gridEntryPrice"`
	GridPosition           string `json:"gridPosition"`
	Version                int    `json:"version"`
	CopyCount              int    `json:"copyCount"`
	SID                    int    `json:"copiedStrategyId"`
	Sharing                bool   `json:"sharing"`
	IsSubAccount           bool   `json:"isSubAccount"`
	StrategyAmount         string `json:"strategyAmount"`
	TrailingUp             bool   `json:"trailingUp"`
	TrailingDown           bool   `json:"trailingDown"`
	TrailingStopUpperLimit bool   `json:"trailingStopUpperLimit"`
	TrailingStopLowerLimit bool   `json:"trailingStopLowerLimit"`
	OrderCurrency          string `json:"orderCurrency"`
	GridUpperLimit         string `json:"gridUpperLimit"`
	GridLowerLimit         string `json:"gridLowerLimit"`
	MatchedCount           int    `json:"matchedCount"`
	GridCount              int    `json:"gridCount"`
	PerGridQuoteQty        string `json:"perGridQuoteQty"`
	PerGridQty             string `json:"perGridQty"`
	FundingFee             string `json:"fundingFee"`
	MarginType             string `json:"marginType"`
}

func (grid *Grid) GetEnv() *GridEnv {
	return TheGridEnv[grid.GID]
}

func (grid *Grid) SetEnv(env *GridEnv) {
	TheGridEnv[grid.GID] = env
}

func (grid *Grid) GetTracking() *GridTracking {
	return grid.GetEnv().Tracking
}

func (tracked *TrackedGrids) GetGridBySID(sid int) *Grid {
	for _, g := range tracked.GridsByGid {
		if g.SID == sid {
			return g
		}
	}
	return nil
}

func (tracked *TrackedGrids) calcPnl(g *Grid, multiplier float64) {
	if g.IsQuote("USDT") {
		tracked.TotalGridInitial["USDT"] += multiplier * g.InitialValue
		tracked.TotalGridPnl["USDT"] += multiplier * g.TotalPnl
	} else if g.IsQuote("USDC") {
		tracked.TotalGridInitial["USDC"] += multiplier * g.InitialValue
		tracked.TotalGridPnl["USDC"] += multiplier * g.TotalPnl
	}
}

func (tracked *TrackedGrids) remove(id int) {
	g, ok := tracked.GridsByGid[id]
	if !ok {
		return
	}
	tracked.ExistingSymbols.Remove(g.Symbol)
	tracked.ExistingSIDs.Remove(g.SID)
	if g.Direction == DirectionMap[LONG] {
		tracked.Longs.Remove(g.GID)
	} else if g.Direction == DirectionMap[SHORT] {
		tracked.Shorts.Remove(g.GID)
	} else {
		tracked.Neutrals.Remove(g.GID)
	}
	tracked.calcPnl(g, -1)
	delete(tracked.GridsByGid, g.GID)
	delete(TheGridEnv, g.GID)
}

func (tracked *TrackedGrids) add(g *Grid, trackContinuous bool) {
	tracked.ExistingSymbols.Add(g.Symbol)
	tracked.ExistingSIDs.Add(g.SID)

	if g.Direction == DirectionMap[LONG] {
		tracked.Longs.Add(g.GID)
	} else if g.Direction == DirectionMap[SHORT] {
		tracked.Shorts.Add(g.GID)
	} else {
		tracked.Neutrals.Add(g.GID)
	}
	initial, _ := strconv.ParseFloat(g.GridInitialValue, 64)
	profit, _ := strconv.ParseFloat(g.GridProfit, 64)
	fundingFee, _ := strconv.ParseFloat(g.FundingFee, 64)
	position, _ := strconv.ParseFloat(g.GridPosition, 64)
	entryPrice, _ := strconv.ParseFloat(g.GridEntryPrice, 64)
	marketPrice, _ := sdk.GetSessionSymbolPrice(g.Symbol)
	lowerLimit, _ := strconv.ParseFloat(g.GridLowerLimit, 64)
	upperLimit, _ := strconv.ParseFloat(g.GridUpperLimit, 64)
	g.BelowLowerLimit = marketPrice < lowerLimit
	g.AboveUpperLimit = marketPrice > upperLimit
	g.InitialValue = initial / float64(g.InitialLeverage)
	g.TotalPnl = profit + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
	g.LastRoi = g.TotalPnl / g.InitialValue
	updateTime := time.Now()
	prevG, ok := tracked.GridsByGid[g.GID]
	tracked.calcPnl(g, 1)
	if ok {
		tracked.calcPnl(prevG, -1)
	}

	if g.GetEnv() == nil {
		g.SetEnv(&GridEnv{Tracking: &GridTracking{
			LowestRoi:      g.LastRoi,
			HighestRoi:     g.LastRoi,
			TimeHighestRoi: updateTime,
			TimeLowestRoi:  updateTime,
			TimeLastChange: updateTime,
		}})
	} else {
		tracking := g.GetTracking()
		if g.LastRoi < tracking.LowestRoi {
			tracking.TimeLowestRoi = updateTime
		}
		if g.LastRoi > tracking.HighestRoi {
			tracking.TimeHighestRoi = updateTime
		}
		tracking.LowestRoi = math.Min(g.LastRoi, tracking.LowestRoi)
		tracking.HighestRoi = math.Max(g.LastRoi, tracking.HighestRoi)

		if prevG != nil {
			if g.LastRoi != prevG.LastRoi {
				tracking.TimeLastChange = updateTime
			}
			if trackContinuous {
				if g.LastRoi > prevG.LastRoi {
					tracking.ContinuousRoiGrowth += 1
					tracking.ContinuousRoiLoss = 0
					tracking.ContinuousRoiNoChange = 0
				} else if g.LastRoi < prevG.LastRoi {
					tracking.ContinuousRoiLoss += 1
					tracking.ContinuousRoiGrowth = 0
					tracking.ContinuousRoiNoChange = 0
				} else {
					tracking.ContinuousRoiNoChange += 1
					tracking.ContinuousRoiGrowth = 0
					tracking.ContinuousRoiLoss = 0
				}
			}
		}
	}
	tracking := g.GetTracking()
	tracking.LocalProfits = append(tracking.LocalProfits, Profit{Profit: g.LastRoi, Time: updateTime})
	tracked.GridsByGid[g.GID] = g
}

func (grid *Grid) GetRunTime() time.Duration {
	return time.Duration(time.Now().Unix()-grid.BookTime/1000) * time.Second
}

func (grid *Grid) MarketPriceWithinRange() bool {
	marketPrice, _ := sdk.GetSessionSymbolPrice(grid.Symbol)
	lowerLimit, _ := strconv.ParseFloat(grid.GridLowerLimit, 64)
	upperLimit, _ := strconv.ParseFloat(grid.GridUpperLimit, 64)
	return marketPrice > lowerLimit && marketPrice < upperLimit
}

func (grid *Grid) String() string {
	tracking := grid.GetTracking()
	extendedProfit := ""
	extendedProfit = fmt.Sprintf(" [%.2f%% (%s), %.2f%% (%s)][+%d, -%d, %d (%s)]",
		tracking.LowestRoi*100,
		time.Since(tracking.TimeLowestRoi).Round(time.Second),
		tracking.HighestRoi*100,
		time.Since(tracking.TimeHighestRoi).Round(time.Second),
		tracking.ContinuousRoiGrowth, tracking.ContinuousRoiLoss, tracking.ContinuousRoiNoChange,
		time.Since(tracking.TimeLastChange).Round(time.Second),
	)
	realized, _ := strconv.ParseFloat(grid.GridProfit, 64)
	outOfRange := ""
	if !grid.MarketPriceWithinRange() {
		outOfRange = "**[OOR]** "
	}
	return fmt.Sprintf("*%d*, Realized: %.2f, **%.2f%%**, Total: %.2f, %s**%.2f%%**%s",
		grid.GID,
		realized, realized/grid.InitialValue*100, grid.TotalPnl, outOfRange, grid.LastRoi*100, extendedProfit)
}
