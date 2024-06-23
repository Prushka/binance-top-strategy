package gsp

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sql"
	mapset "github.com/deckarep/golang-set/v2"
	"time"
)

type openGridResponse struct {
	Grids []*Grid `json:"data"`
	request.BinanceBaseResponse
}

func getOpenGrids() (*openGridResponse, error) {
	url := "https://www.binance.com/bapi/futures/v2/private/future/grid/query-open-grids"
	res, _, err := request.PrivateRequest(url, "POST", nil, &openGridResponse{})
	if err != nil {
		return nil, err
	}
	for _, grid := range res.Grids {
		id := 0
		err = sql.GetDB().ScanOne(&id, "SELECT strategy_id FROM bts.grid_strategy WHERE grid_id=$1", grid.GID)
		if err != nil {
			discord.Errorf("Error getting strategy id for grid %d: %v", grid.GID, err)
			return nil, err
		}
		grid.SID = id
	}
	return res, nil
}

func UpdateOpenGrids(trackContinuous bool) error {
	res, err := getOpenGrids()
	if err != nil {
		return err
	}
	currentIds := mapset.NewSet[int]()
	for _, grid := range res.Grids {
		GGrids.add(grid, trackContinuous)
		currentIds.Add(grid.GID)
	}
	for _, g := range GGrids.GridsByGid {
		if !currentIds.Contains(g.GID) {
			if !SessionCancelledGIDs.Contains(g.GID) {
				discord.Actionf(Display(nil, g,
					"**Gone**",
					0, 0))
			}
			blacklist.BlockTrading(time.Duration(config.TheConfig.TradingBlockMinutesAfterCancel)*time.Minute, "Grid Gone")
			GGrids.remove(g.GID)
		}
	}
	for gid := range TheGridEnv {
		if !currentIds.Contains(gid) {
			delete(TheGridEnv, gid)
		}
	}
	discord.Infof("Open Pairs: %v, USDT[Input: %.2f, PnL: %.2f], USDC[Input: %.2f, PnL: %.2f], L/S/N: %d/%d/%d",
		GGrids.ExistingSymbols, GGrids.TotalGridInitial["USDT"], GGrids.TotalGridPnl["USDT"], GGrids.TotalGridInitial["USDC"], GGrids.TotalGridPnl["USDC"],
		GGrids.Longs.Cardinality(), GGrids.Shorts.Cardinality(), GGrids.Neutrals.Cardinality())
	return nil
}
