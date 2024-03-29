package main

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/notional"
	"BinanceTopStrategies/persistence"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/utils"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"math"
	"strconv"
	"time"
)

type StateOnGridOpen struct {
	SDCountRaw          map[string]int
	SDCountFiltered     map[string]int
	SDCountPairSpecific map[string]int
}

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
	request.BinanceBaseResponse
}

type GridTracking struct {
	lowestRoi             float64
	highestRoi            float64
	timeHighestRoi        time.Time
	timeLowestRoi         time.Time
	timeLastChange        time.Time
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
	request.BinanceBaseResponse
}

func newTrackedGrids() *TrackedGrids {
	return &TrackedGrids{
		shorts:          mapset.NewSet[int](),
		longs:           mapset.NewSet[int](),
		neutrals:        mapset.NewSet[int](),
		existingSIDs:    mapset.NewSet[int](),
		existingSymbols: mapset.NewSet[string](),
		gridsByGid:      make(map[int]*Grid),
	}
}

func persistStateOnGridOpen(gid int) {
	if _, ok := statesOnGridOpen[gid]; !ok {
		statesOnGridOpen[gid] = &StateOnGridOpen{SDCountRaw: bundle.Raw.symbolDirectionCount,
			SDCountFiltered:     bundle.FilteredSortedBySD.symbolDirectionCount,
			SDCountPairSpecific: bundle.SDCountPairSpecific}
		err := persistence.Save(statesOnGridOpen, persistence.GridStatesFileName)
		if err != nil {
			discord.Infof("Error saving state on grid open: %v", err)
		}
	}
}

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

func (tracked *TrackedGrids) Remove(id int) {
	g, ok := tracked.gridsByGid[id]
	if !ok {
		return
	}
	tracked.existingSymbols.Remove(g.Symbol)
	tracked.existingSIDs.Remove(g.SID)
	if g.Direction == DirectionMap[LONG] {
		tracked.longs.Remove(g.GID)
	} else if g.Direction == DirectionMap[SHORT] {
		tracked.shorts.Remove(g.GID)
	} else {
		tracked.neutrals.Remove(g.GID)
	}
	tracked.totalGridInitial -= g.initialValue
	tracked.totalGridPnl -= g.totalPnl
	delete(tracked.gridsByGid, g.GID)
}

