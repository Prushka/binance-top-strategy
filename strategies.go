package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"net/http"
	"time"
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

type StrategyRoi []*Roi

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
	Code          string                 `json:"code"`
	Message       string                 `json:"message"`
	MessageDetail map[string]interface{} `json:"messageDetail"`
	Success       bool                   `json:"success"`
}

type Strategy struct {
	Rois             StrategyRoi
	Symbol           string  `json:"symbol"`
	CopyCount        int     `json:"copyCount"`
	Roi              string  `json:"roi"`
	Pnl              string  `json:"pnl"`
	RunningTime      int     `json:"runningTime"`
	StrategyID       int     `json:"strategyId"`
	StrategyType     int     `json:"strategyType"`
	Direction        int     `json:"direction"`
	UserID           int     `json:"userId"`
	LastDayRoiChange float64 `json:"lastDayRoiChange"`
	Last3HrRoiChange float64 `json:"last3HrRoiChange"`
	LastHrRoiChange  float64 `json:"lastHrRoiChange"`
	StrategyParams   struct {
		Type           string  `json:"type"`
		LowerLimit     string  `json:"lowerLimit"`
		UpperLimit     string  `json:"upperLimit"`
		GridCount      int     `json:"gridCount"`
		TriggerPrice   any     `json:"triggerPrice"`
		StopLowerLimit *string `json:"stopLowerLimit"`
		StopUpperLimit *string `json:"stopUpperLimit"`
		BaseAsset      any     `json:"baseAsset"`
		QuoteAsset     any     `json:"quoteAsset"`
		Leverage       int     `json:"leverage"`
		TrailingUp     bool    `json:"trailingUp"`
		TrailingDown   bool    `json:"trailingDown"`
	} `json:"strategyParams"`
	TrailingType       string `json:"trailingType"`
	LatestMatchedCount int    `json:"latestMatchedCount"`
	MatchedCount       int    `json:"matchedCount"`
	MinInvestment      string `json:"minInvestment"`
}

type GridDetailResponse struct {
	Data GridDetail `json:"data"`
	BinanceBaseResponse
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

type GridDetail struct {
	StrategyID             int    `json:"strategyId"`
	Symbol                 string `json:"symbol"`
	Direction              string `json:"direction"`
	Isolated               bool   `json:"isolated"`
	GridInitialValue       string `json:"gridInitialValue"`
	InitialLeverage        int    `json:"initialLeverage"`
	GridType               string `json:"gridType"`
	GridCount              int    `json:"gridCount"`
	GridUpperLimit         string `json:"gridUpperLimit"`
	GridLowerLimit         string `json:"gridLowerLimit"`
	InitialUpperLimit      string `json:"initialUpperLimit"`
	InitialLowerLimit      string `json:"initialLowerLimit"`
	TriggerType            any    `json:"triggerType"`
	TriggerLever           any    `json:"triggerLever"`
	TriggerPrice           string `json:"triggerPrice"`
	StopTriggerType        string `json:"stopTriggerType"`
	StopUpperLimit         string `json:"stopUpperLimit"`
	StopLowerLimit         string `json:"stopLowerLimit"`
	Cos                    bool   `json:"cos"`
	Cps                    bool   `json:"cps"`
	BookTime               int64  `json:"bookTime"`
	TriggerTime            int64  `json:"triggerTime"`
	EndTime                int    `json:"endTime"`
	PerGridQty             string `json:"perGridQty"`
	PerGridQuoteQty        string `json:"perGridQuoteQty"`
	TrailingUp             bool   `json:"trailingUp"`
	TrailingDown           bool   `json:"trailingDown"`
	TrailingStopUpperLimit bool   `json:"trailingStopUpperLimit"`
	TrailingStopLowerLimit bool   `json:"trailingStopLowerLimit"`
	TrailingUpLimitPrice   any    `json:"trailingUpLimitPrice"`
	TrailingDownLimitPrice any    `json:"trailingDownLimitPrice"`
	OrderCurrency          string `json:"orderCurrency"`
	OpCode                 int    `json:"opCode"`
	OpCodeMsg              string `json:"opCodeMsg"`
	StrategyStatus         string `json:"strategyStatus"`
	IsSubAccount           bool   `json:"isSubAccount"`
	StrategyUserID         int    `json:"strategyUserId"`
	StrategyFuturesUID     int    `json:"strategyFuturesUid"`
	StrategyAmount         string `json:"strategyAmount"`
	Sharing                bool   `json:"sharing"`
	FundingFee             string `json:"fundingFee"`
	MarginType             string `json:"marginType"`
}

type OpenGridResponse struct {
	Data []struct {
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
	} `json:"data"`
	BinanceBaseResponse
}

const (
	SPOT         = 1
	FUTURE       = 2
	NEUTRAL      = 1
	LONG         = 2
	SHORT        = 3
	NOT_TRAILING = "NOT_TRAILING"
	TRAILING_UP  = "TRAILING_UP"
)

var DirectionMap = map[int]string{
	NEUTRAL: "NEUTRAL",
	LONG:    "LONG",
	SHORT:   "SHORT",
}

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
	if err != nil || !roi.Success {
		return nil, err
	}
	return roi.Data, nil
}

