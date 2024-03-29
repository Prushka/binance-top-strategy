package main

import (
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"slices"
	"sort"
	"strconv"
	"strings"
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
		strategiesById:       make(map[int]*Strategy),
		strategiesByUserId:   make(map[int]Strategies),
		userRankings:         make(map[int]int),
		symbolCount:          make(map[string]int),
		symbolDirectionCount: make(map[string]int),
		longs:                mapset.NewSet[int](),
		shorts:               mapset.NewSet[int](),
		neutrals:             mapset.NewSet[int](),
	}
	for _, s := range by {
		_, ok := sss.strategiesById[s.SID]
		if ok {
			continue
		}
		sss.strategiesById[s.SID] = s
		if _, ok := sss.strategiesByUserId[s.UserID]; !ok {
			sss.strategiesByUserId[s.UserID] = make(Strategies, 0)
		}
		sss.strategiesByUserId[s.UserID] = append(sss.strategiesByUserId[s.UserID], s)
		sss.userRankings[s.UserID] += 1
		sss.symbolCount[s.Symbol] += 1
		sss.symbolDirectionCount[s.Symbol+DirectionMap[s.Direction]] += 1
		if s.Direction == LONG {
			sss.longs.Add(s.SID)
		} else if s.Direction == SHORT {
			sss.shorts.Add(s.SID)
		} else {
			sss.neutrals.Add(s.SID)
		}
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
		globalStrategies[s.SID] = s
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
	symbolDirectionCount       map[string]int
	longs                      mapset.Set[int]
	shorts                     mapset.Set[int]
	neutrals                   mapset.Set[int]
	highest                    StrategyMetrics
	lowest                     StrategyMetrics
	ids                        mapset.Set[int]
}

func (t *TrackedStrategies) findStrategyRanking(s Strategy) int {
	symbolDirection := mapset.NewSet[string]()
	counter := 0
	sd := s.Symbol + DirectionMap[s.Direction]
	for _, s := range t.strategies {
		sdd := s.Symbol + DirectionMap[s.Direction]
		if sdd == sd {
			return counter
		}
		if symbolDirection.Contains(sdd) {
			continue
		}
		symbolDirection.Add(sdd)
		counter++
	}
	return -1
}

