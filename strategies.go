package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type QueryStrategyRoi struct {
	RootUserID           int    `json:"rootUserId"`
	StrategyID           int    `json:"strategyId"`
	StreamerStrategyType string `json:"streamerStrategyType"`
}

type QueryTopStrategy struct {
	Page           int    `json:"page"`
	Rows           int    `json:"rows"`
	Direction      any    `json:"direction"`
	StrategyType   int    `json:"strategyType"`
	Symbol         string `json:"symbol"`
	Zone           string `json:"zone"`
	RunningTimeMin int    `json:"runningTimeMin"`
	RunningTimeMax int    `json:"runningTimeMax"`
	Sort           string `json:"sort"`
}

type Strategies []*Strategy

type StrategyRoi []Roi

type Roi struct {
	RootUserID int     `json:"rootUserId"`
	StrategyID int     `json:"strategyId"`
	Roi        float64 `json:"roi"`
	Pnl        float64 `json:"pnl"`
	Time       int64   `json:"time"`
}

type StrategyRoiResponse struct {
	Data StrategyRoi `json:"data"`
	BinanceBaseResponse
}

type StrategiesResponse struct {
	Data  Strategies `json:"data"`
	Total int        `json:"total"`
	BinanceBaseResponse
}

type BinanceBaseResponse struct {
	Code          string  `json:"code"`
	Message       *string `json:"message"`
	MessageDetail *string `json:"messageDetail"`
	Success       bool    `json:"success"`
}

type Strategy struct {
	Rois           []Roi
	Symbol         string `json:"symbol"`
	CopyCount      int    `json:"copyCount"`
	Roi            string `json:"roi"`
	Pnl            string `json:"pnl"`
	RunningTime    int    `json:"runningTime"`
	StrategyID     int    `json:"strategyId"`
	StrategyType   int    `json:"strategyType"`
	Direction      int    `json:"direction"`
	UserID         int    `json:"userId"`
	StrategyParams struct {
		Type           string `json:"type"`
		LowerLimit     string `json:"lowerLimit"`
		UpperLimit     string `json:"upperLimit"`
		GridCount      int    `json:"gridCount"`
		TriggerPrice   any    `json:"triggerPrice"`
		StopLowerLimit any    `json:"stopLowerLimit"`
		StopUpperLimit any    `json:"stopUpperLimit"`
		BaseAsset      any    `json:"baseAsset"`
		QuoteAsset     any    `json:"quoteAsset"`
		Leverage       int    `json:"leverage"`
		TrailingUp     bool   `json:"trailingUp"`
		TrailingDown   bool   `json:"trailingDown"`
	} `json:"strategyParams"`
	TrailingType       string `json:"trailingType"`
	LatestMatchedCount int    `json:"latestMatchedCount"`
	MatchedCount       int    `json:"matchedCount"`
	MinInvestment      string `json:"minInvestment"`
}

const (
	SPOT   = 1
	FUTURE = 2
)

func request[T any](url string, payload any, response T) (T, error) {
	queryJson, _ := json.Marshal(payload)
	resp, err := http.Post(url, "application/json",
		bytes.NewBuffer(queryJson))
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}
	err = json.Unmarshal(body, response)
	if err != nil {
		return response, err
	}
	return response, nil
}

func getStrategyRois(strategyID int, rootUserId int) (StrategyRoi, error) {
	query := &QueryStrategyRoi{
		RootUserID:           rootUserId,
		StrategyID:           strategyID,
		StreamerStrategyType: "UM_GRID",
	}
	roi, err := request(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryRoiChart",
		query, &StrategyRoiResponse{})
	if err != nil {
		return nil, err
	}
	return roi.Data, nil
}

func getTopStrategies(strategyType int, runningTimeMin int, runningTimeMax int) (Strategies, error) {
	query := &QueryTopStrategy{
		Page:           1,
		Rows:           9,
		StrategyType:   strategyType,
		RunningTimeMax: runningTimeMax,
		RunningTimeMin: runningTimeMin,
		Sort:           "roi",
	}
	strategies, err := request(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryTopStrategy",
		query, &StrategiesResponse{})
	if err != nil {
		return nil, err
	}
	return strategies.Data, nil
}