func getTopStrategies(strategyType int, runningTimeMin time.Duration, runningTimeMax time.Duration) (Strategies, error) {
	query := &QueryTopStrategy{
		Page:           1,
		Rows:           TheConfig.StrategiesCount,
		StrategyType:   strategyType,
		RunningTimeMax: int(runningTimeMax.Seconds()),
		RunningTimeMin: int(runningTimeMin.Seconds()),
		Sort:           "roi",
	}
	strategies, err := request(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryTopStrategy",
		query, &StrategiesResponse{})
	if err != nil || !strategies.Success {
		return nil, err
	}
	return strategies.Data, nil
}

func privateRequest[T any](url, method string, payload any, response T) (T, error) {
	p, err := json.Marshal(payload)
	if err != nil {
		return response, err
	}
	var r io.Reader
	if p != nil {
		r = bytes.NewBuffer(p)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		return response, err
	}
	req.Header.Set("Cookie", TheConfig.COOKIE)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	req.Header.Set("Clienttype", "web")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Csrftoken", TheConfig.CSRFToken)
	req.Header.Set("Sec-Ch-Ua", "\\\"Chromium\\\";v=\\\"122\\\", \\\"Not(A:Brand\\\";v=\\\"24\\\", \\\"Google Chrome\\\";v=\\\"122\\\"")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", "\\\"macOS\\\"")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}
	log.Infof("Response: %s", body)
	err = json.Unmarshal(body, response)
	return response, err
}

func getOpenGrids() (*OpenGridResponse, mapset.Set[string], mapset.Set[int], error) {
	existingPairs := mapset.NewSet[string]()
	existingCopiedIds := mapset.NewSet[int]()
	url := "https://www.binance.com/bapi/futures/v2/private/future/grid/query-open-grids"
	res, err := privateRequest(url, "POST", nil, &OpenGridResponse{})
	if err != nil {
		return res, existingPairs, existingCopiedIds, err
	}
	for _, g := range res.Data {
		existingPairs.Add(g.Symbol)
		existingCopiedIds.Add(g.CopiedStrategyID)
	}
	DiscordWebhook(fmt.Sprintf("Open Pairs: %v, Open Ids: %v", existingPairs, existingCopiedIds))
	return res, existingPairs, existingCopiedIds, err
}

func getGridDetail(strategyId string) (GridDetail, error) {
	url := "https://www.binance.com/bapi/futures/v1/private/future/grid/query-grid-detail?strategyId=390204468"
	res, err := privateRequest(url, "GET", nil, &GridDetailResponse{})
	if err != nil {
		return GridDetail{}, err
	}
	log.Infof("Grid: %+v", res)
	return res.Data, err
}

func generateRandomNumberUUID() string {
	const charset = "0123456789"
	b := make([]byte, 19)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
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

func placeGrid(strategy Strategy, initialUSDT float64) error {
	if TheConfig.Paper {
		log.Infof("Paper mode, not placing grid")
		return nil
	}
	if _, ok := DirectionMap[strategy.Direction]; !ok {
		return fmt.Errorf("invalid direction: %d", strategy.Direction)
	}
	leverage := 20
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
		StopTriggerType:        "MARK_PRICE",
		OrderCurrency:          "QUOTE", // not sure
		ClientStrategyID:       "ctrc_web_" + generateRandomNumberUUID(),
		CopiedStrategyID:       strategy.StrategyID,
		TrailingStopLowerLimit: false, // not sure
		TrailingStopUpperLimit: false, // not sure
	}
	if strategy.StrategyParams.StopUpperLimit != nil {
		payload.StopUpperLimit = *strategy.StrategyParams.StopUpperLimit
	}
	if strategy.StrategyParams.StopLowerLimit != nil {
		payload.StopLowerLimit = *strategy.StrategyParams.StopLowerLimit
	}
	PrintAsJson(payload)
	res, err := privateRequest("https://www.binance.com/bapi/futures/v2/private/future/grid/place-grid", "POST", payload, &PlaceGridResponse{})
	PrintAsJson(res)
	return err
}
