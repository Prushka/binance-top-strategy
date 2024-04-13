package config

import (
	"github.com/caarlos0/env"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	ApiKey                            string    `env:"API_KEY"`
	SecretKey                         string    `env:"SECRET_KEY"`
	CSRFToken                         string    `env:"CSRF"`
	COOKIE                            string    `env:"COOKIE"`
	MarginType                        string    `env:"MARGIN_TYPE" envDefault:"CROSSED"`
	StrategiesCount                   int       `env:"STRATEGIES_COUNT" envDefault:"480"`
	RuntimeMinHours                   int       `env:"RUNTIME_MIN_HOURS" envDefault:"3"`
	RuntimeMaxHours                   int       `env:"RUNTIME_MAX_HOURS" envDefault:"168"`
	Paper                             bool      `env:"PAPER" envDefault:"true"`
	DiscordWebhook                    string    `env:"DISCORD_WEBHOOK"`
	DiscordWebhookAction              string    `env:"DISCORD_WEBHOOK_ACTION"`
	DiscordWebhookOrder               string    `env:"DISCORD_WEBHOOK_ORDER"`
	DiscordWebhookError               string    `env:"DISCORD_WEBHOOK_ERROR"`
	DiscordWebhookBlacklist           string    `env:"DISCORD_WEBHOOK_BLACKLIST"`
	DiscordName                       string    `env:"DISCORD_NAME" envDefault:"BTS"`
	DataFolder                        string    `env:"DATA_FOLDER" envDefault:"./data"`
	ShiftMinutesAfterHour             int       `env:"SHIFT_MINUTES_AFTER_HOUR" envDefault:"0"`
	LastNHoursNoDips                  int       `env:"LAST_N_HOURS_NO_DIPS" envDefault:"5"`
	LastNHoursAllPositive             int       `env:"LAST_N_HOURS_NO_DIPS" envDefault:"5"`
	LeavingAsset                      float64   `env:"LEAVING_ASSET" envDefault:"0"`
	CancelNoChangeMinutes             int       `env:"CANCEL_NO_CHANGE_MINUTES" envDefault:"15"`
	MinOppositeDirectionHigherRanking int       `env:"MIN_OPPOSITE_DIRECTION_HIGHER_RANKING" envDefault:"2"`
	SymbolDirectionShrinkMinConstant  int       `env:"SYMBOL_DIRECTION_SHRINK_MIN_CONSTANT" envDefault:"2"`
	SymbolDirectionShrink             []float64 `env:"SYMBOL_DIRECTION_SHRINK" envDefault:"0.82,0.65,0.45"`
	SymbolDirectionShrinkLoss         []float64 `env:"SYMBOL_DIRECTION_SHRINK_LOSS" envDefault:"0,-0.2,-0.35"`
	TradingBlockMinutesAfterCancel    int       `env:"TRADING_BLOCK_MINUTES_AFTER_CANCEL" envDefault:"3"`
	TakeProfits                       []float64 `env:"TAKE_PROFITS" envDefault:"0.5,0.35,0.2,0.12"`
	TakeProfitsMaxLookbackMinutes     []int     `env:"TAKE_PROFITS_MAX_LOOKBACK_MINUTES" envDefault:"5,15,25,35"`
	TakeProfitsBlockMinutes           []int     `env:"TAKE_PROFITS_BLOCK_MINUTES" envDefault:"40,-1,-1,-1"`
	StopLossNotPickedHrs              []int     `env:"STOP_LOSS_NOT_PICKED_HRS" envDefault:"1,2,3,4,5"`
	StopLossNotPicked                 []float64 `env:"STOP_LOSS_NOT_PICKED" envDefault:"-0.05,-0.1,-0.15,-0.3,-0.4"`
	TickEverySeconds                  int       `env:"TICK_EVERY_SECONDS" envDefault:"20"`
	AssetSymbol                       string    `env:"ASSET_SYMBOL" envDefault:"USDT"`
	MaxChunks                         int       `env:"MAX_CHUNKS" envDefault:"6"`
	MinInvestmentPerChunk             float64   `env:"MIN_INVESTMENT_PER_CHUNK" envDefault:"6"`
	MaxCancelLossStrategyDeleted      float64   `env:"MAX_CANCEL_LOSS_STRATEGY_DELETED" envDefault:"-0.2"`
	Mode                              string    `env:"MODE" envDefault:"trading"`
	MaxLeverage                       int       `env:"LEVERAGE" envDefault:"35"`
	KeepTopNStrategiesOfSameSymbol    int       `env:"KEEP_TOP_N_STRATEGIES_OF_SAME_SYMBOL" envDefault:"99"`
	Last3HrWeight                     float64   `env:"LAST_3_HR_WEIGHT" envDefault:"0"`
	Last2HrWeight                     float64   `env:"LAST_2_HR_WEIGHT" envDefault:"1"`
	LastHrWeight                      float64   `env:"LAST_1_HR_WEIGHT" envDefault:"0"`
	StopLossMarkForRemoval            []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL" envDefault:"-0.7,-0.8,-0.9,-1"`
	StopLossMarkForRemovalSlack       []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL_SLACK" envDefault:"0.3,0.2,0.1,-0.1"`
}

var TheConfig = &Config{}

func Init() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
}

func GetScaledProfits(pf float64, leverage int) float64 {
	return (pf / 23) * float64(leverage)
}
