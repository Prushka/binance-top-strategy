package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/utils"
	"encoding/json"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

type StrategyQuery struct {
	Sort       string
	Direction  *int
	Count      int
	RuntimeMax time.Duration
	RuntimeMin time.Duration
	Symbol     string
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

type StrategiesResponse struct {
	Data  Strategies `json:"data"`
	Total int        `json:"total"`
	request.BinanceBaseResponse
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

type UserPair struct {
	Id    int
	Count int
}

type TrackedStrategies struct {
	StrategiesById             map[int]*Strategy
	StrategiesByUserId         map[int]Strategies
	Strategies                 Strategies
	UserRankings               map[int]int
	UsersWithMoreThan1Strategy []UserPair
	SymbolCount                map[string]int
	SymbolDirectionCount       map[string]int
	Longs                      mapset.Set[int]
	Shorts                     mapset.Set[int]
	Neutrals                   mapset.Set[int]
	Highest                    StrategyMetrics
	Lowest                     StrategyMetrics
	Ids                        mapset.Set[int]
}

type QueryTopStrategy struct {
	Page           int    `json:"page"`
	Rows           int    `json:"rows"`
	Direction      *int   `json:"direction"`
	StrategyType   int    `json:"strategyType"`
	Symbol         string `json:"symbol,omitempty"`
	Zone           string `json:"zone"`
	RunningTimeMin int    `json:"runningTimeMin"`
	RunningTimeMax int    `json:"runningTimeMax"`
	Sort           string `json:"sort"`
}

type Strategies []*Strategy

func (by Strategies) toTrackedStrategies() *TrackedStrategies {
	sss := &TrackedStrategies{
		StrategiesById:       make(map[int]*Strategy),
		StrategiesByUserId:   make(map[int]Strategies),
		UserRankings:         make(map[int]int),
		SymbolCount:          make(map[string]int),
		SymbolDirectionCount: make(map[string]int),
		Longs:                mapset.NewSet[int](),
		Shorts:               mapset.NewSet[int](),
		Neutrals:             mapset.NewSet[int](),
	}
	for _, s := range by {
		_, ok := sss.StrategiesById[s.SID]
		if ok {
			continue
		}
		sss.StrategiesById[s.SID] = s
		if _, ok := sss.StrategiesByUserId[s.UserID]; !ok {
			sss.StrategiesByUserId[s.UserID] = make(Strategies, 0)
		}
		sss.StrategiesByUserId[s.UserID] = append(sss.StrategiesByUserId[s.UserID], s)
		sss.UserRankings[s.UserID] += 1
		sss.SymbolCount[s.Symbol] += 1
		sss.SymbolDirectionCount[s.Symbol+DirectionMap[s.Direction]] += 1
		if s.Direction == LONG {
			sss.Longs.Add(s.SID)
		} else if s.Direction == SHORT {
			sss.Shorts.Add(s.SID)
		} else {
			sss.Neutrals.Add(s.SID)
		}
		roi, _ := strconv.ParseFloat(s.Roi, 64)
		pnl, _ := strconv.ParseFloat(s.Pnl, 64)
		if sss.Highest.CopyCount == nil || s.CopyCount > *sss.Highest.CopyCount {
			sss.Highest.CopyCount = &s.CopyCount
		}
		if sss.Lowest.CopyCount == nil || s.CopyCount < *sss.Lowest.CopyCount {
			sss.Lowest.CopyCount = &s.CopyCount
		}
		if sss.Highest.Roi == nil || roi > *sss.Highest.Roi {
			sss.Highest.Roi = &roi
		}
		if sss.Lowest.Roi == nil || roi < *sss.Lowest.Roi {
			sss.Lowest.Roi = &roi
		}
		if sss.Highest.Pnl == nil || pnl > *sss.Highest.Pnl {
			sss.Highest.Pnl = &pnl
		}
		if sss.Lowest.Pnl == nil || pnl < *sss.Lowest.Pnl {
			sss.Lowest.Pnl = &pnl
		}
		if sss.Highest.runningTime == nil || s.RunningTime > *sss.Highest.runningTime {
			sss.Highest.runningTime = &s.RunningTime
		}
		if sss.Lowest.runningTime == nil || s.RunningTime < *sss.Lowest.runningTime {
			sss.Lowest.runningTime = &s.RunningTime
		}
		if sss.Highest.MatchedCount == nil || s.MatchedCount > *sss.Highest.MatchedCount {
			sss.Highest.MatchedCount = &s.MatchedCount
		}
		if sss.Lowest.MatchedCount == nil || s.MatchedCount < *sss.Lowest.MatchedCount {
			sss.Lowest.MatchedCount = &s.MatchedCount
		}
		if sss.Highest.LatestMatchedCount == nil || s.LatestMatchedCount > *sss.Highest.LatestMatchedCount {
			sss.Highest.LatestMatchedCount = &s.LatestMatchedCount
		}
		if sss.Lowest.LatestMatchedCount == nil || s.LatestMatchedCount < *sss.Lowest.LatestMatchedCount {
			sss.Lowest.LatestMatchedCount = &s.LatestMatchedCount
		}
		sss.Strategies = append(sss.Strategies, s)
	}
	if sss.Highest.runningTime != nil {
		sss.Highest.RunningTime = utils.StringPointer(fmt.Sprintf("%s", time.Duration(*sss.Highest.runningTime)*time.Second))
	}
	if sss.Lowest.runningTime != nil {
		sss.Lowest.RunningTime = utils.StringPointer(fmt.Sprintf("%s", time.Duration(*sss.Lowest.runningTime)*time.Second))
	}
	for userId, count := range sss.UserRankings {
		if count > 1 {
			sss.UsersWithMoreThan1Strategy = append(sss.UsersWithMoreThan1Strategy, UserPair{Id: userId, Count: count})
		}
	}
	sort.Slice(sss.UsersWithMoreThan1Strategy, func(i, j int) bool {
		return sss.UsersWithMoreThan1Strategy[i].Count > sss.UsersWithMoreThan1Strategy[j].Count
	})
	sss.Ids = mapset.NewSetFromMapKeys(sss.StrategiesById)
	return sss
}

func (t *TrackedStrategies) findStrategyRanking(s Strategy) int {
	symbolDirection := mapset.NewSet[string]()
	counter := 0
	sd := s.Symbol + DirectionMap[s.Direction]
	for _, s := range t.Strategies {
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
	return fmt.Sprintf("%d, Symbols: %d, L/S/N: %d/%d/%d, SymbolDirections: %v, H: %v, L: %v",
		len(t.StrategiesById), len(t.SymbolCount),
		t.Longs.Cardinality(), t.Shorts.Cardinality(), t.Neutrals.Cardinality(),
		utils.AsJson(t.SymbolDirectionCount), utils.AsJson(t.Highest), utils.AsJson(t.Lowest))
}

func (t *TrackedStrategies) Exists(id int) bool {
	return t.Ids.Contains(id)
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
	if Bundle != nil {
		ranking = fmt.Sprintf(", Rank: Raw: %d, FilterdSD: %d", Bundle.Raw.findStrategyRanking(s),
			Bundle.FilteredSortedBySD.findStrategyRanking(s))
	}
	return fmt.Sprintf("Cpy: %d, Mch: [%d, %d], PnL: %.2f, Rois: %s, [H%%, A/Day/12H/6H: %.1f%%/%.1f%%/%.1f%%/%.1f%%], [A/D/3/2/1H: %s%%/%.1f%%/%.1f%%/%.1f%%/%.1f%%], MinInv: %s%s",
		s.CopyCount, s.MatchedCount, s.LatestMatchedCount, pnl, s.Rois.lastNRecords(config.TheConfig.LastNHoursNoDips),
		s.roiPerHour*100, s.lastDayRoiPerHr*100, s.last12HrRoiPerHr*100, s.last6HrRoiPerHr*100, s.Roi,
		s.lastDayRoiChange*100, s.last3HrRoiChange*100, s.last2HrRoiChange*100, s.lastHrRoiChange*100, s.MinInvestment, ranking)
}

func Display(s *Strategy, grid *Grid, action string, index int, length int) string {
	if grid == nil && s == nil {
		return "Strategy and Grid are both nil"
	}
	if grid != nil {
		if gl, ok := GStrats[grid.SID]; s == nil && ok {
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
		return fmt.Sprintf("%s", utils.ShortDur((time.Duration(rt) * time.Second).Round(time.Minute)))
	}
	if grid == nil {
		marketPrice, _ = sdk.GetSessionSymbolPrice(s.Symbol)
		direction = DirectionMap[s.Direction]
		symbol = s.Symbol
		strategyId = fmt.Sprintf("%d", s.SID)
		leverage = fmt.Sprintf("%dX", s.StrategyParams.Leverage)
		runTime = formatRunTime(int64(s.RunningTime))
		priceRange = formatPriceRange(s.StrategyParams.LowerLimit, s.StrategyParams.UpperLimit)
		grids = fmt.Sprintf("%d", s.StrategyParams.GridCount)
	} else {
		marketPrice, _ = sdk.GetSessionSymbolPrice(grid.Symbol)
		direction = grid.Direction
		symbol = grid.Symbol
		strategyId = fmt.Sprintf("%d", grid.SID)
		leverage = fmt.Sprintf("%.2fX%d=%d", grid.InitialValue, grid.InitialLeverage, int(grid.InitialValue*float64(grid.InitialLeverage)))
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
			leverage = fmt.Sprintf("%dX/%.2fX%d=%d", s.StrategyParams.Leverage, grid.InitialValue, grid.InitialLeverage, int(grid.InitialValue*float64(grid.InitialLeverage)))
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

func mergeStrategies(strategyType int, sps ...StrategyQuery) (*TrackedStrategies, error) {
	sss := make(Strategies, 0)
	for _, sp := range sps {
		if sp.Count == 0 {
			sp.Count = config.TheConfig.StrategiesCount
		}
		if sp.RuntimeMin == 0 {
			sp.RuntimeMin = time.Duration(config.TheConfig.RuntimeMinHours) * time.Hour
		}
		if sp.RuntimeMax == 0 {
			sp.RuntimeMax = time.Duration(config.TheConfig.RuntimeMaxHours) * time.Hour
		}
		by, err := _getTopStrategies(sp.Sort, sp.Direction, strategyType, sp.RuntimeMin, sp.RuntimeMax, sp.Count, sp.Symbol)
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

func getTopStrategies(strategyType int, symbol string) (*TrackedStrategies, error) {
	merged, err := mergeStrategies(strategyType,
		StrategyQuery{Sort: SortByRoi, RuntimeMin: 3 * time.Hour, RuntimeMax: 48 * time.Hour, Symbol: symbol},
		StrategyQuery{Sort: SortByRoi, RuntimeMin: 48 * time.Hour, RuntimeMax: 168 * time.Hour, Symbol: symbol},
		StrategyQuery{Sort: SortByRoi, RuntimeMin: 168 * time.Hour, RuntimeMax: 360 * time.Hour, Count: 20, Symbol: symbol},
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

func _getTopStrategies(sort string, direction *int, strategyType int, runningTimeMin time.Duration, runningTimeMax time.Duration, strategyCount int, symbol string) (Strategies, error) {
	query := &QueryTopStrategy{
		Page:           1,
		Rows:           strategyCount,
		StrategyType:   strategyType,
		RunningTimeMax: int(runningTimeMax.Seconds()),
		RunningTimeMin: int(runningTimeMin.Seconds()),
		Sort:           sort,
		Direction:      direction,
		Symbol:         symbol,
	}
	strategies, res, err := request.PrivateRequest(
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
		discord.Infof("Error, strategies root response has length %d: %+v", len(generic),
			generic)
	}
	for _, v := range generic["data"].([]interface{}) {
		if len(v.(map[string]interface{})) != 14 {
			discord.Infof("Error, strategy response has length %d: %+v", len(v.(map[string]interface{})),
				v)
		}
	}
	return strategies.Data, nil
}