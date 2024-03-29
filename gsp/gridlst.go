package gsp

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"fmt"
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
		GlobalGrids.add(grid, trackContinuous)
		currentIds.Add(grid.GID)
	}
	for _, g := range GlobalGrids.GridsByGid {
		if !currentIds.Contains(g.GID) {
			GlobalGrids.remove(g.GID)
			discord.Info(Display(nil, g,
				fmt.Sprintf("**Gone - Block for %d Minutes**", config.TheConfig.TradingBlockMinutesAfterCancel),
				0, 0), discord.ActionWebhook, discord.DefaultWebhook)
			blacklist.BlockTrading(time.Duration(config.TheConfig.TradingBlockMinutesAfterCancel)*time.Minute, "Grid Gone")
		}
	}
	discord.Infof("Open Pairs: %v, Open Ids: %v, Initial: %f, TotalPnL: %f, C: %f, L/S/N: %d/%d/%d",
		GlobalGrids.ExistingSymbols, GlobalGrids.ExistingSIDs, GlobalGrids.TotalGridInitial, GlobalGrids.TotalGridPnl, GlobalGrids.TotalGridPnl+GlobalGrids.TotalGridInitial,
		GlobalGrids.Longs.Cardinality(), GlobalGrids.Shorts.Cardinality(), GlobalGrids.Neutrals.Cardinality())
	return nil
}
