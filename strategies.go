package main

import (
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"strconv"
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
	Direction      *int   `json:"direction"`
	StrategyType   int    `json:"strategyType"`
	Symbol         string `json:"symbol"`
	Zone           string `json:"zone"`
	RunningTimeMin int    `json:"runningTimeMin"`
	RunningTimeMax int    `json:"runningTimeMax"`
	Sort           string `json:"sort"`
}

type Strategies []*Strategy

func (by Strategies) toTrackedStrategies() *TrackedStrategies {
	sss := &TrackedStrategies{
		strategiesById:     make(map[int]*Strategy),
		strategiesByUserId: make(map[int]Strategies),
		userRankings:       make(map[int]int),
		symbolCount:        make(map[string]int),
	}
	for _, s := range by {
		_, ok := sss.strategiesById[s.StrategyID]
		if ok {
			continue
		}
		sss.strategiesById[s.StrategyID] = s
		if _, ok := sss.strategiesByUserId[s.UserID]; !ok {
			sss.strategiesByUserId[s.UserID] = make(Strategies, 0)
		}
		sss.strategiesByUserId[s.UserID] = append(sss.strategiesByUserId[s.UserID], s)
		sss.userRankings[s.UserID] += 1
		sss.symbolCount[s.Symbol] += 1
		roi, _ := strconv.ParseFloat(s.Roi, 64)
		pnl, _ := strconv.ParseFloat(s.Pnl, 64)
		if sss.highest.CopyCount == nil || s.CopyCount > *sss.highest.CopyCount {
			sss.highest.CopyCount = &s.CopyCount
		}
		if sss.lowest.CopyCount == nil || s.CopyCount < *sss.lowest.CopyCount {
			sss.lowest.CopyCount = &s.CopyCount
		}
		if sss.highest.Roi == nil || roi > *sss.highest.Roi {
			sss.highest.Roi = &roi
		}
		if sss.lowest.Roi == nil || roi < *sss.lowest.Roi {
			sss.lowest.Roi = &roi
		}
		if sss.highest.Pnl == nil || pnl > *sss.highest.Pnl {
			sss.highest.Pnl = &pnl
		}
		if sss.lowest.Pnl == nil || pnl < *sss.lowest.Pnl {
			sss.lowest.Pnl = &pnl
		}
		if sss.highest.runningTime == nil || s.RunningTime > *sss.highest.runningTime {
			sss.highest.runningTime = &s.RunningTime
		}
		if sss.lowest.runningTime == nil || s.RunningTime < *sss.lowest.runningTime {
			sss.lowest.runningTime = &s.RunningTime
		}
		if sss.highest.MatchedCount == nil || s.MatchedCount > *sss.highest.MatchedCount {
			sss.highest.MatchedCount = &s.MatchedCount
		}
		if sss.lowest.MatchedCount == nil || s.MatchedCount < *sss.lowest.MatchedCount {
			sss.lowest.MatchedCount = &s.MatchedCount
		}
		if sss.highest.LatestMatchedCount == nil || s.LatestMatchedCount > *sss.highest.LatestMatchedCount {
			sss.highest.LatestMatchedCount = &s.LatestMatchedCount
		}
		if sss.lowest.LatestMatchedCount == nil || s.LatestMatchedCount < *sss.lowest.LatestMatchedCount {
			sss.lowest.LatestMatchedCount = &s.LatestMatchedCount
		}
		globalStrategies[s.StrategyID] = s
	}
	if sss.highest.runningTime != nil {
		sss.highest.RunningTime = StringPointer(fmt.Sprintf("%s", time.Duration(*sss.highest.runningTime)*time.Second))
	}
	if sss.lowest.runningTime != nil {
		sss.lowest.RunningTime = StringPointer(fmt.Sprintf("%s", time.Duration(*sss.lowest.runningTime)*time.Second))
	}
	for _, s := range sss.strategiesById {
		sss.strategies = append(sss.strategies, s)
	}
	sss.ids = mapset.NewSetFromMapKeys(sss.strategiesById)
	return sss
}

type TrackedStrategies struct {
	strategiesById     map[int]*Strategy
	strategiesByUserId map[int]Strategies
	strategies         Strategies
	userRankings       map[int]int
	symbolCount        map[string]int
	highest            StrategyMetrics
	lowest             StrategyMetrics
	ids                mapset.Set[int]
}

