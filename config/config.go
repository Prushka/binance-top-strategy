package config

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
	"time"
)

type Config struct {
	ApiKey                         string `env:"API_KEY"`
	SecretKey                      string `env:"SECRET_KEY"`
	CSRFToken                      string `db:"CSRF"`
	Cookie                         string `db:"COOKIE"`
	CookieTime                     string `db:"COOKIE_TIME"`
	CookieTimeParsed               time.Time
	MarginType                     string    `env:"MARGIN_TYPE" envDefault:"CROSSED"`
	RuntimeMinHours                int       `env:"RUNTIME_MIN_HOURS" envDefault:"3"`
	RuntimeMaxHours                int       `env:"RUNTIME_MAX_HOURS" envDefault:"168"`
	Paper                          bool      `env:"PAPER" envDefault:"true"`
	DiscordWebhook                 string    `env:"DISCORD_WEBHOOK"`
	DiscordWebhookAction           string    `env:"DISCORD_WEBHOOK_ACTION"`
	DiscordWebhookOrder            string    `env:"DISCORD_WEBHOOK_ORDER"`
	DiscordWebhookError            string    `env:"DISCORD_WEBHOOK_ERROR"`
	DiscordWebhookBlacklist        string    `env:"DISCORD_WEBHOOK_BLACKLIST"`
	DiscordName                    string    `env:"DISCORD_NAME" envDefault:"BTS"`
	Reserved                       float64   `env:"RESERVED" envDefault:"0.10"`
	MaxPerChunk                    float64   `env:"MAX_PER_CHUNK" envDefault:"-1"`
	TradingBlockMinutesAfterCancel int       `env:"TRADING_BLOCK_MINUTES_AFTER_CANCEL" envDefault:"3"`
	TakeProfits                    []float64 `env:"TAKE_PROFITS" envDefault:"0.52,0.295,0.235,0.175,0.15,0.1"`
	TakeProfitsMaxLookBackMinutes  []int     `env:"TAKE_PROFITS_MAX_LOOKBACK_MINUTES" envDefault:"10,15,25,40,50,90"`
	TakeProfitsBlockMinutes        []int     `env:"TAKE_PROFITS_BLOCK_MINUTES" envDefault:"40,-1,-1,-1,-1,-1"`
	TickEverySeconds               int       `env:"TICK_EVERY_SECONDS" envDefault:"30"`
	MaxUSDTChunks                  int       `env:"MAX_USDT_CHUNKS" envDefault:"5"`
	MaxUSDCChunks                  int       `env:"MAX_USDC_CHUNKS" envDefault:"2"`
	MaxNeutrals                    int       `env:"MAX_NEUTRALS" envDefault:"6"`
	MinInvestmentPerChunk          float64   `env:"MIN_INVESTMENT_PER_CHUNK" envDefault:"10"`
	Mode                           string    `env:"MODE" envDefault:"trading"`
	PreferredLeverage              int       `env:"PREFERRED_LEVERAGE" envDefault:"20"`
	MaxLeverage                    int       `env:"MAX_LEVERAGE" envDefault:"60"`
	StopLossMarkForRemoval         []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL" envDefault:"-0.4,-0.7"`
	StopLossMarkForRemovalSLAt     []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL_SL_AT" envDefault:"0,0"`
	NeutralRangeDiff               float64   `env:"NEUTRAL_RANGE_DIFF" envDefault:"0.25"`
	ShortRangeDiff                 float64   `env:"SHORT_RANGE_DIFF" envDefault:"0.2"`
	LongRangeDiff                  float64   `env:"LONG_RANGE_DIFF" envDefault:"0.2"`
	TriggerRangeDiff               float64   `env:"TRIGGER_RANGE_DIFF" envDefault:"0.04"`
	PGUrl                          string    `env:"PGURL" envDefault:"postgresql://postgres:password@localhost:5432"`
}

var TheConfig = &Config{}

func Init() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}

func GetNormalized(pf float64, leverage int) float64 {
	return (pf / 20) * float64(leverage)
}
