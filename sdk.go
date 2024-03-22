package main

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	log "github.com/sirupsen/logrus"
	"strconv"
)

var futuresClient *futures.Client

func sdk() {
	futuresClient = binance.NewFuturesClient(TheConfig.ApiKey, TheConfig.SecretKey) // USDT-M Futures
}

func fetchMarketPrice(symbol string) (float64, error) {
	res, err := _fetchMarketPrice(symbol)
	if err != nil {
		DiscordWebhook(fmt.Sprintf("Error fetching market price: %v", err))
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

func getFutureUSDT() (float64, error) {
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