func (t *TrackedStrategies) String() string {
	return fmt.Sprintf("Found: %d, H: %v, L: %v", len(t.strategiesById), asJson(t.highest), asJson(t.lowest))
}

func (t *TrackedStrategies) exists(id int) bool {
	return t.ids.Contains(id)
}

type StrategyMetrics struct {
	CopyCount          *int     `json:"copyCount"`
	Roi                *float64 `json:"roi"`
	Pnl                *float64 `json:"pnl"`
	runningTime        *int
	RunningTime        *string `json:"runningTime"`
	LatestMatchedCount *int    `json:"latestMatchedCount"`
	MatchedCount       *int    `json:"matchedCount"`
}

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
	Last2HrRoiChange float64 `json:"last2HrRoiChange"`
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

func (s Strategy) String() string {
	runTime := time.Duration(s.RunningTime) * time.Second
	return fmt.Sprintf("%s, Copy: %d, Matched: [%d, %d], A: %s%%, D: %.1f%%, 3H: %.1f%%, 2H: %.1f%%, 1H: %.1f%%, MinInv: %s",
		runTime, s.CopyCount, s.MatchedCount, s.LatestMatchedCount, s.Roi,
		s.LastDayRoiChange*100, s.Last3HrRoiChange*100, s.Last2HrRoiChange*100, s.LastHrRoiChange*100, s.MinInvestment)
}

func display(s *Strategy, grid *Grid, action string, index int, length int) string {
	if grid == nil && s == nil {
		return "Strategy and Grid are both nil"
	}
	ss := ""
	gg := ""
	seq := ""
	direction := ""
	userId := ""
	strategyId := ""
	symbol := ""
	if s == nil {
		direction = grid.Direction
		userId = fmt.Sprintf("%d", grid.RootUserID)
		symbol = grid.Symbol
		strategyId = fmt.Sprintf("%d", grid.CopiedStrategyID)
	} else if grid == nil || DirectionMap[s.Direction] == grid.Direction {
		direction = DirectionMap[s.Direction]
		userId = fmt.Sprintf("%d", s.UserID)
		symbol = s.Symbol
		strategyId = fmt.Sprintf("%d", s.StrategyID)
	} else {
		direction = fmt.Sprintf("S: %s, G: %s", DirectionMap[s.Direction], grid.Direction)
		userId = fmt.Sprintf("S: %d, G: %d", s.UserID, grid.RootUserID)
		symbol = fmt.Sprintf("S: %s, G: %s", s.Symbol, grid.Symbol)
		strategyId = fmt.Sprintf("S: %d, G: %d", s.StrategyID, grid.CopiedStrategyID)
	}
	if s != nil {
		ss = s.String()
	}
	if grid != nil {
		gg = ", " + grid.String()
	}
	if length != 0 {
		seq = fmt.Sprintf("[%d/%d]", index, length)
	}

	return fmt.Sprintf("[%s, %s, %s, %s] %s: %s %s%s", symbol, strategyId, direction, userId, seq, action, ss, gg)
}

const (
	SPOT         = 1
	FUTURE       = 2
	NEUTRAL      = 1
	LONG         = 2
	SHORT        = 3
	NOT_TRAILING = "NOT_TRAILING"
	TRAILING_UP  = "TRAILING_UP"

	SortByRoi       = "roi"
	SortByPnl       = "pnl"
	SortByCopyCount = "copyCount"
	SortByMatched   = "latestMatchedCount"
)

var DirectionMap = map[int]string{
	NEUTRAL: "NEUTRAL",
	LONG:    "LONG",
	SHORT:   "SHORT",
}

func getStrategyRois(strategyID int, rootUserId int) (StrategyRoi, error) {
	query := &QueryStrategyRoi{
		RootUserID:           rootUserId,
		StrategyID:           strategyID,
		StreamerStrategyType: "UM_GRID",
	}
	roi, _, err := request(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryRoiChart",
		query, &StrategyRoiResponse{})
	if err != nil || !roi.Success {
		return nil, err
	}
	return roi.Data, nil
}

type SortPair struct {
	Sort      string
	Direction *int
	Count     int
}

