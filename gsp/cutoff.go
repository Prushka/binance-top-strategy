package gsp

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/persistence"
	"BinanceTopStrategies/utils"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"sort"
	"strconv"
	"time"
)

type sdCountPair struct {
	SymbolDirection string
	Count           int
	MaxMetric       float64
}

func Init() {
	err := persistence.Load(&gridEnv, persistence.GridStatesFileName)
	if err != nil {
		log.Fatalf("Error loading state on grid open: %v", err)
	}
}

func sortBySDCount(filtered Strategies) Strategies {
	filteredBySymbolDirection := make(map[string]Strategies)
	for _, s := range filtered {
		sd := s.Symbol + DirectionMap[s.Direction]
		if _, ok := filteredBySymbolDirection[sd]; !ok {
			filteredBySymbolDirection[sd] = make(Strategies, 0)
		}
		filteredBySymbolDirection[sd] = append(filteredBySymbolDirection[sd], s)
	}
	sdLengths := make([]sdCountPair, 0)
	for sd, s := range filteredBySymbolDirection {
		sdLengths = append(sdLengths, sdCountPair{SymbolDirection: sd, Count: len(s), MaxMetric: s[0].GetMetric()})
	}
	sort.Slice(sdLengths, func(i, j int) bool {
		if sdLengths[i].Count == sdLengths[j].Count {
			return sdLengths[i].MaxMetric > sdLengths[j].MaxMetric
		}
		return sdLengths[i].Count > sdLengths[j].Count
	})
	sortedBySDCount := make(Strategies, 0)
	for _, sd := range sdLengths {
		sortedBySDCount = append(sortedBySDCount, filteredBySymbolDirection[sd.SymbolDirection]...)
	}
	return sortedBySDCount
}

func UpdateTopStrategiesWithRoi() error {
	strategies, err := getTopStrategies(FUTURE, "")
	if err != nil {
		return err
	}
	filtered := make(Strategies, 0)
	for c, s := range strategies.Strategies {
		id := s.SID
		roi, err := RoisCache.Get(fmt.Sprintf("%d-%d", id, s.UserID))
		if err != nil {
			return err
		}
		s.Rois = roi
		s.roi, _ = strconv.ParseFloat(s.Roi, 64)
		s.roi /= 100

		lower, _ := strconv.ParseFloat(s.StrategyParams.LowerLimit, 64)
		upper, _ := strconv.ParseFloat(s.StrategyParams.UpperLimit, 64)
		s.priceDifference = upper/lower - 1
		GStrats[s.SID] = s
		if len(s.Rois) > 1 {
			s.roi = s.Rois[0].Roi
			s.lastDayRoiChange = GetRoiChange(s.Rois, 24*time.Hour)
			s.last3HrRoiChange = GetRoiChange(s.Rois, 3*time.Hour)
			s.last2HrRoiChange = GetRoiChange(s.Rois, 2*time.Hour)
			s.lastHrRoiChange = GetRoiChange(s.Rois, 1*time.Hour)
			s.lastDayRoiPerHr = GetRoiPerHr(s.Rois, 24*time.Hour)
			s.last15HrRoiPerHr = GetRoiPerHr(s.Rois, 15*time.Hour)
			s.last12HrRoiPerHr = GetRoiPerHr(s.Rois, 12*time.Hour)
			s.last9HrRoiPerHr = GetRoiPerHr(s.Rois, 9*time.Hour)
			s.last6HrRoiPerHr = GetRoiPerHr(s.Rois, 6*time.Hour)
			s.lastNHrNoDip = NoDip(s.Rois, time.Duration(config.TheConfig.LastNHoursNoDips)*time.Hour)
			s.lastNHrAllPositive = AllPositive(s.Rois, time.Duration(config.TheConfig.LastNHoursAllPositive)*time.Hour)
			s.roiPerHour = (s.roi - s.Rois[len(s.Rois)-1].Roi) / float64(s.RunningTime/3600)
			prefix := ""
			if s.lastDayRoiChange > 0.1 &&
				s.last3HrRoiChange > 0.03 &&
				s.lastHrRoiChange > 0.02 &&
				s.last2HrRoiChange > s.lastHrRoiChange &&
				s.lastDayRoiPerHr > 0.01 &&
				s.last12HrRoiPerHr > 0.014 &&
				s.priceDifference > 0.05 &&
				//((s.Direction == SHORT && s.StrategyParams.StopUpperLimit != nil) || (s.Direction == LONG && s.StrategyParams.StopLowerLimit != nil) || (s.Direction == NEUTRAL && s.StrategyParams.StopLowerLimit != nil && s.StrategyParams.StopUpperLimit != nil)) &&
				s.lastNHrAllPositive {
				// TODO: price difference can shrink with trailing, e.g., 5.xx% -> 4.xx%
				if GGrids.ExistingSIDs.Contains(s.SID) {
					grid := GGrids.GetGridBySID(s.SID)
					if grid.Tracking.HighestRoi < 0 && grid.GetRunTime() > 1*time.Hour {
						discord.Infof(Display(s, grid, "Grid has negative ROI", 0, 0))
						blacklist.AddSID(s.SID, utils.TillNextRefresh(), "Grid has negative ROI")
						continue
					}
				}
				filtered = append(filtered, s)
				prefix += "Open"
			}
			log.Info(prefix + Display(s, nil, "Found", c+1, len(strategies.StrategiesById)))
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		I := filtered[i].GetMetric()
		J := filtered[j].GetMetric()
		return I > J
	})
	Bundle = &StrategiesBundle{Raw: strategies,
		FilteredSortedBySD:     sortBySDCount(filtered).toTrackedStrategies(),
		FilteredSortedByMetric: filtered.toTrackedStrategies(),
		SDCountPairSpecific:    make(map[string]int)}
	discord.Infof("### Strategies")
	discord.Infof("* Raw: " + Bundle.Raw.String())
	discord.Infof("* Open: " + GetPool().String())
	filteredSymbols := mapset.NewSetFromMapKeys(GetPool().SymbolCount)
	var gridSymbols mapset.Set[string]
	if GGrids.ExistingSymbols.Cardinality() > 0 {
		gridSymbols = GGrids.ExistingSymbols
	} else {
		gridSymbols, err = getGridSymbols()
		if err != nil {
			return err
		}
	}
	err = updateSDCountPairSpecific(filteredSymbols.Union(gridSymbols))
	if err != nil {
		return err
	}
	return nil
}

func updateSDCountPairSpecific(symbols mapset.Set[string]) error {
	log.Infof("Strategy with Symbol Specifics: %v", symbols)
	for _, symbol := range symbols.ToSlice() {
		strategies, err := getTopStrategies(FUTURE, symbol)
		if err != nil {
			return err
		}
		for sd, count := range strategies.SymbolDirectionCount {
			if _, ok := Bundle.SDCountPairSpecific[sd]; !ok {
				Bundle.SDCountPairSpecific[sd] = count
			}
		}
	}
	return nil
}
