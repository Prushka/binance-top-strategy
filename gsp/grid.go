package gsp

import (
	"BinanceTopStrategies/blacklist"
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

func Init() {
	err := persistence.Load(&statesOnGridOpen, persistence.GridStatesFileName)
	if err != nil {
		log.Fatalf("Error loading state on grid open: %v", err)
	}
}

var statesOnGridOpen = make(map[int]*StateOnGridOpen)

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
	LowestRoi             float64
	HighestRoi            float64
	TimeHighestRoi        time.Time
	TimeLowestRoi         time.Time
	TimeLastChange        time.Time
	ContinuousRoiGrowth   int
	ContinuousRoiLoss     int
	ContinuousRoiNoChange int
}

type Grid struct {
	totalPnl               float64
	initialValue           float64
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

type OpenGridResponse struct {
	Grids []*Grid `json:"data"`
	request.BinanceBaseResponse
}

func newTrackedGrids() *TrackedGrids {
	return &TrackedGrids{
		Shorts:          mapset.NewSet[int](),
		Longs:           mapset.NewSet[int](),
		Neutrals:        mapset.NewSet[int](),
		ExistingSIDs:    mapset.NewSet[int](),
		ExistingSymbols: mapset.NewSet[string](),
		GridsByGid:      make(map[int]*Grid),
	}
}

func persistStateOnGridOpen(gid int) {
	if _, ok := statesOnGridOpen[gid]; !ok {
		statesOnGridOpen[gid] = &StateOnGridOpen{SDCountRaw: Bundle.Raw.SymbolDirectionCount,
			SDCountFiltered:     Bundle.FilteredSortedBySD.SymbolDirectionCount,
			SDCountPairSpecific: Bundle.SDCountPairSpecific}
		err := persistence.Save(statesOnGridOpen, persistence.GridStatesFileName)
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
		sdCountWhenOpen = statesOnGridOpen[gid].SDCountRaw[sd]
	case SDFiltered:
		currentSDCount = Bundle.FilteredSortedBySD.SymbolDirectionCount[sd]
		sdCountWhenOpen = statesOnGridOpen[gid].SDCountFiltered[sd]
	case SDPairSpecific:
		currentSDCount = Bundle.SDCountPairSpecific[sd]
		sdCountWhenOpen = statesOnGridOpen[gid].SDCountPairSpecific[sd]
	}
	ratio := float64(currentSDCount) / float64(sdCountWhenOpen)
	return currentSDCount, sdCountWhenOpen, ratio
}

func (tracked *TrackedGrids) Remove(id int) {
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
	tracked.TotalGridInitial -= g.initialValue
	tracked.TotalGridPnl -= g.totalPnl
	delete(tracked.GridsByGid, g.GID)
}

func (tracked *TrackedGrids) Add(g *Grid, trackContinuous bool) {
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
	g.initialValue = initial / float64(g.InitialLeverage)
	g.totalPnl = profit + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
	g.LastRoi = g.totalPnl / g.initialValue
	updateTime := time.Now()
	prevG, ok := tracked.GridsByGid[g.GID]
	tracked.TotalGridInitial += g.initialValue
	tracked.TotalGridPnl += g.totalPnl
	if ok {
		tracked.TotalGridInitial -= prevG.initialValue
		tracked.TotalGridPnl -= prevG.totalPnl
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
		persistStateOnGridOpen(g.GID)
	}
	tracked.GridsByGid[g.GID] = g
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

func (tracked *TrackedGrids) findGridIdsByStrategyId(ids ...int) mapset.Set[int] {
	gridIds := mapset.NewSet[int]()
	idsSet := mapset.NewSet[int](ids...)
	for _, g := range tracked.GridsByGid {
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

func UpdateOpenGrids(trackContinuous bool) error {
	res, err := getOpenGrids()
	if err != nil {
		return err
	}
	currentIds := mapset.NewSet[int]()
	for _, grid := range res.Grids {
		GlobalGrids.Add(grid, trackContinuous)
		currentIds.Add(grid.GID)
	}
	for _, g := range GlobalGrids.GridsByGid {
		if !currentIds.Contains(g.GID) {
			GlobalGrids.Remove(g.GID)
			discord.Info(Display(nil, g,
				fmt.Sprintf("**Gone - Block for %d Minutes**", config.TheConfig.TradingBlockMinutesAfterCancel),
				0, 0), discord.ActionWebhook, discord.DefaultWebhook)
			blacklist.BlockTrading(time.Duration(config.TheConfig.TradingBlockMinutesAfterCancel)*time.Minute, "Grid Gone")
		}
	}
	discord.Infof("Open Pairs: %v, Open Ids: %v, Initial: %f, TotalPnL: %f, C: %f, L/S/N: %d/%d/%d",
		GlobalGrids.ExistingSymbols, GlobalGrids.ExistingSIDs, GlobalGrids.TotalGridInitial, GlobalGrids.TotalGridPnl, GlobalGrids.TotalGridPnl+GlobalGrids.TotalGridInitial,
		GlobalGrids.Longs.Cardinality(), GlobalGrids.Shorts.Cardinality(), GlobalGrids.Neutrals.Cardinality())
	return nil
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
		realized, grid.totalPnl, grid.LastRoi*100, extendedProfit,
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

func PlaceGrid(strategy Strategy, initialUSDT float64) error {
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
