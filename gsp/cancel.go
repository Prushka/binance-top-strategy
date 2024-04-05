package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"strings"
)

type gridToCancel struct {
	MaxLoss   float64
	Reasons   []string
	Grid      *Grid
	Cancelled bool
}

type GridsToCancel map[int]*gridToCancel

func (tc *gridToCancel) canCancel() bool {
	return tc.Grid.LastRoi >= tc.MaxLoss
}

func closeGrid(strategyId int) error {
	if config.TheConfig.Paper {
		log.Infof("Paper mode, not closing grid")
		return nil
	}
	url := "https://www.binance.com/bapi/futures/v1/private/future/grid/close-grid"
	payload := map[string]interface{}{
		"strategyId": strategyId,
	}
	_, _, err := request.PrivateRequest(url, "POST", payload, &request.BinanceBaseResponse{})
	return err
}

func (tc *gridToCancel) Cancel() error {
	grid := tc.Grid
	webhooks := []int{discord.DefaultWebhook}
	if tc.canCancel() {
		err := closeGrid(grid.GID)
		if err != nil {
			return err
		}
		tc.Cancelled = true
		discord.Actionf(Display(nil, grid, "**Cancelled**", 0, 0))
		webhooks = append(webhooks, discord.ActionWebhook)
	} else {
		discord.Infof(Display(nil, grid, "**Skip Cancel**", 0, 0))
	}
	for _, reason := range tc.Reasons {
		discord.Webhooks(" * "+reason, webhooks...)
	}
	return nil
}

func (g GridsToCancel) CancelAll() {
	for _, tc := range g {
		err := tc.Cancel()
		if err != nil {
			discord.Infof("Error cancelling grid: %v", err)
		}
	}
}

func (g GridsToCancel) AddGridToCancel(grid *Grid, maxLoss float64, reason string) {
	tc, ok := g[grid.GID]
	if !ok {
		tc = &gridToCancel{
			MaxLoss: maxLoss,
			Grid:    grid,
		}
		g[grid.GID] = tc
	} else if maxLoss < tc.MaxLoss {
		tc.MaxLoss = maxLoss
	}
	tc.Reasons = append(tc.Reasons, reason)
}

func (g GridsToCancel) IsEmpty() bool {
	return len(g) == 0
}

func (g GridsToCancel) HasCancelled() bool {
	for _, tc := range g {
		if tc.Cancelled {
			return true
		}
	}
	return false
}

func (g GridsToCancel) CancelledGIDs() mapset.Set[int] {
	gids := mapset.NewSet[int]()
	for _, tc := range g {
		if tc.Cancelled {
			gids.Add(tc.Grid.GID)
		}
	}
	return gids
}

func (g GridsToCancel) String() string {
	var s []string
	for _, tc := range g {
		s = append(s, fmt.Sprintf("%d: %s, %.2f%%", tc.Grid.GID, tc.Grid.Symbol, tc.MaxLoss*100))
	}
	return strings.Join(s, " | ")
}
