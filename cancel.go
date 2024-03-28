package main

import (
	"fmt"
	"strings"
)

type GridToCancel struct {
	MaxLoss   float64
	Reasons   []string
	Grid      *Grid
	Cancelled bool
}

type GridsToCancel map[int]*GridToCancel

func (tc GridToCancel) CanCancel() bool {
	return tc.Grid.lastRoi >= tc.MaxLoss
}

func (tc GridToCancel) Cancel() error {
	grid := tc.Grid
	if tc.CanCancel() {
		err := closeGrid(grid.GID)
		if err != nil {
			return err
		}
		tc.Cancelled = true
		DiscordWebhookS(display(nil, grid, "**Cancelled**", 0, 0), ActionWebhook, DefaultWebhook)
	} else {
		Discordf(display(nil, grid, "**Skip Cancel**", 0, 0))
	}
	for _, reason := range tc.Reasons {
		DiscordWebhookS(" * "+reason, ActionWebhook, DefaultWebhook)
	}
	return nil
}

func (g GridsToCancel) CancelAll() {
	for _, tc := range g {
		err := tc.Cancel()
		if err != nil {
			Discordf("Error cancelling grid: %v", err)
		}
	}
}

func (g GridsToCancel) AddGridToCancel(grid *Grid, maxLoss float64, reason string) {
	tc, ok := g[grid.GID]
	if !ok {
		tc = &GridToCancel{
			MaxLoss: maxLoss,
			Grid:    grid,
		}
		g[grid.GID] = tc
	} else if maxLoss < tc.MaxLoss {
		tc.MaxLoss = maxLoss
	}
	tc.Reasons = append(tc.Reasons, reason)
}

func (g GridsToCancel) Empty() bool {
	return len(g) == 0
}

func (g GridsToCancel) hasCancelled() bool {
	for _, tc := range g {
		if tc.Cancelled {
			return true
		}
	}
	return false
}

func (g GridsToCancel) String() string {
	var s []string
	for _, tc := range g {
		s = append(s, fmt.Sprintf("%d: %s, %.2f%%", tc.Grid.GID, tc.Grid.Symbol, tc.MaxLoss*100))
	}
	return strings.Join(s, " | ")
}
