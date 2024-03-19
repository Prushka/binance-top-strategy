package main

import (
	"context"
	"github.com/adshao/go-binance/v2"
	log "github.com/sirupsen/logrus"
)

func main() {
	configure()
	futuresClient := binance.NewFuturesClient(TheConfig.ApiKey, TheConfig.SecretKey) // USDT-M Futures
	res, err := futuresClient.NewGetBalanceService().Do(context.Background())
	if err != nil {
		return
	}
	log.Infof("Futures Account Info: %+v", res)
}
