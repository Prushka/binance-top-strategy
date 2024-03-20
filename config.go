package main

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiKey          string `env:"API_KEY"`
	SecretKey       string `env:"SECRET_KEY"`
	CSRFToken       string `env:"CSRF"`
	COOKIE          string `env:"COOKIE"`
	StrategiesCount int    `env:"STRATEGIES_COUNT" envDefault:"25"`
}

var TheConfig = &Config{}

func configure() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
