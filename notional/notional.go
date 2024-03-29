package notional

import (
	"BinanceTopStrategies/cache"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"math"
	"sort"
	"time"
)

var BracketsCache = cache.CreateCache[*Response](20*time.Minute,
	func() (*Response, error) {
		return getBrackets()
	},
)

type Bracket struct {
	BracketSeq                   int     `json:"bracketSeq"`
	BracketNotionalFloor         int     `json:"bracketNotionalFloor"`
	BracketNotionalCap           int     `json:"bracketNotionalCap"`
	BracketMaintenanceMarginRate float64 `json:"bracketMaintenanceMarginRate"`
	CumFastMaintenanceAmount     float64 `json:"cumFastMaintenanceAmount"`
	MinOpenPosLeverage           int     `json:"minOpenPosLeverage"`
	MaxOpenPosLeverage           int     `json:"maxOpenPosLeverage"`
}

type Symbol struct {
	Symbol        string     `json:"symbol"`
	UpdateTime    int64      `json:"updateTime"`
	NotionalLimit int        `json:"notionalLimit"`
	RiskBrackets  []*Bracket `json:"riskBrackets"`
}

type Response struct {
	Symbols struct {
		Brackets []*Symbol `json:"brackets"`
	} `json:"data"`
	SymbolMap map[string]*Symbol
	request.BinanceBaseResponse
}

func GetLeverage(symbol string, initialAsset float64, maxLeverage int) int {
	brackets, err := BracketsCache.Get()
	if err != nil {
		return maxLeverage
	}
	s, ok := brackets.SymbolMap[symbol]
	if !ok {
		return maxLeverage
	}
	for _, b := range s.RiskBrackets {
		if float64(b.MinOpenPosLeverage)*initialAsset <= float64(b.BracketNotionalCap) { // fits in this bracket
			leverage := int(math.Min(float64(b.BracketNotionalCap)/initialAsset, float64(b.MaxOpenPosLeverage)))
			discord.Infof("Notional Leverage: %d, Initial: %f, Max Leverage: %d", leverage, initialAsset, maxLeverage)
			if leverage > maxLeverage {
				return maxLeverage
			}
			return leverage
		}
	}
	return maxLeverage
}

func getBrackets() (*Response, error) {
	resp, _, err := request.Request("https://www.binance.com/bapi/futures/v1/friendly/future/common/brackets",
		"{}", &Response{})
	if err != nil {
		return nil, err
	}
	for _, s := range resp.Symbols.Brackets {
		sort.Slice(s.RiskBrackets, func(i, j int) bool {
			return s.RiskBrackets[i].BracketSeq < s.RiskBrackets[j].BracketSeq
		})
	}
	resp.SymbolMap = make(map[string]*Symbol)
	for _, s := range resp.Symbols.Brackets {
		existing, ok := resp.SymbolMap[s.Symbol]
		if !ok {
			resp.SymbolMap[s.Symbol] = s
		} else {
			if existing.UpdateTime < s.UpdateTime {
				resp.SymbolMap[s.Symbol] = s
			}
		}
	}
	return resp, nil
}
