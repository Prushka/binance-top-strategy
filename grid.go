package main

import (
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
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

type Grid struct {
	totalPnl               float64
	initialValue           float64
	profit                 float64
	StrategyID             int    `json:"strategyId"`
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
	CopiedStrategyID       int    `json:"copiedStrategyId"`
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
	totalGridInitial float64
	totalGridPnl     float64
	totalShorts      int
	totalLongs       int
	totalNeutrals    int
	existingIds      mapset.Set[int]
	existingPairs    mapset.Set[string]
	Data             []*Grid `json:"data"`
	BinanceBaseResponse
}

type TrackedGrid struct {
	LowestRoi             float64
	HighestRoi            float64
	LastRoi               float64
	ContinuousRoiGrowth   int
	ContinuousRoiLoss     int
	ContinuousRoiNoChange int
	grid                  *Grid
}

func (grid *Grid) String() string {
	extendedProfit := ""
	tracked, ok := globalGrids[grid.CopiedStrategyID]
	if ok {
		extendedProfit = fmt.Sprintf(" [%.2f%%, %.2f%%][+%d, -%d, %d]",
			tracked.LowestRoi*100, tracked.HighestRoi*100, tracked.ContinuousRoiGrowth, tracked.ContinuousRoiLoss, tracked.ContinuousRoiNoChange)
	}
	return fmt.Sprintf("In: %.2f, RealizedPnL: %s, TotalPnL: %f, Profit: %f%%%s",
		grid.initialValue,
		grid.GridProfit, grid.totalPnl, grid.profit*100, extendedProfit)
}

func (grid *Grid) track() *TrackedGrid {
	_, ok := globalGrids[grid.CopiedStrategyID]
	if !ok {
		globalGrids[grid.CopiedStrategyID] = &TrackedGrid{
			LowestRoi:  grid.profit,
			HighestRoi: grid.profit,
			LastRoi:    grid.profit,
		}
	}
	tracked := globalGrids[grid.CopiedStrategyID]
	tracked.grid = grid

	if grid.profit < tracked.LowestRoi {
		tracked.LowestRoi = grid.profit
	}
	if grid.profit > tracked.HighestRoi {
		tracked.HighestRoi = grid.profit
	}
	if ok {
		if grid.profit > tracked.LastRoi {
			tracked.ContinuousRoiGrowth += 1
			tracked.ContinuousRoiLoss = 0
			tracked.ContinuousRoiNoChange = 0
		} else if grid.profit < tracked.LastRoi {
			tracked.ContinuousRoiLoss += 1
			tracked.ContinuousRoiGrowth = 0
			tracked.ContinuousRoiNoChange = 0
		} else {
			tracked.ContinuousRoiNoChange += 1
			tracked.ContinuousRoiGrowth = 0
			tracked.ContinuousRoiLoss = 0
		}
	}
	tracked.LastRoi = grid.profit
	return tracked
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
	_, err := privateRequest(url, "POST", payload, &BinanceBaseResponse{})
	return err
}

func closeGridConv(copiedId int, openGrids *OpenGridResponse) error {
	gridToClose := copiedId
	gridCurrId := 0
	for _, g := range openGrids.Data {
		if g.CopiedStrategyID == gridToClose {
			gridCurrId = g.StrategyID
			break
		}
	}
	if gridCurrId != 0 {
		err := closeGrid(gridCurrId)
		if err != nil {
			return err
		}
	}
	return nil
}

func placeGrid(strategy Strategy, initialUSDT float64) error {
	if TheConfig.Paper {
		log.Infof("Paper mode, not placing grid")
		return nil
	}
	if _, ok := DirectionMap[strategy.Direction]; !ok {
		return fmt.Errorf("invalid direction: %d", strategy.Direction)
	}
	payload := &PlaceGridRequest{
		Symbol:                 strategy.Symbol,
		Direction:              DirectionMap[strategy.Direction],
		Leverage:               TheConfig.Leverage,
		MarginType:             "CROSSED",
		GridType:               strategy.StrategyParams.Type,
		GridCount:              strategy.StrategyParams.GridCount,
		GridLowerLimit:         strategy.StrategyParams.LowerLimit,
		GridUpperLimit:         strategy.StrategyParams.UpperLimit,
		GridInitialValue:       fmt.Sprintf("%.2f", initialUSDT*float64(TheConfig.Leverage)),
		Cos:                    true,
		Cps:                    true,
		TrailingUp:             strategy.StrategyParams.TrailingUp,
		TrailingDown:           strategy.StrategyParams.TrailingDown,
		OrderCurrency:          "BASE",
		ClientStrategyID:       "ctrc_web_" + generateRandomNumberUUID(),
		CopiedStrategyID:       strategy.StrategyID,
		TrailingStopLowerLimit: false, // !!t[E.w2.stopLowerLimit]
		TrailingStopUpperLimit: false, // !1 in js
	}
	if payload.TrailingUp || payload.TrailingDown {
		payload.OrderCurrency = "QUOTE"
	}
	if strategy.StrategyParams.StopUpperLimit != nil {
		payload.StopUpperLimit = *strategy.StrategyParams.StopUpperLimit
		payload.StopTriggerType = "MARK_PRICE"
	}
	if strategy.StrategyParams.StopLowerLimit != nil {
		payload.StopLowerLimit = *strategy.StrategyParams.StopLowerLimit
		payload.StopTriggerType = "MARK_PRICE"
		payload.TrailingStopLowerLimit = true
	}
	s, _ := json.Marshal(payload)
	DiscordWebhookS(string(s), OrderWebhook)
	res, err := privateRequest("https://www.binance.com/bapi/futures/v2/private/future/grid/place-grid", "POST", payload, &PlaceGridResponse{})
	if !res.Success {
		return fmt.Errorf(res.Message)
	}
	return err
}
