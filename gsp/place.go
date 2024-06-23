package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sql"
	"BinanceTopStrategies/utils"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
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

func PlaceGrid(strategy Strategy, input float64, leverage int, useCopy bool) error {
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
		GridLowerLimit:         strategy.StrategyParams.LowerLimitStr,
		GridUpperLimit:         strategy.StrategyParams.UpperLimitStr,
		GridInitialValue:       fmt.Sprintf("%.2f", input*float64(leverage)),
		Cos:                    true,
		Cps:                    true,
		TrailingUp:             strategy.StrategyParams.TrailingUp,
		TrailingDown:           strategy.StrategyParams.TrailingDown,
		OrderCurrency:          "BASE",
		ClientStrategyID:       "ctrc_web_" + utils.GenerateRandomNumberUUID(),
		TrailingStopLowerLimit: false, // !!t[E.w2.stopLowerLimit]
		TrailingStopUpperLimit: false, // !1 in js
	}
	if useCopy {
		payload.CopiedStrategyID = strategy.SID
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
	discord.Orderf(discord.Json(string(s)))
	if config.TheConfig.Paper {
		log.Infof("Paper mode, not placing grid")
		return nil
	}
	resp, _, err := request.PrivateRequest("https://www.binance.com/bapi/futures/v2/private/future/grid/place-grid", "POST", payload, &placeGridResponse{})
	if err == nil {
		err = sql.SimpleTransaction(func(tx pgx.Tx) error {
			_, err = tx.Exec(context.Background(), `INSERT INTO bts.grid_strategy (strategy_id, grid_id) VALUES ($1, $2)`,
				strategy.SID, resp.Data.StrategyID)
			return err
		})
		if err != nil {
			log.Errorf("Error inserting grid_strategy: %v", err)
		}
	}
	return err
}
