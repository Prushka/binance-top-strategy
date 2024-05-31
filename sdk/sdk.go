package sdk

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/utils"
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	log "github.com/sirupsen/logrus"
	"math"
	"strconv"
)

var FuturesClient *futures.Client
var sessionSymbolPrice = make(map[string]float64)

func ClearSessionSymbolPrice() {
	sessionSymbolPrice = make(map[string]float64)
}

func GetSessionSymbolPrice(symbol string) (float64, error) {
	if _, ok := sessionSymbolPrice[symbol]; !ok {
		marketPrice, err := fetchMarketPrice(symbol)
		if err != nil {
			return 0, err
		}
		sessionSymbolPrice[symbol] = marketPrice
	}
	return sessionSymbolPrice[symbol], nil
}

func Init() {
	FuturesClient = binance.NewFuturesClient(config.TheConfig.ApiKey, config.TheConfig.SecretKey) // USDT-M Futures
}

func fetchMarketPrice(symbol string) (float64, error) {
	res, err := FuturesClient.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, err
	}
	for _, p := range res {
		if p.Symbol == symbol {
			return strconv.ParseFloat(p.Price, 64)
		}
	}
	return 0, fmt.Errorf("symbol not found")
}

func PopulatePrices(symbol string, timeStart int64, timeEnd int64) error {
	res, err := FuturesClient.NewKlinesService().Symbol(symbol).Interval("1m").
		StartTime(timeStart).EndTime(timeEnd).Limit(2000).Do(context.Background())
	if err != nil {
		log.Errorf("Start: %d, End: %d", timeStart, timeEnd)
		return err
	}
	if len(res) == 0 {
		return fmt.Errorf("no data")
	}
	for _, r := range res {
		log.Info(utils.AsJson(r))
	}
	log.Infof("Populated %d prices", len(res))
	return nil
}

func GetPrices(symbol string, timeStart int64, timeEnd int64) (float64, float64, error) {
	if timeStart == timeEnd {
		timeEnd = timeStart + 3600*1000
	}
	res, err := FuturesClient.NewKlinesService().Symbol(symbol).Interval("30m").
		StartTime(timeStart).EndTime(timeEnd).Do(context.Background())
	if err != nil {
		log.Errorf("Start: %d, End: %d", timeStart, timeEnd)
		return 0, 0, err
	}
	if len(res) == 0 {
		return 0, 0, fmt.Errorf("no data")
	}
	start, err := strconv.ParseFloat(res[0].Close, 64) // start time + 30 minutes
	if err != nil {
		return 0, 0, err
	}
	end, err := strconv.ParseFloat(res[len(res)-1].Close, 64) // end time + 30 minutes
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func GetFuture(currency string) (float64, error) {
	res, err := FuturesClient.NewGetBalanceService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	c := 0.0
	for _, b := range res {
		log.Infof("Asset: %s, Balance: %s", b.Asset, b.Balance)
		if b.Asset == currency {
			i, err := strconv.ParseFloat(b.Balance, 64)
			if err != nil {
				return 0, err
			}
			unPnl, err := strconv.ParseFloat(b.CrossUnPnl, 64)
			if err != nil {
				return 0, err
			}
			availableBalance, err := strconv.ParseFloat(b.AvailableBalance, 64)
			if err != nil {
				return 0, err
			}
			c = math.Min(i+unPnl, availableBalance)
			log.Infof("%s: %f", currency, c)
			break
		}
	}
	return c, nil
}