func mergeStrategies(strategyType int, runningTimeMin time.Duration, runningTimeMax time.Duration, sps ...SortPair) (*TrackedStrategies, error) {
	sss := make(Strategies, 0)
	for _, sp := range sps {
		if sp.Count == 0 {
			sp.Count = TheConfig.StrategiesCount
		}
		by, err := _getTopStrategies(sp.Sort, sp.Direction, strategyType, runningTimeMin, runningTimeMax, sp.Count)
		if err != nil {
			return nil, err
		}
		sss = append(sss, by...)
		time.Sleep(1 * time.Second)
	}
	return sss.toTrackedStrategies(), nil
}

func getTopStrategies(strategyType int, runningTimeMin time.Duration, runningTimeMax time.Duration) (*TrackedStrategies, error) {
	merged, err := mergeStrategies(strategyType, runningTimeMin, runningTimeMax,
		SortPair{Sort: SortByRoi},
		//SortPair{Sort: SortByRoi, Direction: IntPointer(SHORT)},
		//SortPair{Sort: SortByRoi, Direction: IntPointer(LONG)},
		//SortPair{Sort: SortByMatched},
		//SortPair{Sort: SortByPnl},
		//SortPair{Sort: SortByCopyCount},
	)
	if err != nil {
		return nil, err
	}
	return merged, nil
}

func _getTopStrategies(sort string, direction *int, strategyType int, runningTimeMin time.Duration, runningTimeMax time.Duration, strategyCount int) (Strategies, error) {
	query := &QueryTopStrategy{
		Page:           1,
		Rows:           strategyCount,
		StrategyType:   strategyType,
		RunningTimeMax: int(runningTimeMax.Seconds()),
		RunningTimeMin: int(runningTimeMin.Seconds()),
		Sort:           sort,
		Direction:      direction,
	}
	strategies, res, err := request(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryTopStrategy",
		query, &StrategiesResponse{})
	if err != nil || !strategies.Success {
		return nil, err
	}
	generic := map[string]interface{}{}
	err = json.Unmarshal(res, &generic)
	if err != nil {
		return nil, err
	}
	if len(generic) != 6 {
		DiscordWebhook(fmt.Sprintf("Error, strategies root response has length %d: %+v", len(generic),
			generic))
	}
	for _, v := range generic["data"].([]interface{}) {
		if len(v.(map[string]interface{})) != 14 {
			DiscordWebhook(fmt.Sprintf("Error, strategy response has length %d: %+v", len(v.(map[string]interface{})),
				v))
		}
	}
	return strategies.Data, nil
}

func getOpenGrids() (*OpenGridResponse, error) {
	url := "https://www.binance.com/bapi/futures/v2/private/future/grid/query-open-grids"
	res, err := privateRequest(url, "POST", nil, &OpenGridResponse{})
	if err != nil {
		return res, err
	}
	res.existingPairs = mapset.NewSet[string]()
	res.existingIds = mapset.NewSet[int]()
	for _, g := range res.Data {
		res.existingPairs.Add(g.Symbol)
		res.existingIds.Add(g.CopiedStrategyID)
		initial, _ := strconv.ParseFloat(g.GridInitialValue, 64)
		profit, _ := strconv.ParseFloat(g.GridProfit, 64)
		fundingFee, _ := strconv.ParseFloat(g.FundingFee, 64)
		position, _ := strconv.ParseFloat(g.GridPosition, 64)
		entryPrice, _ := strconv.ParseFloat(g.GridEntryPrice, 64)
		marketPrice, _ := fetchMarketPrice(g.Symbol)
		g.initialValue = initial / float64(g.InitialLeverage)
		g.totalPnl = profit + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
		g.profit = g.totalPnl / g.initialValue
		res.totalGridInitial += g.initialValue
		res.totalGridPnl += g.totalPnl
		if g.Direction == DirectionMap[LONG] {
			res.totalLongs += 1
		} else if g.Direction == DirectionMap[SHORT] {
			res.totalShorts += 1
		} else {
			res.totalNeutrals += 1
		}
	}
	DiscordWebhook(fmt.Sprintf("Open Pairs: %v, Open Ids: %v, Initial: %f, TotalPnL: %f, C: %f, L/S/N: %d/%d/%d",
		res.existingPairs, res.existingIds, res.totalGridInitial, res.totalGridPnl, res.totalGridPnl+res.totalGridInitial,
		res.totalLongs, res.totalShorts, res.totalNeutrals))
	return res, err
}
