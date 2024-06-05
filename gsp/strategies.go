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
	"github.com/syohex/go-texttable"
	"sort"
	"strconv"
	"time"
)

type StrategyQuery struct {
	Sort       string
	Direction  *int
	Count      int
	RuntimeMax time.Duration
	RuntimeMin time.Duration
	Symbol     string
	Type       int
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
	Rois               StrategyRoi
	Symbol             string `json:"symbol"`
	CopyCount          int    `json:"copyCount"`
	RoiStr             string `json:"roi"`
	PnlStr             string `json:"pnl"`
	RunningTime        int    `json:"runningTime"`
	SID                int    `json:"strategyId"`
	StrategyType       int    `json:"strategyType"`
	Direction          int    `json:"direction"`
	UserID             int    `json:"userId"`
	Roi                float64
	Pnl                float64
	LastDayRoiChange   float64
	Last3HrRoiChange   float64
	Last2HrRoiChange   float64
	LastHrRoiChange    float64
	LastDayRoiPerHr    float64
	Last15HrRoiPerHr   float64
	Last12HrRoiPerHr   float64
	Last9HrRoiPerHr    float64
	Last6HrRoiPerHr    float64
	Last3HrRoiPerHr    float64
	LastNHrNoDip       bool
	LastNHrAllPositive bool
	RoiPerHour         float64
	ReasonNotPicked    []string
	TimeDiscovered     time.Time
	TimeNotFound       time.Time
	RoisFetchedAt      time.Time
	Concluded          bool
	UserMetricsDB      `json:"-"`
	StrategyParams     StrategyParams `json:"strategyParams"`
	TrailingType       string         `json:"trailingType"`
	LatestMatchedCount int            `json:"latestMatchedCount"`
	MatchedCount       int            `json:"matchedCount"`
	MinInvestment      string         `json:"minInvestment"`
}

type StrategyParams struct {
	Type           string  `json:"type"`
	LowerLimitStr  string  `json:"lowerLimit"`
	UpperLimitStr  string  `json:"upperLimit"`
	LowerLimit     float64 `json:"-"`
	UpperLimit     float64 `json:"-"`
	GridCount      int     `json:"gridCount"`
	TriggerPrice   *string `json:"triggerPrice"`
	StopLowerLimit *string `json:"stopLowerLimit"`
	StopUpperLimit *string `json:"stopUpperLimit"`
	BaseAsset      any     `json:"baseAsset"`
	QuoteAsset     any     `json:"quoteAsset"`
	Leverage       int     `json:"leverage"`
	TrailingUp     bool    `json:"trailingUp"`
	TrailingDown   bool    `json:"trailingDown"`
}

type UserPair struct {
	Id    int
	Count int
}

