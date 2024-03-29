package sdk

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	log "github.com/sirupsen/logrus"
	"strconv"
)

var futuresClient *futures.Client
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
	futuresClient = binance.NewFuturesClient(config.TheConfig.ApiKey, config.TheConfig.SecretKey) // USDT-M Futures
}

func fetchMarketPrice(symbol string) (float64, error) {
	res, err := _fetchMarketPrice(symbol)
	if err != nil {
		discord.Infof("Error fetching market price: %v", err)
		return 0, err
	}
	return res, nil
}

func _fetchMarketPrice(symbol string) (float64, error) {
	res, err := futuresClient.NewListPricesService().Symbol(symbol).Do(context.Background())
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

func GetFutureUSDT() (float64, error) {
	res, err := futuresClient.NewGetBalanceService().Do(context.Background())
	if err != nil {
		return 0, err
	}
	for _, b := range res {
		log.Infof("Asset: %s, Balance: %s", b.Asset, b.Balance)
		if b.Asset == "USDT" {
			i, err := strconv.ParseFloat(b.Balance, 64)
			if err != nil {
				return 0, err
			}
			return i, nil
		}
	}
	return 0, nil
}
