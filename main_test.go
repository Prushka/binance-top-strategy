package main

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/utils"
	log "github.com/sirupsen/logrus"
	"testing"
)

func TestT(t *testing.T) {
	config.Init()
	log.Infof("Public IP: %s", utils.GetPublicIP())
	discord.Init()
	sdk.Init()
	updateOpenGrids(true)
	updateOpenGrids(true)
	updateOpenGrids(true)
}
