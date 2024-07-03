package gsp

import (
	mapset "github.com/deckarep/golang-set/v2"
)

var SessionCancelledGIDs = mapset.NewSet[int]()

type Grids []*Grid

var openGrids Grids

type TotalProfit struct {
	Input float64
	Pnl   float64
	Roi   float64
}

func GetOpenGrids() Grids {
	return openGrids
}

func (grids Grids) AllSymbols() mapset.Set[string] {
	symbols := mapset.NewSet[string]()
	for _, g := range grids {
		symbols.Add(g.Symbol)
	}
	return symbols
}

func (grids Grids) AllSIDs() mapset.Set[int] {
	sids := mapset.NewSet[int]()
	for _, g := range grids {
		sids.Add(g.SID)
	}
	return sids
}

func (grids Grids) GetChunks(quote string) int {
	chunks := 0
	for _, g := range grids {
		if g.IsQuote(quote) {
			chunks++
		}
	}
	return chunks
}

func (grids Grids) GetLSN() (int, int, int) {
	longs := 0
	shorts := 0
	neutrals := 0
	for _, g := range grids {
		switch g.Direction {
		case "LONG":
			longs++
		case "SHORT":
			shorts++
		case "NEUTRAL":
			neutrals++
		}
	}
	return longs, shorts, neutrals
}

func (grids Grids) FindGID(gid int) *Grid {
	for _, g := range grids {
		if g.GID == gid {
			return g
		}
	}
	return nil
}

func (grids Grids) FindSID(sid int) *Grid {
	for _, g := range grids {
		if g.SID == sid {
			return g
		}
	}
	return nil
}

func (grids Grids) TotalProfits() (TotalProfit, TotalProfit) {
	return grids.TotalProfitByQuote("USDT"), grids.TotalProfitByQuote("USDC")
}

func (grids Grids) TotalProfitByQuote(quote string) TotalProfit {
	tp := TotalProfit{}
	for _, g := range grids {
		if g.IsQuote(quote) {
			tp.Input += g.InitialValue
			tp.Pnl += g.LastPnl
		}
	}
	tp.Roi = tp.Pnl / tp.Input
	return tp
}

type Strategies []*Strategy

var pool Strategies

func GetPool() Strategies {
	return pool
}

func SetPool(strategies Strategies) {
	pool = strategies
}

func (by Strategies) ByUID() map[int]Strategies {
	byUID := make(map[int]Strategies)
	for _, s := range by {
		if _, ok := byUID[s.UserID]; !ok {
			byUID[s.UserID] = make(Strategies, 0)
		}
		byUID[s.UserID] = append(byUID[s.UserID], s)
	}
	return byUID
}

func (by Strategies) FindSID(sid int) *Strategy {
	for _, s := range by {
		if s.SID == sid {
			return s
		}
	}
	return nil
}

func (by Strategies) AllSymbols() mapset.Set[string] {
	symbols := mapset.NewSet[string]()
	for _, s := range by {
		symbols.Add(s.Symbol)
	}
	return symbols
}

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

func (by Strategies) Users() int {
	users := mapset.NewSet[int]()
	for _, s := range by {
		users.Add(s.UserID)
	}
	return users.Cardinality()
}
