package main

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiKey                         string `env:"API_KEY"`
	SecretKey                      string `env:"SECRET_KEY"`
	CSRFToken                      string `env:"CSRF"`
	COOKIE                         string `env:"COOKIE"`
	StrategiesCount                int    `env:"STRATEGIES_COUNT" envDefault:"45"`
	RuntimeMinHours                int    `env:"RUNTIME_MIN_HOURS" envDefault:"3"`
	RuntimeMaxHours                int    `env:"RUNTIME_MAX_HOURS" envDefault:"168"`
	Paper                          bool   `env:"PAPER" envDefault:"true"`
	DiscordWebhook                 string `env:"DISCORD_WEBHOOK"`
	DiscordName                    string `env:"DISCORD_NAME" envDefault:"BTS"`
	TickEveryMinutes               int    `env:"TICK_EVERY_MINUTES" envDefault:"5"`
	MaxChunks                      int    `env:"MAX_CHUNKS" envDefault:"6"`
	Mode                           string `env:"MODE" envDefault:"trading"`
	Leverage                       int    `env:"LEVERAGE" envDefault:"20"`
	KeepTopNStrategiesOfSameSymbol int    `env:"KEEP_TOP_N_STRATEGIES_OF_SAME_SYMBOL" envDefault:"3"`
}

var TheConfig = &Config{}

func configure() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
