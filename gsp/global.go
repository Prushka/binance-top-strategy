package gsp

import mapset "github.com/deckarep/golang-set/v2"

var SessionCancelledGIDs = mapset.NewSet[int]()

var GGrids = &TrackedGrids{
	Shorts:           mapset.NewSet[int](),
	Longs:            mapset.NewSet[int](),
	Neutrals:         mapset.NewSet[int](),
	ExistingSIDs:     mapset.NewSet[int](),
	ExistingSymbols:  mapset.NewSet[string](),
	GridsByGid:       make(map[int]*Grid),
	TotalGridInitial: map[string]float64{},
	TotalGridPnl:     map[string]float64{},
}
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
