package main

import (
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"sort"
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
		sss.strategies = append(sss.strategies, s)
	}
	if sss.highest.runningTime != nil {
		sss.highest.RunningTime = StringPointer(fmt.Sprintf("%s", time.Duration(*sss.highest.runningTime)*time.Second))
	}
	if sss.lowest.runningTime != nil {
		sss.lowest.RunningTime = StringPointer(fmt.Sprintf("%s", time.Duration(*sss.lowest.runningTime)*time.Second))
	}
	for userId, count := range sss.userRankings {
		if count > 1 {
			sss.usersWithMoreThan1Strategy = append(sss.usersWithMoreThan1Strategy, UserPair{Id: userId, Count: count})
		}
	}
	sort.Slice(sss.usersWithMoreThan1Strategy, func(i, j int) bool {
		return sss.usersWithMoreThan1Strategy[i].Count > sss.usersWithMoreThan1Strategy[j].Count
	})
	sss.ids = mapset.NewSetFromMapKeys(sss.strategiesById)
	return sss
}

type UserPair struct {
	Id    int
	Count int
}

type TrackedStrategies struct {
	strategiesById             map[int]*Strategy
	strategiesByUserId         map[int]Strategies
	strategies                 Strategies
	userRankings               map[int]int
	usersWithMoreThan1Strategy []UserPair
	symbolCount                map[string]int
	highest                    StrategyMetrics
	lowest                     StrategyMetrics
	ids                        mapset.Set[int]
}

func (t *TrackedStrategies) findStrategyRanking(id int) int {
	for i, s := range t.strategies {
		if s.StrategyID == id {
			return i
		}
	}
	return -1
}

func (t *TrackedStrategies) String() string {
	return fmt.Sprintf("%d, Symbols: %d, %v, H: %v, L: %v", len(t.strategiesById), len(t.symbolCount), asJson(t.usersWithMoreThan1Strategy), asJson(t.highest), asJson(t.lowest))
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
	Symbol           string `json:"symbol"`
	CopyCount        int    `json:"copyCount"`
	Roi              string `json:"roi"`
	Pnl              string `json:"pnl"`
	RunningTime      int    `json:"runningTime"`
	StrategyID       int    `json:"strategyId"`
	StrategyType     int    `json:"strategyType"`
	Direction        int    `json:"direction"`
	UserID           int    `json:"userId"`
	roi              float64
	lastDayRoiChange float64
	last3HrRoiChange float64
	last2HrRoiChange float64
	lastHrRoiChange  float64
	lastDayRoiPerHr  float64
	roiPerHour       float64
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
	return fmt.Sprintf("%s, Copy: %d, Matched: [%d, %d], PnL: %s, PerH: %.1f%%, PerHLastDay: %.1f%%, A: %s%%, D: %.1f%%, 3H: %.1f%%, 2H: %.1f%%, 1H: %.1f%%, MinInv: %s",
		runTime, s.CopyCount, s.MatchedCount, s.LatestMatchedCount, s.Pnl, s.roiPerHour*100, s.lastDayRoiPerHr*100, s.Roi,
		s.lastDayRoiChange*100, s.last3HrRoiChange*100, s.last2HrRoiChange*100, s.lastHrRoiChange*100, s.MinInvestment)
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
		seq = fmt.Sprintf("%d/%d - ", index, length)
	}

	return fmt.Sprintf("[%s%s, %s, %s, %s] %s: %s%s", seq, symbol, strategyId, direction, userId, action, ss, gg)
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
		SortPair{Sort: SortByRoi, Direction: IntPointer(SHORT), Count: 15},
		SortPair{Sort: SortByRoi, Direction: IntPointer(NEUTRAL), Count: 15},
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
