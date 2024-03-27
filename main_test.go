package main

import (
	log "github.com/sirupsen/logrus"
	"testing"
)

func TestT(t *testing.T) {
	configure()
	log.Infof("Public IP: %s", getPublicIP())
	DiscordService()
	sdk()
	tick()
}
