package config

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
	"time"
)

type Config struct {
	Redis                          string `env:"REDIS" envDefault:"localhost:6379"`
	RedisPassword                  string `env:"REDIS_PASSWORD" envDefault:""`
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
	DataFolder                     string    `env:"DATA_FOLDER" envDefault:"./data"`
	ShiftMinutesAfterHour          int       `env:"SHIFT_MINUTES_AFTER_HOUR" envDefault:"0"`
	LastNHoursNoDips               int       `env:"LAST_N_HOURS_NO_DIPS" envDefault:"6"`
	LastNHoursAllPositive          int       `env:"LAST_N_HOURS_NO_DIPS" envDefault:"6"`
	Reserved                       float64   `env:"RESERVED" envDefault:"0.15"`
	MaxPerChunk                    float64   `env:"MAX_PER_CHUNK" envDefault:"-1"`
	TradingBlockMinutesAfterCancel int       `env:"TRADING_BLOCK_MINUTES_AFTER_CANCEL" envDefault:"3"`
	TakeProfits                    []float64 `env:"TAKE_PROFITS" envDefault:"0.6,0.34,0.27,0.2"`
	TakeProfitsMaxLookbackMinutes  []int     `env:"TAKE_PROFITS_MAX_LOOKBACK_MINUTES" envDefault:"10,15,25,40"`
	TakeProfitsBlockMinutes        []int     `env:"TAKE_PROFITS_BLOCK_MINUTES" envDefault:"40,-1,-1,-1"`
	StopLossNotPickedHrs           []int     `env:"STOP_LOSS_NOT_PICKED_HRS" envDefault:"1,2,3,4,5,6"`
	StopLossNotPicked              []float64 `env:"STOP_LOSS_NOT_PICKED" envDefault:"0,-0.05,-0.1,-0.15,-0.2,-0.3"`
	TickEverySeconds               int       `env:"TICK_EVERY_SECONDS" envDefault:"30"`
	MaxUSDTChunks                  int       `env:"MAX_USDT_CHUNKS" envDefault:"5"`
	MaxUSDCChunks                  int       `env:"MAX_USDC_CHUNKS" envDefault:"2"`
	MaxNeutrals                    int       `env:"MAX_NEUTRALS" envDefault:"6"`
	MinInvestmentPerChunk          float64   `env:"MIN_INVESTMENT_PER_CHUNK" envDefault:"10"`
	MaxCancelLossStrategyDeleted   float64   `env:"MAX_CANCEL_LOSS_STRATEGY_DELETED" envDefault:"0"`
	Mode                           string    `env:"MODE" envDefault:"trading"`
	PreferredLeverage              int       `env:"PREFERRED_LEVERAGE" envDefault:"20"`
	MaxLeverage                    int       `env:"MAX_LEVERAGE" envDefault:"60"`
	KeepTopNStrategiesOfSameSymbol int       `env:"KEEP_TOP_N_STRATEGIES_OF_SAME_SYMBOL" envDefault:"99"`
	Last3HrWeight                  float64   `env:"LAST_3_HR_WEIGHT" envDefault:"0"`
	Last2HrWeight                  float64   `env:"LAST_2_HR_WEIGHT" envDefault:"1"`
	LastHrWeight                   float64   `env:"LAST_1_HR_WEIGHT" envDefault:"0"`
	StopLossMarkForRemoval         []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL" envDefault:"-0.35,-0.8"`
	StopLossMarkForRemovalSLAt     []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL_SL_AT" envDefault:"0,-0.1"`
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
	return (pf / 23) * float64(leverage)
}
