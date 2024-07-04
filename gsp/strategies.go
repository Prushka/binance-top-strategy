package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/utils"
	"encoding/json"
	"fmt"
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

func (s *Strategy) GetMatchedRatio() float64 {
	assumedRatioPerGrid := (s.StrategyParams.UpperLimit/s.StrategyParams.LowerLimit - 1) / float64(s.StrategyParams.GridCount)
	return (float64(s.MatchedCount) / (float64(s.RunningTime) / 3600)) * assumedRatioPerGrid * s.GetNormalizedRoi() * 12000
}

func (s *Strategy) GetNormalizedRoi() float64 {
	return s.Roi / (float64(s.RunningTime) / 3600)
}

func (grid *Grid) GetMatchedRatio() float64 {
	upper, _ := strconv.ParseFloat(grid.GridUpperLimit, 64)
	lower, _ := strconv.ParseFloat(grid.GridLowerLimit, 64)
	assumedRatioPerGrid := (upper/lower - 1) / float64(grid.GridCount)
	return (float64(grid.MatchedCount) / (float64(grid.GetRunTime().Seconds()) / 3600)) * assumedRatioPerGrid * grid.GetNormalizedRoi() * 12000
}

func (grid *Grid) GetNormalizedRoi() float64 {
	return grid.LastRoi / (float64(grid.GetRunTime().Seconds()) / 3600)
}

func (s *Strategy) MarketPriceWithinRange() bool {
	marketPrice, _ := sdk.GetSessionSymbolPrice(s.Symbol)
	return marketPrice > s.StrategyParams.LowerLimit && marketPrice < s.StrategyParams.UpperLimit
}

func (s *Strategy) String() string {
	ended := ""
	rois := ""
	if len(s.Rois) > 0 {
		if !s.Rois.isRunning() {
			ended = "Ended: " + time.Unix(s.Rois[0].Time, 0).Format("2006-01-02 15:04:05") + " ,"
		}
		rois = fmt.Sprintf("Rois: %s, ", s.Rois.lastNRecords(6))
	}
	return fmt.Sprintf("%sPnL: %.2f, %sMinInv: %s, User: $%.1f/$%.1f",
		ended, s.Pnl, rois,
		s.MinInvestment, s.UserInput, s.UserTotalInput)
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
	if s != nil && len(s.Rois) == 0 {
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
	userPoolStrategies := ""
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
		userPoolStrategies = fmt.Sprintf("Pool: %d",
			len(GetPool().ByUID()[s.UserID]))
	}
	if grid != nil {
		gg = ", " + grid.String()
	}
	if length != 0 {
		seq = fmt.Sprintf("%d/%d - ", index, length)
	}

	return fmt.Sprintf("* [%s%s%s, %s, %s, %f/%s, %s Grids, %s, %s, %s] %s: %s%s",
		seq, utils.FormatPair(symbol), direction, leverage, runTime,
		marketPrice, priceRange, grids, strategyId, wl, userPoolStrategies, action, ss, gg)
}

const (
	SPOT         = 1
	FUTURE       = 2
	NEUTRAL      = 1
	LONG         = 2
	SHORT        = 3
	TOTAL        = 99
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

func mergeStrategies(sps ...StrategyQuery) (Strategies, error) {
	sss := make(Strategies, 0)
	for _, sp := range sps {
		if sp.Count == 0 {
			sp.Count = 3000
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
		return sss[i].Pnl > sss[j].Pnl
	})
	return sss, nil
}

func getTopStrategies(sType int) (Strategies, error) {
	var queries []StrategyQuery
	for i := 0; i < 48; i += 2 {
		queries = append(queries, StrategyQuery{Type: sType, Sort: SortByPnl, RuntimeMin: time.Duration(i) * time.Hour, RuntimeMax: time.Duration(i+2) * time.Hour})
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
			RuntimeMin: roughRuntime - 9*time.Hour,
			RuntimeMax: roughRuntime + 9*time.Hour,
			Symbol:     sym, Direction: utils.IntPointer(direction),
			Count: 2000}
	}
	merged, err := mergeStrategies(getQuery("USDT"), getQuery("USDC"))
	if err != nil {
		return nil, err
	}
	for _, s := range merged {
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
