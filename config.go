package main

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiKey                         string  `env:"API_KEY"`
	SecretKey                      string  `env:"SECRET_KEY"`
	CSRFToken                      string  `env:"CSRF"`
	COOKIE                         string  `env:"COOKIE"`
	StrategiesCount                int     `env:"STRATEGIES_COUNT" envDefault:"70"`
	RuntimeMinHours                int     `env:"RUNTIME_MIN_HOURS" envDefault:"3"`
	RuntimeMaxHours                int     `env:"RUNTIME_MAX_HOURS" envDefault:"200"`
	Paper                          bool    `env:"PAPER" envDefault:"true"`
	DiscordWebhook                 string  `env:"DISCORD_WEBHOOK"`
	DiscordName                    string  `env:"DISCORD_NAME" envDefault:"BTS"`
	TickEveryMinutes               int     `env:"TICK_EVERY_MINUTES" envDefault:"5"`
	MaxChunks                      int     `env:"MAX_CHUNKS" envDefault:"6"`
	Mode                           string  `env:"MODE" envDefault:"trading"`
	Leverage                       int     `env:"LEVERAGE" envDefault:"20"`
	KeepTopNStrategiesOfSameSymbol int     `env:"KEEP_TOP_N_STRATEGIES_OF_SAME_SYMBOL" envDefault:"99"`
	Last3HrWeight                  float64 `env:"LAST_3_HR_WEIGHT" envDefault:"1"`
	Last2HrWeight                  float64 `env:"LAST_2_HR_WEIGHT" envDefault:"0.8"`
	LastHrWeight                   float64 `env:"LAST_1_HR_WEIGHT" envDefault:"0.6"`
}

var TheConfig = &Config{}

func configure() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}
