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
var Bundle *StrategiesBundle

type StrategiesBundle struct {
	Raw *TrackedStrategies
}

func GetPool() *TrackedStrategies {
	return Bundle.Raw
}
