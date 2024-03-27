package main

import (
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"math"
	"strconv"
	"time"
)

type PlaceGridRequest struct {
	Symbol                 string `json:"symbol"`
	Direction              string `json:"direction"`
	Leverage               int    `json:"leverage"`
	MarginType             string `json:"marginType"`
	GridType               string `json:"gridType"`
	GridCount              int    `json:"gridCount"`
	GridLowerLimit         string `json:"gridLowerLimit"`
	GridUpperLimit         string `json:"gridUpperLimit"`
	GridInitialValue       string `json:"gridInitialValue"`
	Cos                    bool   `json:"cos"`
	Cps                    bool   `json:"cps"`
	TrailingUp             bool   `json:"trailingUp,omitempty"`
	TrailingDown           bool   `json:"trailingDown,omitempty"`
	OrderCurrency          string `json:"orderCurrency"`
	StopUpperLimit         string `json:"stopUpperLimit,omitempty"`
	StopLowerLimit         string `json:"stopLowerLimit,omitempty"`
	TriggerPrice           string `json:"triggerPrice,omitempty"`
	TriggerType            string `json:"triggerType,omitempty"`
	TrailingStopUpperLimit bool   `json:"trailingStopUpperLimit"`
	TrailingStopLowerLimit bool   `json:"trailingStopLowerLimit"`
	StopTriggerType        string `json:"stopTriggerType,omitempty"`
	ClientStrategyID       string `json:"clientStrategyId,omitempty"`
	CopiedStrategyID       int    `json:"copiedStrategyId"`
}

type PlaceGridResponse struct {
	Data struct {
		StrategyID       int    `json:"strategyId"`
		ClientStrategyID string `json:"clientStrategyId"`
		StrategyType     string `json:"strategyType"`
		StrategyStatus   string `json:"strategyStatus"`
		UpdateTime       int64  `json:"updateTime"`
	} `json:"data"`
	BinanceBaseResponse
}

type GridTracking struct {
	lowestRoi             float64
	highestRoi            float64
	timeHighestRoi        time.Time
	timeLowestRoi         time.Time
	continuousRoiGrowth   int
	continuousRoiLoss     int
	continuousRoiNoChange int
}