type TrackedStrategies struct {
	StrategiesBySID            map[int]*Strategy
	StrategiesByUserId         map[int]Strategies
	Strategies                 Strategies
	UserRankings               map[int]int
	UsersWithMoreThan1Strategy []UserPair
	SymbolCount                map[string]int
	SymbolDirectionCount       SDCount
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

func (by Strategies) GetLSN() (int, int, int) {
	longs := 0
	shorts := 0
	neutrals := 0
	for _, s := range by {
		switch s.Direction {
		case LONG:
			longs++
		case SHORT:
			shorts++
		case NEUTRAL:
			neutrals++
		}
	}
	return longs, shorts, neutrals
}

func (by Strategies) toTrackedStrategies() *TrackedStrategies {
	sss := &TrackedStrategies{
		StrategiesBySID:      make(map[int]*Strategy),
		StrategiesByUserId:   make(map[int]Strategies),
		UserRankings:         make(map[int]int),
		SymbolCount:          make(map[string]int),
		SymbolDirectionCount: make(SDCount),
		Longs:                mapset.NewSet[int](),
		Shorts:               mapset.NewSet[int](),
		Neutrals:             mapset.NewSet[int](),
	}
	for _, s := range by {
		_, ok := sss.StrategiesBySID[s.SID]
		if ok {
			continue
		}
		sss.StrategiesBySID[s.SID] = s
		if _, ok := sss.StrategiesByUserId[s.UserID]; !ok {
			sss.StrategiesByUserId[s.UserID] = make(Strategies, 0)
		}
		sss.StrategiesByUserId[s.UserID] = append(sss.StrategiesByUserId[s.UserID], s)
		sss.UserRankings[s.UserID] += 1
		sss.SymbolCount[s.Symbol] += 1
		if _, ok := sss.SymbolDirectionCount[s.Symbol]; !ok {
			sss.SymbolDirectionCount[s.Symbol] = make(map[string]int)
		}
		sss.SymbolDirectionCount[s.Symbol][DirectionMap[s.Direction]] += 1
		if s.Direction == LONG {
			sss.Longs.Add(s.SID)
		} else if s.Direction == SHORT {
			sss.Shorts.Add(s.SID)
		} else {
			sss.Neutrals.Add(s.SID)
		}
		if sss.Highest.CopyCount == nil || s.CopyCount > *sss.Highest.CopyCount {
			sss.Highest.CopyCount = &s.CopyCount
		}
		if sss.Lowest.CopyCount == nil || s.CopyCount < *sss.Lowest.CopyCount {
			sss.Lowest.CopyCount = &s.CopyCount
		}
		if sss.Highest.Roi == nil || s.Roi > *sss.Highest.Roi {
			sss.Highest.Roi = &s.Roi
		}
		if sss.Lowest.Roi == nil || s.Roi < *sss.Lowest.Roi {
			sss.Lowest.Roi = &s.Roi
		}
		if sss.Highest.Pnl == nil || s.Pnl > *sss.Highest.Pnl {
			sss.Highest.Pnl = &s.Pnl
		}
		if sss.Lowest.Pnl == nil || s.Pnl < *sss.Lowest.Pnl {
			sss.Lowest.Pnl = &s.Pnl
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
	sss.Ids = mapset.NewSetFromMapKeys(sss.StrategiesBySID)
	return sss
}

func (t *TrackedStrategies) findStrategyRanking(s Strategy) int {
	symbolDirection := mapset.NewSet[string]()
	counter := 0
	sd := s.SD()
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
	tbl := &texttable.TextTable{}
	tbl.SetHeader(fmt.Sprintf("Symbol %d", len(t.SymbolCount)),
		fmt.Sprintf("L %d", t.Longs.Cardinality()),
		fmt.Sprintf("S %d", t.Shorts.Cardinality()),
		fmt.Sprintf("N %d", t.Neutrals.Cardinality()))
	symbols := mapset.NewSetFromMapKeys(t.SymbolCount).ToSlice()
	sort.Strings(symbols)
	for _, symbol := range symbols {
		directionMap := t.SymbolDirectionCount[symbol]
		tbl.AddRow(utils.FormatPair(symbol), fmt.Sprintf("%d", directionMap["LONG"]),
			fmt.Sprintf("%d", directionMap["SHORT"]), fmt.Sprintf("%d", directionMap["NEUTRAL"]))
	}
	return fmt.Sprintf("%d, H: %v, L: %v\n```\n%s```\n%v",
		len(t.StrategiesBySID), utils.AsJson(t.Highest), utils.AsJson(t.Lowest),
		tbl.Draw(), t.UsersWithMoreThan1Strategy)
}

func (t *TrackedStrategies) Exists(id int) bool {
	return t.Ids.Contains(id)
}

func (s *Strategy) MarketPriceWithinRange() bool {
	marketPrice, _ := sdk.GetSessionSymbolPrice(s.Symbol)
	return marketPrice > s.StrategyParams.LowerLimit && marketPrice < s.StrategyParams.UpperLimit
}

func (s *Strategy) String() string {
	ranking := ""
	ended := ""
	if Bundle != nil {
		ranking = fmt.Sprintf(", Raw: %d, FilterdSD: %d", Bundle.Raw.findStrategyRanking(*s),
			GetPool().findStrategyRanking(*s))
	}
	if !s.Rois.isRunning() {
		ended = "Ended: " + time.Unix(s.Rois[0].Time, 0).Format("2006-01-02 15:04:05") + " ,"
	}
	return fmt.Sprintf("%sPnL: %.2f, Rois: %s, [A/D/3/2/1H: %.1f%%/%.1f%%/%.1f%%/%.1f%%/%.1f%%], MinInv: %s%s, User: $%.1f/$%.1f",
		ended, s.Pnl, s.Rois.lastNRecords(config.TheConfig.LastNHoursNoDips),
		s.Rois[0].Roi,
		s.LastDayRoiChange*100, s.Last3HrRoiChange*100, s.Last2HrRoiChange*100, s.LastHrRoiChange*100, s.MinInvestment, ranking, s.UserInput, s.UserTotalInput)
}

func (s *Strategy) GetMetric() float64 {
	return s.Last3HrRoiPerHr
}

func (s *Strategy) SD() string {
	return s.Symbol + DirectionMap[s.Direction]
}

func (s *Strategy) PopulateRois() error {
	id := s.SID
	rois, err := RoisCache.Get(fmt.Sprintf("%d-%d", id, s.UserID))
	if err != nil {
		return err
	}
	s.Rois = rois
	if len(s.Rois) > 1 {
		s.Roi = s.Rois[0].Roi
		s.LastDayRoiChange = s.Rois.GetRoiChange(24 * time.Hour)
		s.Last3HrRoiChange = s.Rois.GetRoiChange(3 * time.Hour)
		s.Last2HrRoiChange = s.Rois.GetRoiChange(2 * time.Hour)
		s.LastHrRoiChange = s.Rois.GetRoiChange(1 * time.Hour)
		s.LastDayRoiPerHr = s.Rois.GetRoiPerHr(24 * time.Hour)
		s.Last15HrRoiPerHr = s.Rois.GetRoiPerHr(15 * time.Hour)
		s.Last12HrRoiPerHr = s.Rois.GetRoiPerHr(12 * time.Hour)
		s.Last9HrRoiPerHr = s.Rois.GetRoiPerHr(9 * time.Hour)
		s.Last6HrRoiPerHr = s.Rois.GetRoiPerHr(6 * time.Hour)
		s.Last3HrRoiPerHr = s.Rois.GetRoiPerHr(3 * time.Hour)
		s.LastNHrNoDip = s.Rois.AllPositive(time.Duration(config.TheConfig.LastNHoursNoDips)*time.Hour, 0)
		s.LastNHrAllPositive = s.Rois.AllPositive(time.Duration(config.TheConfig.LastNHoursAllPositive)*time.Hour, 0.005)
		s.RoiPerHour = (s.Roi - s.Rois[len(s.Rois)-1].Roi) / float64(s.RunningTime/3600)
	}
	return nil
}

func (s *Strategy) Sanitize() {
	s.Roi, _ = strconv.ParseFloat(s.RoiStr, 64)
	s.Roi /= 100
	s.Pnl, _ = strconv.ParseFloat(s.PnlStr, 64)

	s.StrategyParams.LowerLimit, _ = strconv.ParseFloat(s.StrategyParams.LowerLimitStr, 64)
	s.StrategyParams.UpperLimit, _ = strconv.ParseFloat(s.StrategyParams.UpperLimitStr, 64)
}

func (rois StrategyRoi) isRunning() bool {
	latestTime := time.Unix(rois[0].Time, 0)
	return time.Now().Sub(latestTime) <= 95*time.Minute
}

func Display(s *Strategy, grid *Grid, action string, index int, length int) string {
	if s != nil {
		err := s.PopulateRois()
		if err != nil {
			discord.Errorf("Error populating rois for %d: %s", s.SID, err)
			s = nil
		}
	}
	if grid == nil && s == nil {
		return "Strategy and Grid are both nil"
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
	wl := ""
	formatPriceRange := func(lower, upper, symbol, direction string) string {
		mp, _ := sdk.GetSessionSymbolPrice(symbol)
		l, _ := strconv.ParseFloat(lower, 64)
		u, _ := strconv.ParseFloat(upper, 64)
		diff := (u/l - 1) * 100
		relative := 0.0
		switch direction {
		case "LONG":
			relative = (mp - l) / l * 100
		case "SHORT":
			relative = (u - mp) / u * 100
		case "NEUTRAL":
			mid := (l + u) / 2
			relative = (mp - mid) / mid * 100
		}
		return fmt.Sprintf("%s-%s, %.1f%%, R: %.1f%%", lower, upper, diff, relative)
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
		priceRange = formatPriceRange(s.StrategyParams.LowerLimitStr, s.StrategyParams.UpperLimitStr, s.Symbol, DirectionMap[s.Direction])
		grids = fmt.Sprintf("%d", s.StrategyParams.GridCount)
	} else {
		marketPrice, _ = sdk.GetSessionSymbolPrice(grid.Symbol)
		direction = grid.Direction
		symbol = grid.Symbol
		strategyId = fmt.Sprintf("%d", grid.SID)
		leverage = fmt.Sprintf("%.2fX%d=%d", grid.InitialValue, grid.InitialLeverage, int(grid.InitialValue*float64(grid.InitialLeverage)))
		runTime = formatRunTime(time.Now().Unix() - grid.BookTime/1000)
		priceRange = formatPriceRange(grid.GridLowerLimit, grid.GridUpperLimit, grid.Symbol, grid.Direction)
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
			if s.StrategyParams.LowerLimitStr != grid.GridLowerLimit || s.StrategyParams.UpperLimitStr != grid.GridUpperLimit {
				priceRange = fmt.Sprintf("S/G: %s/%s", formatPriceRange(s.StrategyParams.LowerLimitStr, s.StrategyParams.UpperLimitStr, s.Symbol, DirectionMap[s.Direction]),
					formatPriceRange(grid.GridLowerLimit, grid.GridUpperLimit, grid.Symbol, grid.Direction))
			}
			userWl, err := UserWLCache.Get(fmt.Sprintf("%d", s.UserID))
			if err == nil {
				wl = userWl.String()
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

	return fmt.Sprintf("* [%s%s%s, %s, %s, %f/%s, %s Grids, %s %s] %s: %s%s",
		seq, utils.FormatPair(symbol), direction, leverage, runTime,
		marketPrice, priceRange, grids, strategyId, wl, action, ss, gg)
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

var DirectionSMap = map[string]int{
	"NEUTRAL": NEUTRAL,
	"LONG":    LONG,
	"SHORT":   SHORT,
}

func mergeStrategies(sps ...StrategyQuery) (*TrackedStrategies, error) {
	sss := make(Strategies, 0)
	for _, sp := range sps {
		if sp.Count == 0 {
			sp.Count = config.TheConfig.StrategiesCount
		}
		if sp.RuntimeMin == -1 {
			sp.RuntimeMin = time.Duration(config.TheConfig.RuntimeMinHours) * time.Hour
		}
		if sp.RuntimeMax == -1 {
			sp.RuntimeMax = time.Duration(config.TheConfig.RuntimeMaxHours) * time.Hour
		}
		by, err := _getTopStrategies(sp.Sort, sp.Direction, sp.Type, sp.RuntimeMin, sp.RuntimeMax, sp.Count, sp.Symbol)
		if err != nil {
			return nil, err
		}
		sss = append(sss, by...)
	}
	sort.Slice(sss, func(i, j int) bool {
		return sss[i].Roi > sss[j].Roi
	})
	return sss.toTrackedStrategies(), nil
}

func getTopStrategies(symbol string) (*TrackedStrategies, error) {
	var queries []StrategyQuery
	for i := 0; i < 48; i += 2 {
		queries = append(queries, StrategyQuery{Type: FUTURE, Sort: SortByRoi, RuntimeMin: time.Duration(i) * time.Hour, RuntimeMax: time.Duration(i+2) * time.Hour, Symbol: symbol})
		queries = append(queries, StrategyQuery{Type: SPOT, Sort: SortByRoi, RuntimeMin: time.Duration(i) * time.Hour, RuntimeMax: time.Duration(i+2) * time.Hour, Symbol: symbol})
	}
	merged, err := mergeStrategies(queries...)
	if err != nil {
		return nil, err
	}
	return merged, nil
}

func DiscoverRootStrategy(sid int, symbol string, direction int, roughRuntime time.Duration) (*Strategy, error) {
	getQuery := func(quote string) StrategyQuery {
		sym := symbol[:len(symbol)-4] + quote
		return StrategyQuery{Type: FUTURE, Sort: SortByPnl,
			RuntimeMin: roughRuntime - 4*time.Hour,
			RuntimeMax: roughRuntime + time.Duration(config.TheConfig.MaxLookBackBookingMinutes+240)*time.Minute,
			Symbol:     sym, Direction: utils.IntPointer(direction),
			Count: 2000}
	}
	merged, err := mergeStrategies(getQuery("USDT"), getQuery("USDC"))
	if err != nil {
		return nil, err
	}
	for _, s := range merged.Strategies {
		if s.SID == sid {
			return s, nil
		}
	}
	return nil, nil
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
	strategies, res, err := request.Request(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryTopStrategy",
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
	for _, strategy := range strategies.Data {
		strategy.Sanitize()
	}
	return strategies.Data, nil
}
