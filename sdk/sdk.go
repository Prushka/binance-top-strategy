package sdk

import (
	"BinanceTopStrategies/config"
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

func GetFuture(currency string) (float64, error) {
	res, err := FuturesClient.NewGetBalanceService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	c := 0.0
	for _, b := range res {
		log.Debugf("Asset: %s, Balance: %s", b.Asset, b.Balance)
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
			log.Debugf("%s: %f", currency, c)
			break
		}
	}
	return c, nil
}