func (t *TrackedStrategies) String() string {
	return fmt.Sprintf("%d, Symbols: %d, L/S/N: %d/%d/%d, SymbolDirections: %v, H: %v, L: %v", len(t.strategiesById), len(t.symbolCount),
		t.longs.Cardinality(), t.shorts.Cardinality(), t.neutrals.Cardinality(),
		asJson(t.symbolDirectionCount), asJson(t.highest), asJson(t.lowest))
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
	SID              int    `json:"strategyId"`
	StrategyType     int    `json:"strategyType"`
	Direction        int    `json:"direction"`
	UserID           int    `json:"userId"`
	roi              float64
	lastDayRoiChange float64
	last3HrRoiChange float64
	last2HrRoiChange float64
	lastHrRoiChange  float64
	lastDayRoiPerHr  float64
	last12HrRoiPerHr float64
	last6HrRoiPerHr  float64
	lastNHrNoDip     bool
	roiPerHour       float64
	priceDifference  float64
	StrategyParams   struct {
		Type           string  `json:"type"`
		LowerLimit     string  `json:"lowerLimit"`
		UpperLimit     string  `json:"upperLimit"`
		GridCount      int     `json:"gridCount"`
		TriggerPrice   *string `json:"triggerPrice"`
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

func (r StrategyRoi) lastNRecords(n int) string {
	n += 1
	if len(r) < n {
		n = len(r)
	}
	var ss []string
	for i := 0; i < n; i++ {
		ss = append(ss, fmt.Sprintf("%.2f%%", r[i].Roi*100))
	}
	slices.Reverse(ss)
	return strings.Join(ss, ", ")
}

func (s Strategy) String() string {
	pnl, _ := strconv.ParseFloat(s.Pnl, 64)
	ranking := ""
	if bundle != nil {
		ranking = fmt.Sprintf(", Rank: Raw: %d, FilterdSD: %d", bundle.Raw.findStrategyRanking(s), bundle.FilteredSortedBySD.findStrategyRanking(s))
	}
	return fmt.Sprintf("Cpy: %d, Mch: [%d, %d], PnL: %.2f, Rois: %s, [H%%, A/Day/12H/6H: %.1f%%/%.1f%%/%.1f%%/%.1f%%], [A/D/3/2/1H: %s%%/%.1f%%/%.1f%%/%.1f%%/%.1f%%], MinInv: %s%s",
		s.CopyCount, s.MatchedCount, s.LatestMatchedCount, pnl, s.Rois.lastNRecords(TheConfig.LastNHoursNoDips),
		s.roiPerHour*100, s.lastDayRoiPerHr*100, s.last12HrRoiPerHr*100, s.last6HrRoiPerHr*100, s.Roi,
		s.lastDayRoiChange*100, s.last3HrRoiChange*100, s.last2HrRoiChange*100, s.lastHrRoiChange*100, s.MinInvestment, ranking)
}

func display(s *Strategy, grid *Grid, action string, index int, length int) string {
	if grid == nil && s == nil {
		return "Strategy and Grid are both nil"
	}
	if grid != nil {
		if gl, ok := globalStrategies[grid.SID]; s == nil && ok {
			s = gl
		}
	}
	ss := ""
	gg := ""
	seq := ""
	direction := ""
	strategyId := ""
	symbol := ""
	leverage := ""
	runTime := ""
	priceRange := ""
	grids := ""
	marketPrice := 0.0
	formatPriceRange := func(lower, upper string) string {
		l, _ := strconv.ParseFloat(lower, 64)
		u, _ := strconv.ParseFloat(upper, 64)
		diff := (u/l - 1) * 100
		return fmt.Sprintf("%s-%s, %.2f%%", lower, upper, diff)
	}
	formatRunTime := func(rt int64) string {
		return fmt.Sprintf("%s", shortDur((time.Duration(rt) * time.Second).Round(time.Minute)))
	}
	if grid == nil {
		marketPrice, _ = getSessionSymbolPrice(s.Symbol)
		direction = DirectionMap[s.Direction]
		symbol = s.Symbol
		strategyId = fmt.Sprintf("%d", s.SID)
		leverage = fmt.Sprintf("%dX", s.StrategyParams.Leverage)
		runTime = formatRunTime(int64(s.RunningTime))
		priceRange = formatPriceRange(s.StrategyParams.LowerLimit, s.StrategyParams.UpperLimit)
		grids = fmt.Sprintf("%d", s.StrategyParams.GridCount)
	} else {
		marketPrice, _ = fetchMarketPrice(grid.Symbol)
		direction = grid.Direction
		symbol = grid.Symbol
		strategyId = fmt.Sprintf("%d", grid.SID)
		leverage = fmt.Sprintf("%.2fX%d=%d", grid.initialValue, grid.InitialLeverage, int(grid.initialValue*float64(grid.InitialLeverage)))
		runTime = formatRunTime(time.Now().Unix() - grid.BookTime/1000)
		priceRange = formatPriceRange(grid.GridLowerLimit, grid.GridUpperLimit)
		grids = fmt.Sprintf("%d", grid.GridCount)

		if s != nil {
			if DirectionMap[s.Direction] != grid.Direction {
				direction = fmt.Sprintf("S/G: %s/%s", DirectionMap[s.Direction], grid.Direction)
			}
			if s.Symbol != grid.Symbol {
				symbol = fmt.Sprintf("S/G: %s/%s", s.Symbol, grid.Symbol)
			}
			if s.SID != grid.SID {
				strategyId = fmt.Sprintf("S/G: %d/%d", s.SID, grid.SID)
			}
			if s.StrategyParams.GridCount != grid.GridCount {
				grids = fmt.Sprintf("S/G: %d/%d", s.StrategyParams.GridCount, grid.GridCount)
			}
			leverage = fmt.Sprintf("%dX/%.2fX%d=%d", s.StrategyParams.Leverage, grid.initialValue, grid.InitialLeverage, int(grid.initialValue*float64(grid.InitialLeverage)))
			runTime = fmt.Sprintf("%s/%s", formatRunTime(int64(s.RunningTime)), formatRunTime(time.Now().Unix()-grid.BookTime/1000))
			if s.StrategyParams.LowerLimit != grid.GridLowerLimit || s.StrategyParams.UpperLimit != grid.GridUpperLimit {
				priceRange = fmt.Sprintf("S/G: %s/%s", formatPriceRange(s.StrategyParams.LowerLimit, s.StrategyParams.UpperLimit),
					formatPriceRange(grid.GridLowerLimit, grid.GridUpperLimit))
			}
		}
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

	return fmt.Sprintf("* [%s%s%s, %s, %s, %s @ %f, %s Grids, %s] %s: %s%s",
		seq, symbol, direction, leverage, runTime,
		priceRange, marketPrice, grids, strategyId, action, ss, gg)
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
	if err != nil {
		return nil, err
	}
	return roi.Data, nil
}

type SortPair struct {
	Sort       string
	Direction  *int
	Count      int
	RuntimeMax time.Duration
	RuntimeMin time.Duration
}

func mergeStrategies(strategyType int, sps ...SortPair) (*TrackedStrategies, error) {
	sss := make(Strategies, 0)
	for _, sp := range sps {
		if sp.Count == 0 {
			sp.Count = TheConfig.StrategiesCount
		}
		if sp.RuntimeMin == 0 {
			sp.RuntimeMin = time.Duration(TheConfig.RuntimeMinHours) * time.Hour
		}
		if sp.RuntimeMax == 0 {
			sp.RuntimeMax = time.Duration(TheConfig.RuntimeMaxHours) * time.Hour
		}
		by, err := _getTopStrategies(sp.Sort, sp.Direction, strategyType, sp.RuntimeMin, sp.RuntimeMax, sp.Count)
		if err != nil {
			return nil, err
		}
		sss = append(sss, by...)
	}
	sort.Slice(sss, func(i, j int) bool {
		roiI, _ := strconv.ParseFloat(sss[i].Roi, 64)
		roiJ, _ := strconv.ParseFloat(sss[j].Roi, 64)
		return roiI > roiJ
	})
	return sss.toTrackedStrategies(), nil
}

func getTopStrategies(strategyType int) (*TrackedStrategies, error) {
	merged, err := mergeStrategies(strategyType,
		SortPair{Sort: SortByRoi, RuntimeMin: 3 * time.Hour, RuntimeMax: 48 * time.Hour},
		SortPair{Sort: SortByRoi, RuntimeMin: 48 * time.Hour, RuntimeMax: 168 * time.Hour},
		SortPair{Sort: SortByRoi, RuntimeMin: 168 * time.Hour, RuntimeMax: 360 * time.Hour, Count: 20},
		//SortPair{Sort: SortByRoi, Direction: IntPointer(SHORT), Count: 15},
		//SortPair{Sort: SortByRoi, Direction: IntPointer(NEUTRAL), Count: 15},
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
	strategies, res, err := privateRequest(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryTopStrategy", "POST",
		query, &StrategiesResponse{})
	// this API returns different results based on if user agents or another header is passed to it
	// if no such header is passed to it, it returns grids count min 2 (high risk)
	if err != nil {
		return nil, err
	}
	generic := map[string]interface{}{}
	err = json.Unmarshal(res, &generic)
	if err != nil {
		return nil, err
	}
	if len(generic) != 6 {
		Discordf("Error, strategies root response has length %d: %+v", len(generic),
			generic)
	}
	for _, v := range generic["data"].([]interface{}) {
		if len(v.(map[string]interface{})) != 14 {
			Discordf("Error, strategy response has length %d: %+v", len(v.(map[string]interface{})),
				v)
		}
	}
	return strategies.Data, nil
}
