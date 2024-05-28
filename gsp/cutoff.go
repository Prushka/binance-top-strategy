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
	return addToRankingStore(strategies.Strategies)
}

func IsGridOriStrategyRunning(grid *Grid) (*Strategy, error) {
	oriSID := grid.SID
	var oriUid int
	err := sql.GetDB().ScanOne(&oriUid, `SELECT user_id FROM bts.strategy WHERE strategy_id = $1`,
		oriSID)
	if err != nil {
		return nil, err
	}
	rois, err := RoisCache.Get(fmt.Sprintf("%d-%d", oriSID, oriUid))
	if err != nil {
		return nil, err
	}
	if !rois.isRunning() {
		return nil, nil
	}
	discoverStrategy, err := DiscoverRootStrategy(oriSID, grid.Symbol, DirectionSMap[grid.Direction], grid.GetRunTime())
	if err != nil {
		return nil, err
	}
	if discoverStrategy != nil {
		log.Infof("Strategy %d is running", grid.SID)
		return discoverStrategy, nil
	}
	return nil, nil
}

func AddToPool(strategies Strategies) {
	Bundle = &StrategiesBundle{Raw: strategies.toTrackedStrategies()}
}
