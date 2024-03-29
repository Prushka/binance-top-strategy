package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/persistence"
	"BinanceTopStrategies/sdk"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"math"
	"strconv"
	"time"
)

var envOnGridsOpen = make(map[int]*envOnGridOpen)

type envOnGridOpen struct {
	SDRaw          map[string]int
	SDFiltered     map[string]int
	SDPairSpecific map[string]int
}

type GridTracking struct {
	LowestRoi             float64
	HighestRoi            float64
	TimeHighestRoi        time.Time
	TimeLowestRoi         time.Time
	TimeLastChange        time.Time
	ContinuousRoiGrowth   int
	ContinuousRoiLoss     int
	ContinuousRoiNoChange int
}

type TrackedGrids struct {
	TotalGridInitial float64
	TotalGridPnl     float64
	Shorts           mapset.Set[int]
	Longs            mapset.Set[int]
	Neutrals         mapset.Set[int]
	ExistingSIDs     mapset.Set[int]
	ExistingSymbols  mapset.Set[string]
	GridsByGid       map[int]*Grid
}

type Grid struct {
	TotalPnl               float64
	InitialValue           float64
	LastRoi                float64
	Tracking               *GridTracking
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

func persistGridEnvs(gid int) {
	if _, ok := envOnGridsOpen[gid]; !ok {
		envOnGridsOpen[gid] = &envOnGridOpen{SDRaw: Bundle.Raw.SymbolDirectionCount,
			SDFiltered:     Bundle.FilteredSortedBySD.SymbolDirectionCount,
			SDPairSpecific: Bundle.SDCountPairSpecific}
		err := persistence.Save(envOnGridsOpen, persistence.GridStatesFileName)
		if err != nil {
			discord.Infof("Error saving state on grid open: %v", err)
		}
	}
}

func GridSDCount(gid int, symbol, direction string, setType string) (int, int, float64) {
	sd := symbol + direction
	var currentSDCount int
	var sdCountWhenOpen int
	switch setType {
	case SDRaw:
		currentSDCount = Bundle.Raw.SymbolDirectionCount[sd]
		sdCountWhenOpen = envOnGridsOpen[gid].SDRaw[sd]
	case SDFiltered:
		currentSDCount = Bundle.FilteredSortedBySD.SymbolDirectionCount[sd]
		sdCountWhenOpen = envOnGridsOpen[gid].SDFiltered[sd]
	case SDPairSpecific:
		currentSDCount = Bundle.SDCountPairSpecific[sd]
		sdCountWhenOpen = envOnGridsOpen[gid].SDPairSpecific[sd]
	}
	ratio := float64(currentSDCount) / float64(sdCountWhenOpen)
	return currentSDCount, sdCountWhenOpen, ratio
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
	tracked.TotalGridInitial -= g.InitialValue
	tracked.TotalGridPnl -= g.TotalPnl
	delete(tracked.GridsByGid, g.GID)
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
	g.InitialValue = initial / float64(g.InitialLeverage)
	g.TotalPnl = profit + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
	g.LastRoi = g.TotalPnl / g.InitialValue
	updateTime := time.Now()
	prevG, ok := tracked.GridsByGid[g.GID]
	tracked.TotalGridInitial += g.InitialValue
	tracked.TotalGridPnl += g.TotalPnl
	if ok {
		tracked.TotalGridInitial -= prevG.InitialValue
		tracked.TotalGridPnl -= prevG.TotalPnl
		tracking := prevG.Tracking
		if g.LastRoi < tracking.LowestRoi {
			tracking.TimeLowestRoi = updateTime
		}
		if g.LastRoi > tracking.HighestRoi {
			tracking.TimeHighestRoi = updateTime
		}
		if g.LastRoi != prevG.LastRoi {
			tracking.TimeLastChange = updateTime
		}
		tracking.LowestRoi = math.Min(g.LastRoi, tracking.LowestRoi)
		tracking.HighestRoi = math.Max(g.LastRoi, tracking.HighestRoi)
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
		g.Tracking = tracking
	} else {
		g.Tracking = &GridTracking{
			LowestRoi:      g.LastRoi,
			HighestRoi:     g.LastRoi,
			TimeHighestRoi: updateTime,
			TimeLowestRoi:  updateTime,
			TimeLastChange: updateTime,
		}
		persistGridEnvs(g.GID)
	}
	tracked.GridsByGid[g.GID] = g
}

func (grid *Grid) String() string {
	tracking := grid.Tracking
	extendedProfit := ""
	extendedProfit = fmt.Sprintf(" [%.2f%% (%s), %.2f%% (%s)][+%d, -%d, %d (%s)]",
		tracking.LowestRoi*100,
		time.Since(tracking.TimeLowestRoi).Round(time.Second),
		tracking.HighestRoi*100,
		time.Since(tracking.TimeHighestRoi).Round(time.Second),
		tracking.ContinuousRoiGrowth, tracking.ContinuousRoiLoss, tracking.ContinuousRoiNoChange,
		time.Since(tracking.TimeLastChange).Round(time.Second),
	)
	formatSDRatio := func(setType string) string {
		currentSD, sdWhenOpen, ratio := GridSDCount(grid.GID, grid.Symbol, grid.Direction, setType)
		return fmt.Sprintf("%s: %d/%d/%.1f%%", setType, currentSD, sdWhenOpen, ratio*100)
	}
	realized, _ := strconv.ParseFloat(grid.GridProfit, 64)
	return fmt.Sprintf("*%d*, Realized: %.2f, Total: %.2f, =%.2f%%%s, %s, %s, %s",
		grid.GID,
		realized, grid.TotalPnl, grid.LastRoi*100, extendedProfit,
		formatSDRatio(SDRaw), formatSDRatio(SDFiltered), formatSDRatio(SDPairSpecific))
}
