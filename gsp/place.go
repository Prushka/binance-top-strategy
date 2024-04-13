package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/notional"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/utils"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
)

type placeGridRequest struct {
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

type placeGridResponse struct {
	Data struct {
		StrategyID       int    `json:"strategyId"`
		ClientStrategyID string `json:"clientStrategyId"`
		StrategyType     string `json:"strategyType"`
		StrategyStatus   string `json:"strategyStatus"`
		UpdateTime       int64  `json:"updateTime"`
	} `json:"data"`
	request.BinanceBaseResponse
}

func (s Strategy) MaxLeverage(initialUSDT float64) int {
	leverage := config.TheConfig.MaxLeverage
	if s.StrategyParams.Leverage < leverage {
		leverage = s.StrategyParams.Leverage
	}
	leverage = notional.GetLeverage(s.Symbol, initialUSDT, leverage)
	return leverage
}

func PlaceGrid(strategy Strategy, initialUSDT float64, leverage int) error {
	if _, ok := DirectionMap[strategy.Direction]; !ok {
		return fmt.Errorf("invalid direction: %d", strategy.Direction)
	}
	payload := &placeGridRequest{
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
	if strategy.Direction == SHORT {
		payload.TrailingDown = true
	}
	s, _ := json.Marshal(payload)
	discord.Orderf(discord.Json(string(s)))
	if config.TheConfig.Paper {
		log.Infof("Paper mode, not placing grid")
		return nil
	}
	_, _, err := request.PrivateRequest("https://www.binance.com/bapi/futures/v2/private/future/grid/place-grid", "POST", payload, &placeGridResponse{})
	return err
}
