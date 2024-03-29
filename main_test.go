package main

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/gsp"
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
	gsp.updateOpenGrids(true)
	gsp.updateOpenGrids(true)
	gsp.updateOpenGrids(true)
}
