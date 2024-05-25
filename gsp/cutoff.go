package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sql"
	"fmt"
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
	strategies, err := getTopStrategies("")
	if err != nil {
		return err
	}
	discord.Infof("Fetched strategies: %d", len(strategies.Strategies))
	for _, s := range strategies.Strategies {
		err := s.addToRankingStore()
		if err != nil {
			return err
		}
	}
	return nil
}

func IsGridOriStrategyRunning(grid *Grid) (bool, error) {
	oriSID := grid.SID
	var oriUid int
	sql.GetDB().ScanOne(&oriUid, `SELECT user_id FROM bts.strategy WHERE strategy_id = $1`,
		oriSID)
	rois, err := RoisCache.Get(fmt.Sprintf("%d-%d", oriSID, oriUid))
	if err != nil {
		return false, err
	}
	if !rois.isRunning() {
		return false, nil
	}
	discoverStrategy, err := DiscoverGridRootStrategy(oriSID, grid.Symbol, DirectionSMap[grid.Direction], grid.GetRunTime())
	if err != nil {
		return false, err
	}
	if discoverStrategy != nil {
		log.Infof("Strategy %d is running", grid.SID)
		return true, nil
	}
	return false, nil
}

func UpdateTopStrategiesWithRoi(strategies Strategies) error {
	for _, s := range strategies {
		id := s.SID
		rois, err := RoisCache.Get(fmt.Sprintf("%d-%d", id, s.UserID))
		if err != nil {
			return err
		}
		s.Rois = rois
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
		}
	}
	Bundle = &StrategiesBundle{Raw: strategies.toTrackedStrategies()}
	return nil
}
