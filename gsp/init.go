package gsp

import (
	"BinanceTopStrategies/persistence"
	log "github.com/sirupsen/logrus"
)

func Init() {
	err := persistence.Load(&gridEnv, persistence.GridStatesFileName)
	if err != nil {
		log.Fatalf("Error loading state on grid open: %v", err)
	}
	err = persistence.Load(&ForRemoval, persistence.MarkForRemovalFileName)
	if err != nil {
		log.Fatalf("Error loading mark for removal: %v", err)
	}
}
