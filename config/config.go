package config

import (
	"BinanceTopStrategies/cleanup"
	"context"
	"github.com/caarlos0/env"
	"github.com/redis/rueidis"
	log "github.com/sirupsen/logrus"
	"os"
	"reflect"
	"strings"
)

type Config struct {
	Redis                             string    `env:"REDIS" envDefault:"localhost:6379"`
	RedisPassword                     string    `env:"REDIS_PASSWORD" envDefault:""`
	ApiKey                            string    `env:"API_KEY"`
	SecretKey                         string    `env:"SECRET_KEY"`
	CSRFToken                         string    `redis:"CSRF"`
	COOKIE                            string    `redis:"COOKIE"`
	MarginType                        string    `env:"MARGIN_TYPE" envDefault:"CROSSED"`
	StrategiesCount                   int       `env:"STRATEGIES_COUNT" envDefault:"3000"`
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
	LastNHoursNoDips                  int       `env:"LAST_N_HOURS_NO_DIPS" envDefault:"6"`
	LastNHoursAllPositive             int       `env:"LAST_N_HOURS_NO_DIPS" envDefault:"6"`
	LeavingAsset                      float64   `env:"LEAVING_ASSET" envDefault:"20"`
	MaxPerChunk                       float64   `env:"MAX_PER_CHUNK" envDefault:"35"`
	CancelNoChangeMinutes             int       `env:"CANCEL_NO_CHANGE_MINUTES" envDefault:"30"`
	MinOppositeDirectionHigherRanking int       `env:"MIN_OPPOSITE_DIRECTION_HIGHER_RANKING" envDefault:"2"`
	SymbolDirectionShrinkMinConstant  int       `env:"SYMBOL_DIRECTION_SHRINK_MIN_CONSTANT" envDefault:"2"`
	SymbolDirectionShrink             []float64 `env:"SYMBOL_DIRECTION_SHRINK" envDefault:"0.82,0.65,0.45"`
	SymbolDirectionShrinkLoss         []float64 `env:"SYMBOL_DIRECTION_SHRINK_LOSS" envDefault:"0,-0.2,-0.35"`
	TradingBlockMinutesAfterCancel    int       `env:"TRADING_BLOCK_MINUTES_AFTER_CANCEL" envDefault:"3"`
	TakeProfits                       []float64 `env:"TAKE_PROFITS" envDefault:"0.6,0.38,0.25,0.15"`
	TakeProfitsMaxLookbackMinutes     []int     `env:"TAKE_PROFITS_MAX_LOOKBACK_MINUTES" envDefault:"10,20,30,40"`
	TakeProfitsBlockMinutes           []int     `env:"TAKE_PROFITS_BLOCK_MINUTES" envDefault:"40,-1,-1,-1"`
	StopLossNotPickedHrs              []int     `env:"STOP_LOSS_NOT_PICKED_HRS" envDefault:"1,2,3,4,5,6"`
	StopLossNotPicked                 []float64 `env:"STOP_LOSS_NOT_PICKED" envDefault:"0,-0.05,-0.1,-0.15,-0.2,-0.3"`
	TickEverySeconds                  int       `env:"TICK_EVERY_SECONDS" envDefault:"300"`
	AssetSymbol                       string    `env:"ASSET_SYMBOL" envDefault:"USDT"`
	MaxChunks                         int       `env:"MAX_CHUNKS" envDefault:"7"`
	MinInvestmentPerChunk             float64   `env:"MIN_INVESTMENT_PER_CHUNK" envDefault:"6"`
	MaxCancelLossStrategyDeleted      float64   `env:"MAX_CANCEL_LOSS_STRATEGY_DELETED" envDefault:"0"`
	Mode                              string    `env:"MODE" envDefault:"trading"`
	PreferredLeverage                 int       `env:"PREFERRED_LEVERAGE" envDefault:"18"`
	MaxLeverage                       int       `env:"MAX_LEVERAGE" envDefault:"25"`
	KeepTopNStrategiesOfSameSymbol    int       `env:"KEEP_TOP_N_STRATEGIES_OF_SAME_SYMBOL" envDefault:"99"`
	Last3HrWeight                     float64   `env:"LAST_3_HR_WEIGHT" envDefault:"0"`
	Last2HrWeight                     float64   `env:"LAST_2_HR_WEIGHT" envDefault:"1"`
	LastHrWeight                      float64   `env:"LAST_1_HR_WEIGHT" envDefault:"0"`
	StopLossMarkForRemoval            []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL" envDefault:"-0.9,-1.1"`
	StopLossMarkForRemovalSlack       []float64 `env:"STOP_LOSS_MARK_FOR_REMOVAL_SLACK" envDefault:"0.5,0.2"`
	NeutralRangeDiff                  float64   `env:"NEUTRAL_RANGE_DIFF" envDefault:"0.4"`
	ShortRangeDiff                    float64   `env:"SHORT_RANGE_DIFF" envDefault:"0.4"`
	LongRangeDiff                     float64   `env:"LONG_RANGE_DIFF" envDefault:"0.4"`
	TriggerRangeDiff                  float64   `env:"TRIGGER_RANGE_DIFF" envDefault:"0.04"`
	PGUrl                             string    `env:"PGURL" envDefault:"postgresql://postgres:password@localhost:5432"`
	MaxLookBackBookingHours           int       `env:"MAX_LOOK_BACK_BOOKING_HOURS" envDefault:"2"`
}

var TheConfig = &Config{}

var rdb rueidis.Client

func Init() {
	err := env.Parse(TheConfig)
	if err != nil {
		log.Fatalf("error parsing config: %v", err)
	}
	redisFields := make(map[string]reflect.StructField)
	for i := 0; i < reflect.ValueOf(TheConfig).Elem().NumField(); i++ {
		field := reflect.ValueOf(TheConfig).Elem().Field(i)
		tag := reflect.TypeOf(TheConfig).Elem().Field(i).Tag.Get("redis")
		if tag == "" {
			continue
		}
		if field.Kind() == reflect.String {
			redisFields[tag] = reflect.TypeOf(TheConfig).Elem().Field(i)
		}
	}
	rdb, err = rueidis.NewClient(rueidis.ClientOption{
		InitAddress: []string{TheConfig.Redis},
		Password:    TheConfig.RedisPassword,
	})
	if err != nil {
		panic(err)
	}
	cleanup.AddOnStopFunc(func(_ os.Signal) {
		rdb.Close()
	})
	for k, v := range redisFields {
		ctx := context.Background()
		s, err := rdb.Do(ctx, rdb.B().Get().Key(k).Build()).ToString()
		if err == nil {
			s = strings.ReplaceAll(s, "\n", "")
			reflect.ValueOf(TheConfig).Elem().FieldByName(v.Name).SetString(s)
		}
	}
}

func GetScaledProfits(pf float64, leverage int) float64 {
	return (pf / 23) * float64(leverage)
}
