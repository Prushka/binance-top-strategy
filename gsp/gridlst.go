package gsp

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/persistence"
	"BinanceTopStrategies/request"
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
	return res, nil
}

func getGridSymbols() (mapset.Set[string], error) {
	res, err := getOpenGrids()
	if err != nil {
		return nil, err
	}
	symbols := mapset.NewSet[string]()
	for _, grid := range res.Grids {
		symbols.Add(grid.Symbol)
	}
	return symbols, nil
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
	for gid := range gridEnv {
		if !currentIds.Contains(gid) {
			delete(gridEnv, gid)
		}
	}
	err = persistence.Save(gridEnv, persistence.GridStatesFileName)
	if err != nil {
		discord.Errorf("**Error saving grid env**: %v", err)
	}
	discord.Infof("Open Pairs: %v, Initial: %.2f, TotalPnL: %.2f, C: %.2f, L/S/N: %d/%d/%d",
		GGrids.ExistingSymbols, GGrids.TotalGridInitial, GGrids.TotalGridPnl, GGrids.TotalGridPnl+GGrids.TotalGridInitial,
		GGrids.Longs.Cardinality(), GGrids.Shorts.Cardinality(), GGrids.Neutrals.Cardinality())
	return nil
}
