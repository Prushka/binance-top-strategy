package config

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiKey                                 string    `env:"API_KEY"`
	SecretKey                              string    `env:"SECRET_KEY"`
	CSRFToken                              string    `env:"CSRF"`
	COOKIE                                 string    `env:"COOKIE"`
	MarginType                             string    `env:"MARGIN_TYPE" envDefault:"CROSSED"`
	StrategiesCount                        int       `env:"STRATEGIES_COUNT" envDefault:"160"`
	RuntimeMinHours                        int       `env:"RUNTIME_MIN_HOURS" envDefault:"3"`
	RuntimeMaxHours                        int       `env:"RUNTIME_MAX_HOURS" envDefault:"168"`
	Paper                                  bool      `env:"PAPER" envDefault:"true"`
	DiscordWebhook                         string    `env:"DISCORD_WEBHOOK"`
	DiscordWebhookAction                   string    `env:"DISCORD_WEBHOOK_ACTION"`
	DiscordWebhookOrder                    string    `env:"DISCORD_WEBHOOK_ORDER"`
	DiscordName                            string    `env:"DISCORD_NAME" envDefault:"BTS"`
	DataFolder                             string    `env:"DATA_FOLDER" envDefault:"./data"`
	LastNHoursNoDips                       int       `env:"LAST_N_HOURS_NO_DIPS" envDefault:"6"`
	LeavingAsset                           float64   `env:"LEAVING_ASSET" envDefault:"10"`
	CancelNoChangeMinutes                  int       `env:"CANCEL_NO_CHANGE_MINUTES" envDefault:"15"`
	CancelSymbolDirectionShrinkMinConstant int       `env:"CANCEL_SYMBOL_DIRECTION_SHRINK_MIN_CONSTANT" envDefault:"2"`
	CancelSymbolDirectionShrink            float64   `env:"CANCEL_SYMBOL_DIRECTION_SHRINK" envDefault:"0.82"`
	CancelWithLossSymbolDirectionShrink    float64   `env:"CANCEL_WITH_LOSS_SYMBOL_DIRECTION_SHRINK" envDefault:"0.65"`
	MaxLossWithSymbolDirectionShrink       float64   `env:"MAX_LOSS_WITH_SYMBOL_DIRECTION_SHRINK" envDefault:"-0.2"`
	TradingBlockMinutesAfterCancel         int       `env:"TRADING_BLOCK_MINUTES_AFTER_CANCEL" envDefault:"3"`
	TakeProfits                            []float64 `env:"TAKE_PROFITS" envDefault:"0.8,0.5,0.3"`
	TakeProfitsMaxLookbackMinutes          []int     `env:"TAKE_PROFITS_MAX_LOOKBACK_MINUTES" envDefault:"5,15,25"`
	TakeProfitsBlockMinutes                []int     `env:"TAKE_PROFITS_BLOCK_MINUTES" envDefault:"40,-1,-1"`
	TickEverySeconds                       int       `env:"TICK_EVERY_SECONDS" envDefault:"60"`
	AssetSymbol                            string    `env:"ASSET_SYMBOL" envDefault:"USDT"`
	MaxChunks                              int       `env:"MAX_CHUNKS" envDefault:"4"`
	MaxCancelLoss                          float64   `env:"MAX_CANCEL_LOSS" envDefault:"0"`
	MaxCancelLossStrategyDeleted           float64   `env:"MAX_CANCEL_LOSS_STRATEGY_DELETED" envDefault:"-0.2"`
	Mode                                   string    `env:"MODE" envDefault:"trading"`
	MaxLeverage                            int       `env:"LEVERAGE" envDefault:"40"`
	KeepTopNStrategiesOfSameSymbol         int       `env:"KEEP_TOP_N_STRATEGIES_OF_SAME_SYMBOL" envDefault:"99"`
	Last3HrWeight                          float64   `env:"LAST_3_HR_WEIGHT" envDefault:"0"`
	Last2HrWeight                          float64   `env:"LAST_2_HR_WEIGHT" envDefault:"1"`
	LastHrWeight                           float64   `env:"LAST_1_HR_WEIGHT" envDefault:"0"`
}

var TheConfig = &Config{}

func Init() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}