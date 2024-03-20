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
	StrategiesCount int    `env:"STRATEGIES_COUNT" envDefault:"30"`
	RuntimeMinHours int    `env:"RUNTIME_MIN_HOURS" envDefault:"12"`
	RuntimeMaxHours int    `env:"RUNTIME_MAX_HOURS" envDefault:"168"`
	Paper           bool   `env:"PAPER" envDefault:"true"`
	DiscordWebhook  string `env:"DISCORD_WEBHOOK"`
}

var TheConfig = &Config{}

func configure() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
