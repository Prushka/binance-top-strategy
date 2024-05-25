package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sort"
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
		err := s.PopulateRois()
		if err != nil {
			return err
		}
	}
	Bundle = &StrategiesBundle{Raw: strategies.toTrackedStrategies()}
	return nil
}
