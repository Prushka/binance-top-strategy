package gsp

import mapset "github.com/deckarep/golang-set/v2"

var GlobalStrategies = make(map[int]*Strategy)
var GlobalGrids = &TrackedGrids{
	Shorts:          mapset.NewSet[int](),
	Longs:           mapset.NewSet[int](),
	Neutrals:        mapset.NewSet[int](),
	ExistingSIDs:    mapset.NewSet[int](),
	ExistingSymbols: mapset.NewSet[string](),
	GridsByGid:      make(map[int]*Grid),
}
var Bundle *StrategiesBundle

type StrategiesBundle struct {
	Raw                    *TrackedStrategies
	FilteredSortedBySD     *TrackedStrategies
	FilteredSortedByMetric *TrackedStrategies
	SDCountPairSpecific    map[string]int
}

const (
	SDRaw          = "SDRaw"
	SDFiltered     = "SDFiltered"
	SDPairSpecific = "SDPairSpecific"
)
