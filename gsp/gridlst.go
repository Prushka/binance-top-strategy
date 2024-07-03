package gsp

import (
	"BinanceTopStrategies/blacklist"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sql"
	"sort"
	"time"
)

type openGridResponse struct {
	Grids Grids `json:"data"`
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

func UpdateOpenGrids() error {
	res, err := getOpenGrids()
	if err != nil {
		return err
	}
	for _, grid := range res.Grids {
		grid.sanitize()
	}
	for _, g := range openGrids { // previous grids
		if res.Grids.FindGID(g.GID) == nil {
			if !SessionCancelledGIDs.Contains(g.GID) {
				discord.Actionf(Display(nil, g,
					"**Gone**",
					0, 0))
			}
			blacklist.BlockTrading(time.Duration(config.TheConfig.TradingBlockMinutesAfterCancel)*time.Minute, "Grid Gone")
		}
	}
	openGrids = res.Grids
	sort.Slice(openGrids, func(i, j int) bool {
		return openGrids[i].GID < openGrids[j].GID
	})
	USDT, USDC := openGrids.TotalProfits()
	long, short, neutral := openGrids.GetLSN()
	discord.Infof("USDT[Input: %.2f, PnL: %.2f], USDC[Input: %.2f, PnL: %.2f], L/S/N: %d/%d/%d",
		USDT.Input, USDT.Pnl, USDC.Input, USDC.Pnl, long, short, neutral)
	return nil
}
