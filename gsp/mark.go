package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/persistence"
)

type MarkForRemoval struct {
	Grids map[int]float64
}

var ForRemoval = MarkForRemoval{
	Grids: make(map[int]float64),
}

func GridMarkForRemoval(gid int, maxLoss float64) {
	ForRemoval.Grids[gid] = maxLoss
	err := persistence.Save(ForRemoval, persistence.MarkForRemovalFileName)
	if err != nil {
		discord.Errorf("Error saving mark for removal: %v", err)
	}
}

func GetMaxLoss(gid int) *float64 {
	if v, ok := ForRemoval.Grids[gid]; ok {
		return &v
	}
	return nil
}
