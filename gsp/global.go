package gsp

var GlobalStrategies = make(map[int]*Strategy)
var GlobalGrids = newTrackedGrids()
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
