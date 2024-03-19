package main

import (
	"context"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	"github.com/go-co-op/gocron"
	log "github.com/sirupsen/logrus"
	"sort"
	"strconv"
	"time"
)

var scheduler = gocron.NewScheduler(time.Now().Location())
var futuresClient *futures.Client

func sdk() {
	futuresClient = binance.NewFuturesClient(TheConfig.ApiKey, TheConfig.SecretKey) // USDT-M Futures
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

func fetchStrategies() (Strategies, error) {
	strategies, err := getTopStrategies(FUTURE, dayToSeconds(2), dayToSeconds(7))
	if err != nil {
		return nil, err
	}
	for _, strategy := range strategies {
		id := strategy.StrategyID
		roi, err := getStrategyRois(id, strategy.UserID)
		if err != nil {
			return nil, err
		}
		for _, r := range roi {
			r.Time = r.Time / 1000
		}
		sort.Slice(roi, func(i, j int) bool {
			return roi[i].Time > roi[j].Time
		})
		strategy.Rois = roi
	}
	sort.Slice(strategies, func(i, j int) bool {
		return strategies[i].Roi > strategies[j].Roi
	})
	return strategies, nil
}

func tick() error {
	sdk()
	usdt, err := getFutureUSDT()
	if err != nil {
		return err
	}
	log.Infof("USDT: %f", usdt)
	m, err := fetchStrategies()
	if err != nil {
		return err
	}
	filtered := make(Strategies, 0)
	for _, s := range m {
		log.Infof("Strategy: %s, %s, %d", s.Roi, s.Symbol, len(s.Rois))
		if len(s.Rois) > 1 {
			latestTimestamp := s.Rois[0].Time
			latestRoi := s.Rois[0].Roi
			for _, r := range s.Rois {
				oneDayAgo := latestTimestamp - int64(dayToSeconds(1))
				thirtyHoursAgo := latestTimestamp - int64(hourToSeconds(30))
				if r.Time < oneDayAgo && r.Time > thirtyHoursAgo {
					roiChange := latestRoi - r.Roi
					if roiChange > 0.1 {
						filtered = append(filtered, s)
						log.Infof("Valid RoiChange: %s, %f", s.Symbol, roiChange)
					} else {
						log.Infof("Invalid RoiChange: %s, %f", s.Symbol, roiChange)
					}
					break
				}
			}
		}
		log.Infof("----------------")
	}
	return nil
}

func main() {
	configure()
	err := tick()
	if err != nil {
		log.Fatal(err)
	}
}