type Grid struct {
	totalPnl               float64
	initialValue           float64
	lastRoi                float64
	tracking               *GridTracking
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

type OpenGridResponse struct {
	Grids []*Grid `json:"data"`
	BinanceBaseResponse
}

func newTrackedGrids() *TrackedGrids {
	return &TrackedGrids{
		shorts:          mapset.NewSet[int](),
		longs:           mapset.NewSet[int](),
		neutrals:        mapset.NewSet[int](),
		existingIds:     mapset.NewSet[int](),
		existingSymbols: mapset.NewSet[string](),
		gridsByUid:      make(map[int]*Grid),
	}
}

func (tracked *TrackedGrids) Remove(id int) {
	g, ok := tracked.gridsByUid[id]
	if !ok {
		return
	}
	tracked.existingSymbols.Remove(g.Symbol)
	tracked.existingIds.Remove(g.SID)
	if g.Direction == DirectionMap[LONG] {
		tracked.longs.Remove(g.GID)
	} else if g.Direction == DirectionMap[SHORT] {
		tracked.shorts.Remove(g.GID)
	} else {
		tracked.neutrals.Remove(g.GID)
	}
	tracked.totalGridInitial -= g.initialValue
	tracked.totalGridPnl -= g.totalPnl
	delete(tracked.gridsByUid, g.GID)
}

func (tracked *TrackedGrids) Add(g *Grid, trackContinuous bool) {
	tracked.existingSymbols.Add(g.Symbol)
	tracked.existingIds.Add(g.SID)

	if g.Direction == DirectionMap[LONG] {
		tracked.longs.Add(g.GID)
	} else if g.Direction == DirectionMap[SHORT] {
		tracked.shorts.Add(g.GID)
	} else {
		tracked.neutrals.Add(g.GID)
	}
	initial, _ := strconv.ParseFloat(g.GridInitialValue, 64)
	profit, _ := strconv.ParseFloat(g.GridProfit, 64)
	fundingFee, _ := strconv.ParseFloat(g.FundingFee, 64)
	position, _ := strconv.ParseFloat(g.GridPosition, 64)
	entryPrice, _ := strconv.ParseFloat(g.GridEntryPrice, 64)
	marketPrice, _ := fetchMarketPrice(g.Symbol)
	g.initialValue = initial / float64(g.InitialLeverage)
	g.totalPnl = profit + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
	g.lastRoi = g.totalPnl / g.initialValue
	updateTime := time.Now()
	oldG, ok := tracked.gridsByUid[g.GID]
	tracked.totalGridInitial += g.initialValue
	tracked.totalGridPnl += g.totalPnl
	if ok {
		tracked.totalGridInitial -= oldG.initialValue
		tracked.totalGridPnl -= oldG.totalPnl
		tracking := oldG.tracking
		if g.lastRoi < tracking.lowestRoi {
			tracking.timeLowestRoi = updateTime
		}
		if g.lastRoi > tracking.highestRoi {
			tracking.timeHighestRoi = updateTime
		}
		tracking.lowestRoi = math.Min(g.lastRoi, tracking.lowestRoi)
		tracking.highestRoi = math.Max(g.lastRoi, tracking.highestRoi)

		if trackContinuous {
			if g.lastRoi > oldG.lastRoi {
				tracking.continuousRoiGrowth += 1
				tracking.continuousRoiLoss = 0
				tracking.continuousRoiNoChange = 0
			} else if g.lastRoi < oldG.lastRoi {
				tracking.continuousRoiLoss += 1
				tracking.continuousRoiGrowth = 0
				tracking.continuousRoiNoChange = 0
			} else {
				tracking.continuousRoiNoChange += 1
				tracking.continuousRoiGrowth = 0
				tracking.continuousRoiLoss = 0
			}
		}
		g.tracking = tracking
	} else {
		g.tracking = &GridTracking{
			lowestRoi:      g.lastRoi,
			highestRoi:     g.lastRoi,
			timeHighestRoi: updateTime,
			timeLowestRoi:  updateTime,
		}
	}
	tracked.gridsByUid[g.GID] = g
}

type TrackedGrids struct {
	totalGridInitial float64
	totalGridPnl     float64
	shorts           mapset.Set[int]
	longs            mapset.Set[int]
	neutrals         mapset.Set[int]
	existingIds      mapset.Set[int]
	existingSymbols  mapset.Set[string]
	gridsByUid       map[int]*Grid
}

func (tracked *TrackedGrids) findGridIdsByStrategyId(ids ...int) mapset.Set[int] {
	gridIds := mapset.NewSet[int]()
	idsSet := mapset.NewSet[int](ids...)
	for _, g := range tracked.gridsByUid {
		if idsSet.Contains(g.SID) {
			gridIds.Add(g.GID)
		}
	}
	return gridIds
}

func updateOpenGrids(trackContinuous bool) error {
	url := "https://www.binance.com/bapi/futures/v2/private/future/grid/query-open-grids"
	res, _, err := privateRequest(url, "POST", nil, &OpenGridResponse{})
	if err != nil {
		return err
	}
	currentIds := mapset.NewSet[int]()
	for _, grid := range res.Grids {
		gGrids.Add(grid, trackContinuous)
		currentIds.Add(grid.GID)
	}
	for _, g := range gGrids.gridsByUid {
		if !currentIds.Contains(g.GID) {
			gGrids.Remove(g.GID)
			DiscordWebhook(display(globalStrategies[g.SID], g, "Gone", 0, 0))
		}
	}
	DiscordWebhook(fmt.Sprintf("Open Pairs: %v, Open Ids: %v, Initial: %f, TotalPnL: %f, C: %f, L/S/N: %d/%d/%d",
		gGrids.existingSymbols, gGrids.existingIds, gGrids.totalGridInitial, gGrids.totalGridPnl, gGrids.totalGridPnl+gGrids.totalGridInitial,
		gGrids.longs.Cardinality(), gGrids.shorts.Cardinality(), gGrids.neutrals.Cardinality()))
	return nil
}

func (grid *Grid) String() string {
	tracking := grid.tracking
	extendedProfit := ""
	extendedProfit = fmt.Sprintf(" [%.2f%% (%s), %.2f%% (%s)][+%d, -%d, %d]",
		tracking.lowestRoi*100,
		time.Since(tracking.timeLowestRoi).Round(time.Second),
		tracking.highestRoi*100,
		time.Since(tracking.timeHighestRoi).Round(time.Second),
		tracking.continuousRoiGrowth, tracking.continuousRoiLoss, tracking.continuousRoiNoChange)
	d := time.Now().Unix() - grid.BookTime/1000
	dDuration := time.Duration(d) * time.Second
	notional := int(grid.initialValue * float64(grid.InitialLeverage))
	return fmt.Sprintf("%.2fX%d=%d, %s, Realized: %s, Total: %f, Profit: %f%%%s, %s-%s",
		grid.initialValue, grid.InitialLeverage, notional, dDuration,
		grid.GridProfit, grid.totalPnl, grid.lastRoi*100, extendedProfit, grid.GridLowerLimit, grid.GridUpperLimit)
}

func closeGrid(strategyId int) error {
	if TheConfig.Paper {
		log.Infof("Paper mode, not closing grid")
		return nil
	}
	url := "https://www.binance.com/bapi/futures/v1/private/future/grid/close-grid"
	payload := map[string]interface{}{
		"strategyId": strategyId,
	}
	_, _, err := privateRequest(url, "POST", payload, &BinanceBaseResponse{})
	return err
}

func placeGrid(strategy Strategy, initialUSDT float64) error {
	if _, ok := DirectionMap[strategy.Direction]; !ok {
		return fmt.Errorf("invalid direction: %d", strategy.Direction)
	}
	leverage := TheConfig.MaxLeverage
	if strategy.StrategyParams.Leverage < leverage {
		leverage = strategy.StrategyParams.Leverage
	}
	leverage = getLeverage(strategy.Symbol, initialUSDT, leverage)
	payload := &PlaceGridRequest{
		Symbol:                 strategy.Symbol,
		Direction:              DirectionMap[strategy.Direction],
		Leverage:               leverage,
		MarginType:             "CROSSED",
		GridType:               strategy.StrategyParams.Type,
		GridCount:              strategy.StrategyParams.GridCount,
		GridLowerLimit:         strategy.StrategyParams.LowerLimit,
		GridUpperLimit:         strategy.StrategyParams.UpperLimit,
		GridInitialValue:       fmt.Sprintf("%.2f", initialUSDT*float64(leverage)),
		Cos:                    true,
		Cps:                    true,
		TrailingUp:             strategy.StrategyParams.TrailingUp,
		TrailingDown:           strategy.StrategyParams.TrailingDown,
		OrderCurrency:          "BASE",
		ClientStrategyID:       "ctrc_web_" + generateRandomNumberUUID(),
		CopiedStrategyID:       strategy.SID,
		TrailingStopLowerLimit: false, // !!t[E.w2.stopLowerLimit]
		TrailingStopUpperLimit: false, // !1 in js
	}
	if payload.TrailingUp || payload.TrailingDown {
		payload.OrderCurrency = "QUOTE"
		if strategy.StrategyParams.StopLowerLimit != nil {
			payload.TrailingStopLowerLimit = true
		}
	}
	if strategy.StrategyParams.TriggerPrice != nil {
		payload.TriggerPrice = *strategy.StrategyParams.TriggerPrice
		payload.TriggerType = "MARK_PRICE"
	}
	if strategy.StrategyParams.StopUpperLimit != nil {
		payload.StopUpperLimit = *strategy.StrategyParams.StopUpperLimit
		payload.StopTriggerType = "MARK_PRICE"
	}
	if strategy.StrategyParams.StopLowerLimit != nil {
		payload.StopLowerLimit = *strategy.StrategyParams.StopLowerLimit
		payload.StopTriggerType = "MARK_PRICE"
	}
	s, _ := json.Marshal(payload)
	DiscordWebhookS(DiscordJson(string(s)), OrderWebhook)
	if TheConfig.Paper {
		log.Infof("Paper mode, not placing grid")
		return nil
	}
	_, _, err := privateRequest("https://www.binance.com/bapi/futures/v2/private/future/grid/place-grid", "POST", payload, &PlaceGridResponse{})
	return err
}
