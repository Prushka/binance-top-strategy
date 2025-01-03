package notional

import (
	"BinanceTopStrategies/cache"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/utils"
	"math"
	"sort"
	"time"
)

var bracketsCache = cache.CreateCache[*response](20*time.Minute,
	func() (*response, error) {
		return getBrackets()
	},
)

type bracket struct {
	BracketSeq                   int     `json:"bracketSeq"`
	BracketNotionalFloor         int     `json:"bracketNotionalFloor"`
	BracketNotionalCap           int     `json:"bracketNotionalCap"`
	BracketMaintenanceMarginRate float64 `json:"bracketMaintenanceMarginRate"`
	CumFastMaintenanceAmount     float64 `json:"cumFastMaintenanceAmount"`
	MinOpenPosLeverage           int     `json:"minOpenPosLeverage"`
	MaxOpenPosLeverage           int     `json:"maxOpenPosLeverage"`
}

type symbol struct {
	Symbol        string     `json:"symbol"`
	UpdateTime    int64      `json:"updateTime"`
	NotionalLimit int        `json:"notionalLimit"`
	RiskBrackets  []*bracket `json:"riskBrackets"`
}

type response struct {
	Symbols struct {
		Brackets []*symbol `json:"brackets"`
	} `json:"data"`
	SymbolMap map[string]*symbol
	request.BinanceBaseResponse
}

func GetLeverage(symbol string, initialAsset float64) int {
	brackets, err := bracketsCache.Get()
	if err != nil {
		return -1
	}
	s, ok := brackets.SymbolMap[symbol]
	if !ok {
		return -1
	}
	for _, b := range s.RiskBrackets {
		if float64(b.MinOpenPosLeverage)*initialAsset <= float64(b.BracketNotionalCap) { // fits in this bracket
			leverage := int(math.Min(float64(b.BracketNotionalCap)/initialAsset, float64(b.MaxOpenPosLeverage)))
			return leverage
		}
	}
	return -1
}

func MaxLeverage(symbol string) int {
	brackets, err := bracketsCache.Get()
	if err != nil {
		return -1
	}
	s, ok := brackets.SymbolMap[symbol]
	if !ok {
		return -1
	}
	m := 0
	for _, b := range s.RiskBrackets {
		m = utils.IntMax(m, b.MaxOpenPosLeverage)
	}
	return m
}

func getBrackets() (*response, error) {
	resp, _, err := request.Request("https://www.binance.com/bapi/futures/v1/friendly/future/common/brackets",
		"{}", &response{})
	if err != nil {
		return nil, err
	}
	for _, s := range resp.Symbols.Brackets {
		sort.Slice(s.RiskBrackets, func(i, j int) bool {
			return s.RiskBrackets[i].BracketSeq < s.RiskBrackets[j].BracketSeq
		})
	}
	resp.SymbolMap = make(map[string]*symbol)
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
