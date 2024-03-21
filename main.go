package main

import (
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"github.com/adshao/go-binance/v2/futures"
	mapset "github.com/deckarep/golang-set/v2"
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
	strategies, err := getTopStrategies(FUTURE, time.Duration(TheConfig.RuntimeMinHours)*time.Hour, time.Duration(TheConfig.RuntimeMaxHours)*time.Hour)
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
		ri, _ := strconv.ParseFloat(strategies[i].Roi, 64)
		rj, _ := strconv.ParseFloat(strategies[j].Roi, 64)
		return ri > rj
	})
	return strategies, nil
}

func GetRoiChange(roi StrategyRoi, t time.Duration) float64 {
	latestTimestamp := roi[0].Time
	latestRoi := roi[0].Roi
	for _, r := range roi {
		l := latestTimestamp - int64(t.Seconds())
		if r.Time <= l {
			return latestRoi - r.Roi
		}
	}
	return latestRoi - roi[len(roi)-1].Roi
}

func tick() error {
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
		runTime := time.Duration(s.RunningTime) * time.Second
		if len(s.Rois) > 1 {
			s.LastDayRoiChange = GetRoiChange(s.Rois, 24*time.Hour)
			s.Last3HrRoiChange = GetRoiChange(s.Rois, 3*time.Hour)
			s.Last2HrRoiChange = GetRoiChange(s.Rois, 2*time.Hour)
			s.LastHrRoiChange = GetRoiChange(s.Rois, 1*time.Hour)
			log.Infof("Last Day: %f, Last 3Hr: %f, Last 2Hr: %f, Last Hr: %f, Runtime: %s",
				s.LastDayRoiChange, s.Last3HrRoiChange, s.Last2HrRoiChange, s.LastHrRoiChange, runTime)
			if s.LastDayRoiChange > 0.1 && s.Last3HrRoiChange > 0.05 && s.Last2HrRoiChange > 0 && s.LastHrRoiChange > -0.05 {
				filtered = append(filtered, s)
				log.Infof("Picked")
			}
		}
		log.Infof("----------------")
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Last2HrRoiChange > filtered[j].Last2HrRoiChange
	})

	openGrids, existingPairs, existingIds, err := getOpenGrids()
	if err != nil {
		return err
	}

	filteredCopiedIds := mapset.NewSet[int]()
	for _, s := range filtered {
		filteredCopiedIds.Add(s.StrategyID)
	}

	log.Infof("----------------")
	expiredCopiedIds := existingIds.Difference(filteredCopiedIds)
	DiscordWebhook(fmt.Sprintf("Expired Strategies: %v", expiredCopiedIds))
	for _, id := range expiredCopiedIds.ToSlice() {
		log.Infof("Closing Grid: %d", id)
		err := closeGridConv(id, openGrids)
		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}

	if expiredCopiedIds.Cardinality() > 0 {
		DiscordWebhook("Cleared expired grids - Skip current run")
		return nil
	}

	log.Infof("----------------")

	chunks := float64(6 - len(openGrids.Data))
	log.Infof("Opening %f chunks", chunks)
	invChunk := (usdt - chunks) / chunks
	for _, s := range filtered {
		minInvestment, _ := strconv.ParseFloat(s.MinInvestment, 64)
		runTime := time.Duration(s.RunningTime) * time.Second
		DiscordWebhook(fmt.Sprintf("Investing %d: %s, %f/%f, Last Day: %f, Last 3Hr: %f, Last 2Hr: %f, Last Hr: %f, Roi: %s, Min Investment: %s, Runtime: %s",
			s.StrategyID, s.Symbol, invChunk, minInvestment, s.LastDayRoiChange,
			s.Last3HrRoiChange, s.Last2HrRoiChange, s.LastHrRoiChange, s.Roi, s.MinInvestment, runTime))
		if !existingPairs.Contains(s.Symbol) {
			errr := placeGrid(*s, invChunk)
			if errr != nil {
				log.Errorf("Error placing grid: %v", errr)
			} else {
				log.Infof("Placed Grid")
				time.Sleep(1 * time.Second)
			}
		} else {
			log.Infof("Already placed symbol")
		}
		log.Infof("----------------")
	}

	log.Infof("----------------")

	openGrids, existingPairs, existingIds, err = getOpenGrids()
	if err != nil {
		return err
	}
	return nil
}

func closeGridConv(copiedId int, openGrids *OpenGridResponse) error {
	gridToClose := copiedId
	gridCurrId := 0
	for _, g := range openGrids.Data {
		if g.CopiedStrategyID == gridToClose {
			gridCurrId = g.StrategyID
			break
		}
	}
	if gridCurrId != 0 {
		err := closeGrid(gridCurrId)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	configure()
	log.Infof("Public IP: %s", getPublicIP())
	if TheConfig.Paper {
		DiscordWebhook("Paper Trading")
	} else {
		DiscordWebhook("Real Trading")
	}
	sdk()
	_, err := scheduler.Every(TheConfig.TickEveryMinutes).Minutes().Do(
		func() {
			err := tick()
			if err != nil {
				log.Errorf("Error: %v", err)
			}
		},
	)
	if err != nil {
		log.Errorf("Error: %v", err)
		return
	}
	scheduler.StartBlocking()
}
