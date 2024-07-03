package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"
)

func Scrape(sType int, sString string) error {
	t := time.Now()
	discord.Infof("### Strategies %s: %v", sString, time.Now().Format("2006-01-02 15:04:05"))
	strategies, err := getTopStrategies(sType)
	if err != nil {
		discord.Errorf("Strategies %s: %v", sString, err)
		return err
	}
	discord.Infof("Fetched strategies: %d", len(strategies))
	err = addToRankingStore(strategies)
	if err != nil {
		discord.Errorf("Strategies %s: %v", sString, err)
		return err
	}
	discord.Infof("*Strategies %s run took: %v*", sString, time.Since(t))
	return nil
}

func IsGridOriStrategyRunning(grid *Grid) (*Strategy, error) {
	oriSID := grid.SID
	var oriUid int
	err := sql.GetDB().ScanOne(&oriUid, `SELECT user_id FROM bts.strategy WHERE strategy_id = $1`,
		oriSID)
	if err == nil {
		rois, err := RoisCache.Get(fmt.Sprintf("%d-%d", oriSID, oriUid))
		if err != nil {
			return nil, err
		}
		if !rois.isRunning() {
			return nil, nil
		}
	}
	discoverStrategy, err := DiscoverRootStrategy(oriSID, grid.Symbol, DirectionSMap[grid.Direction], grid.GetRunTime())
	if err != nil {
		return nil, err
	}
	if discoverStrategy != nil {
		log.Debugf("Strategy %d is running", grid.SID)
		return discoverStrategy, nil
	}
	return nil, nil
}
