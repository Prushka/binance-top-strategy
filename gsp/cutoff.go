package gsp

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/utils"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"sort"
	"time"
)

type sdCountPair struct {
	SymbolDirection string
	Count           int
	MaxMetric       float64
}

func sortBySDCount(filtered Strategies) Strategies {
	filteredBySymbolDirection := make(map[string]Strategies)
	for _, s := range filtered {
		sd := s.SD()
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

func Scrape() error {
	strategies, err := getTopStrategies(FUTURE, "")
	if err != nil {
		return err
	}
	discord.Infof("* New: " + strategies.String())
	for _, s := range strategies.Strategies {
		err := s.addToRankingStore()
		if err != nil {
			return err
		}
	}
	return nil
}

func UpdateTopStrategiesWithRoi() error {
	strategies, err := getTopStrategies(FUTURE, "")
	if err != nil {
		return err
	}
	filtered := make(Strategies, 0)

	discord.Infof("* New: " + strategies.String())
	for c, s := range strategies.Strategies {
		err := s.addToRankingStore()
		if err != nil {
			return err
		}
		continue
		id := s.SID
		rois, err := RoisCache.Get(fmt.Sprintf("%d-%d", id, s.UserID))
		if err != nil {
			return err
		}
		s.Rois = rois
		s.PriceDifference = s.StrategyParams.UpperLimit/s.StrategyParams.LowerLimit - 1
		GStrats[s.SID] = s
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
			prefix := ""
			reasons := make([]string, 0)
			picked := true
			if s.LastDayRoiChange <= 0.1 {
				reasons = append(reasons, "Last Day ROI <= 0.1")
				picked = false
			}
			if s.Last3HrRoiChange <= 0.03 {
				reasons = append(reasons, "Last 3Hr ROI <= 0.03")
				picked = false
			}
			if s.LastHrRoiChange <= 0.016 {
				reasons = append(reasons, "Last Hr ROI <= 0.016")
				picked = false
			}
			if s.LastDayRoiPerHr <= 0.01 {
				reasons = append(reasons, "Last Day ROI/Hr <= 0.01")
				picked = false
			}
			if s.Last12HrRoiPerHr <= 0.014 {
				reasons = append(reasons, "Last 12Hr ROI/Hr <= 0.014")
				picked = false
			}
			if !s.Rois.AllPositive(3*time.Hour, 0.01) {
				reasons = append(reasons, "Not all positive in last 3Hr (1% cutoff)")
				picked = false
			}
			if s.PriceDifference <= 0.045 {
				reasons = append(reasons, "Price difference <= 0.045")
				picked = false
			}
			if !s.LastNHrNoDip {
				reasons = append(reasons, "Last N Hr has dip")
				picked = false
			}
			if !s.LastNHrAllPositive {
				reasons = append(reasons, "Last N Hr not all positive")
				picked = false
			}
			if GGrids.ExistingSIDs.Contains(s.SID) {
				grid := GGrids.GetGridBySID(s.SID)
				_, localHighest := grid.GetTracking().GetLocalWithin(1 * time.Hour)
				if grid.GetRunTime() > 1*time.Hour && localHighest < 0.01 {
					reason := "Grid ROI < 0.01"
					blacklist.AddSID(s.SID, utils.TillNextRefresh(), reason)
					reasons = append(reasons, reason)
					picked = false
				}
			}
			if picked {
				filtered = append(filtered, s)
				prefix += "Open"
			}
			s.ReasonNotPicked = reasons
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
		SDCountPairSpecific:    make(SDCount)}
	//discord.Infof("* Open: " + GetPool().String())
	//filteredSymbols := mapset.NewSetFromMapKeys(GetPool().SymbolCount)
	//var gridSymbols mapset.Set[string]
	//if GGrids.ExistingSymbols.Cardinality() > 0 {
	//	gridSymbols = GGrids.ExistingSymbols
	//} else {
	//	gridSymbols, err = getGridSymbols()
	//	if err != nil {
	//		return err
	//	}
	//}
	//err = updateSDCountPairSpecific(filteredSymbols.Union(gridSymbols))
	//if err != nil {
	//	return err
	//}
	return nil
}

func updateSDCountPairSpecific(symbols mapset.Set[string]) error {
	log.Infof("Strategy with Symbol Specifics: %v", symbols)
	for _, symbol := range symbols.ToSlice() {
		strategies, err := getTopStrategies(FUTURE, symbol)
		if err != nil {
			return err
		}
		for direction, count := range strategies.SymbolDirectionCount[symbol] {
			if _, ok := Bundle.SDCountPairSpecific[symbol]; !ok {
				Bundle.SDCountPairSpecific[symbol] = make(map[string]int)
			}
			Bundle.SDCountPairSpecific[symbol][direction] = count
		}
	}
	return nil
}