func (tracked *TrackedGrids) Add(g *Grid, trackContinuous bool) {
	tracked.existingSymbols.Add(g.Symbol)
	tracked.existingSIDs.Add(g.SID)

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
	marketPrice, _ := sdk.GetSessionSymbolPrice(g.Symbol)
	g.initialValue = initial / float64(g.InitialLeverage)
	g.totalPnl = profit + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
	g.lastRoi = g.totalPnl / g.initialValue
	updateTime := time.Now()
	prevG, ok := tracked.gridsByGid[g.GID]
	tracked.totalGridInitial += g.initialValue
	tracked.totalGridPnl += g.totalPnl
	if ok {
		tracked.totalGridInitial -= prevG.initialValue
		tracked.totalGridPnl -= prevG.totalPnl
		tracking := prevG.tracking
		if g.lastRoi < tracking.lowestRoi {
			tracking.timeLowestRoi = updateTime
		}
		if g.lastRoi > tracking.highestRoi {
			tracking.timeHighestRoi = updateTime
		}
		if g.lastRoi != prevG.lastRoi {
			tracking.timeLastChange = updateTime
		}
		tracking.lowestRoi = math.Min(g.lastRoi, tracking.lowestRoi)
		tracking.highestRoi = math.Max(g.lastRoi, tracking.highestRoi)
		if trackContinuous {
			if g.lastRoi > prevG.lastRoi {
				tracking.continuousRoiGrowth += 1
				tracking.continuousRoiLoss = 0
				tracking.continuousRoiNoChange = 0
			} else if g.lastRoi < prevG.lastRoi {
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
			timeLastChange: updateTime,
		}
		persistStateOnGridOpen(g.GID)
	}
	tracked.gridsByGid[g.GID] = g
}

type TrackedGrids struct {
	totalGridInitial float64
	totalGridPnl     float64
	shorts           mapset.Set[int]
	longs            mapset.Set[int]
	neutrals         mapset.Set[int]
	existingSIDs     mapset.Set[int]
	existingSymbols  mapset.Set[string]
	gridsByGid       map[int]*Grid
}

func (tracked *TrackedGrids) findGridIdsByStrategyId(ids ...int) mapset.Set[int] {
	gridIds := mapset.NewSet[int]()
	idsSet := mapset.NewSet[int](ids...)
	for _, g := range tracked.gridsByGid {
		if idsSet.Contains(g.SID) {
			gridIds.Add(g.GID)
		}
	}
	return gridIds
}

func getOpenGrids() (*OpenGridResponse, error) {
	url := "https://www.binance.com/bapi/futures/v2/private/future/grid/query-open-grids"
	res, _, err := request.PrivateRequest(url, "POST", nil, &OpenGridResponse{})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func getGridSymbols() (mapset.Set[string], error) {
	res, err := getOpenGrids()
	if err != nil {
		return nil, err
	}
	symbols := mapset.NewSet[string]()
	for _, grid := range res.Grids {
		symbols.Add(grid.Symbol)
	}
	return symbols, nil
}

func updateOpenGrids(trackContinuous bool) error {
	res, err := getOpenGrids()
	if err != nil {
		return err
	}
	currentIds := mapset.NewSet[int]()
	for _, grid := range res.Grids {
		gGrids.Add(grid, trackContinuous)
		currentIds.Add(grid.GID)
	}
	for _, g := range gGrids.gridsByGid {
		if !currentIds.Contains(g.GID) {
			gGrids.Remove(g.GID)
			discord.Info(display(nil, g,
				fmt.Sprintf("**Gone - Block for %d Minutes**", config.TheConfig.TradingBlockMinutesAfterCancel),
				0, 0), discord.ActionWebhook, discord.DefaultWebhook)
			tradingBlock = time.Now().Add(time.Duration(config.TheConfig.TradingBlockMinutesAfterCancel) * time.Minute)
		}
	}
	discord.Infof("Open Pairs: %v, Open Ids: %v, Initial: %f, TotalPnL: %f, C: %f, L/S/N: %d/%d/%d",
		gGrids.existingSymbols, gGrids.existingSIDs, gGrids.totalGridInitial, gGrids.totalGridPnl, gGrids.totalGridPnl+gGrids.totalGridInitial,
		gGrids.longs.Cardinality(), gGrids.shorts.Cardinality(), gGrids.neutrals.Cardinality())
	return nil
}

func (grid *Grid) String() string {
	tracking := grid.tracking
	extendedProfit := ""
	extendedProfit = fmt.Sprintf(" [%.2f%% (%s), %.2f%% (%s)][+%d, -%d, %d (%s)]",
		tracking.lowestRoi*100,
		time.Since(tracking.timeLowestRoi).Round(time.Second),
		tracking.highestRoi*100,
		time.Since(tracking.timeHighestRoi).Round(time.Second),
		tracking.continuousRoiGrowth, tracking.continuousRoiLoss, tracking.continuousRoiNoChange,
		time.Since(tracking.timeLastChange).Round(time.Second),
	)
	formatSDRatio := func(setType string) string {
		currentSD, sdWhenOpen, ratio := gridSDCount(grid.GID, grid.Symbol, grid.Direction, setType)
		return fmt.Sprintf("%s: %d/%d/%.1f%%", setType, currentSD, sdWhenOpen, ratio*100)
	}
	realized, _ := strconv.ParseFloat(grid.GridProfit, 64)
	return fmt.Sprintf("*%d*, Realized: %.2f, Total: %.2f, =%.2f%%%s, %s, %s, %s",
		grid.GID,
		realized, grid.totalPnl, grid.lastRoi*100, extendedProfit,
		formatSDRatio(SDRaw), formatSDRatio(SDFiltered), formatSDRatio(SDPairSpecific))
}

func closeGrid(strategyId int) error {
	if config.TheConfig.Paper {
		log.Infof("Paper mode, not closing grid")
		return nil
	}
	url := "https://www.binance.com/bapi/futures/v1/private/future/grid/close-grid"
	payload := map[string]interface{}{
		"strategyId": strategyId,
	}
	_, _, err := request.PrivateRequest(url, "POST", payload, &request.BinanceBaseResponse{})
	return err
}

func placeGrid(strategy Strategy, initialUSDT float64) error {
	if _, ok := DirectionMap[strategy.Direction]; !ok {
		return fmt.Errorf("invalid direction: %d", strategy.Direction)
	}
	leverage := config.TheConfig.MaxLeverage
	if strategy.StrategyParams.Leverage < leverage {
		leverage = strategy.StrategyParams.Leverage
	}
	leverage = notional.GetLeverage(strategy.Symbol, initialUSDT, leverage)
	payload := &PlaceGridRequest{
		Symbol:                 strategy.Symbol,
		Direction:              DirectionMap[strategy.Direction],
		Leverage:               leverage,
		MarginType:             config.TheConfig.MarginType,
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
		ClientStrategyID:       "ctrc_web_" + utils.GenerateRandomNumberUUID(),
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
	discord.Info(discord.Json(string(s)), discord.OrderWebhook)
	if config.TheConfig.Paper {
		log.Infof("Paper mode, not placing grid")
		return nil
	}
	_, _, err := request.PrivateRequest("https://www.binance.com/bapi/futures/v2/private/future/grid/place-grid", "POST", payload, &PlaceGridResponse{})
	return err
}
