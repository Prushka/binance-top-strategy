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

func IfRoiDecreasedWithin(roi StrategyRoi, t time.Duration) bool {
	// check if any roi row within t duration has decreased compared to the previous one
	latestTimestamp := roi[0].Time
	for i := 0; i < len(roi)-1; i++ {
		if roi[i].Roi-roi[i+1].Roi < 0 {
			return true
		}
		l := latestTimestamp - int64(t.Seconds())
		if roi[i+1].Time <= l {
			return false
		}
	}
	return false
}

var globalStrategies = make(map[int]*Strategy)

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
	validRois := make(Strategies, 0)
	for _, s := range m {
		log.Infof("Strategy: %s, %s, %d", s.Roi, s.Symbol, len(s.Rois))
		if len(s.Rois) > 1 {
			s.LastDayRoiChange = GetRoiChange(s.Rois, 24*time.Hour)
			s.Last3HrRoiChange = GetRoiChange(s.Rois, 3*time.Hour)
			s.Last2HrRoiChange = GetRoiChange(s.Rois, 2*time.Hour)
			s.LastHrRoiChange = GetRoiChange(s.Rois, 1*time.Hour)
			log.Info(s.display())
			if s.LastDayRoiChange > 0.1 && s.Last3HrRoiChange > 0.05 && s.Last2HrRoiChange > 0 && s.LastHrRoiChange > -0.05 {
				validRois = append(validRois, s)
				log.Info("Picked")
			}
		}
		globalStrategies[s.StrategyID] = s
		log.Info("----------------")
	}
	sort.Slice(validRois, func(i, j int) bool {
		I := validRois[i]
		J := validRois[j]
		iWeight := I.Last3HrRoiChange*TheConfig.Last3HrWeight + I.Last2HrRoiChange*TheConfig.Last2HrWeight + I.LastHrRoiChange*TheConfig.LastHrWeight
		jWeight := J.Last3HrRoiChange*TheConfig.Last3HrWeight + J.Last2HrRoiChange*TheConfig.Last2HrWeight + J.LastHrRoiChange*TheConfig.LastHrWeight
		return iWeight > jWeight
	})
	DiscordWebhook(fmt.Sprintf("Found %d valid strategies", len(validRois)))

	symbolCount := make(map[string]int)

	filtered := make(Strategies, 0)
	for _, s := range validRois {
		if symbolCount[s.Symbol] < TheConfig.KeepTopNStrategiesOfSameSymbol {
			filtered = append(filtered, s)
			symbolCount[s.Symbol]++
		}
	}

	openGrids, err := getOpenGrids()
	if err != nil {
		return err
	}
	for _, grid := range openGrids.Data {
		trackRoi(grid)
	}

	filteredCopiedIds := mapset.NewSet[int]()
	for _, s := range filtered {
		filteredCopiedIds.Add(s.StrategyID)
	}

	log.Infof("----------------")
	expiredCopiedIds := openGrids.existingIds.Difference(filteredCopiedIds)
	if expiredCopiedIds.Cardinality() > 0 {
		DiscordWebhook(fmt.Sprintf("Expired Strategies: %v", expiredCopiedIds))
	}
	closedIds := mapset.NewSet[int]()
	for c, id := range expiredCopiedIds.ToSlice() {
		reason := ""
		att, ok := globalStrategies[id]
		if m.findById(id) == nil {
			reason += "Strategy not found"
		} else if ok && !filteredCopiedIds.Contains(id) {
			reason += "Strategy not picked"
		}

		log.Infof("Closing Grid: %d", id)
		tracked, ok := globalGrids[id]
		if ok && tracked.LastRoi < -0.04 { // attempting to close loss
			if tracked.ContinuousRoiLoss < 3 {
				DiscordWebhook(display(att, tracked.grid, "Skip Cancel "+reason, c+1, expiredCopiedIds.Cardinality()))
				continue
			}
		}
		err := closeGridConv(id, openGrids)
		if err != nil {
			return err
		}
		closedIds.Add(id)
		DiscordWebhook(display(att, tracked.grid, "Cancelled "+reason, c+1, expiredCopiedIds.Cardinality()))
		time.Sleep(1 * time.Second)
	}

	for _, grid := range openGrids.Data {
		t, ok := globalGrids[grid.CopiedStrategyID]
		if ok && t.ContinuousRoiNoChange > 3 && grid.profit >= 0 {
			err := closeGridConv(grid.CopiedStrategyID, openGrids)
			if err != nil {
				return err
			}
			closedIds.Add(grid.CopiedStrategyID)
			DiscordWebhook(display(globalStrategies[grid.CopiedStrategyID], t.grid, "Cancelled No Change", 0, 0))
			time.Sleep(1 * time.Second)
		}
	}

	if closedIds.Cardinality() > 0 && !TheConfig.Paper {
		DiscordWebhook("Cleared expired grids - Skip current run")
		return nil
	}

	log.Infof("----------------")

	for c, grid := range openGrids.Data {
		DiscordWebhook(display(m.findById(grid.CopiedStrategyID), grid, "Existing", c+1, len(openGrids.Data)))
	}

	if TheConfig.MaxChunks-len(openGrids.Data) <= 0 && !TheConfig.Paper {
		DiscordWebhook("Max Chunks reached, No cancel - Skip current run")
		return nil
	}

	chunksInt := TheConfig.MaxChunks - len(openGrids.Data)
	chunks := float64(TheConfig.MaxChunks - len(openGrids.Data))
	invChunk := (usdt - chunks*0.8) / chunks
	idealInvChunk := (usdt + openGrids.totalGridPnl + openGrids.totalGridInitial) / float64(TheConfig.MaxChunks)
	log.Infof("Ideal Investment: %f, allowed Investment: %f, missing %f chunks", idealInvChunk, invChunk, chunks)
	if invChunk > idealInvChunk {
		invChunk = idealInvChunk
	}
	canPlace := make(Strategies, 0)
	for _, s := range filtered {
		if s.Last2HrRoiChange > s.LastHrRoiChange {
			canPlace = append(canPlace, s)
		}
	}
	DiscordWebhook(fmt.Sprintf("Found %d strategies with increasing Roi over 3 hrs", len(canPlace)))
	for c, s := range canPlace {
		if !openGrids.existingIds.Contains(s.StrategyID) {
			DiscordWebhook(display(s, nil, "New", c+1, len(canPlace)))
		}
		if !openGrids.existingPairs.Contains(s.Symbol) {
			switch s.Direction {
			case LONG:
				if openGrids.totalLongs >= TheConfig.MaxLongs {
					DiscordWebhook("Max Longs reached, Skip")
					continue
				}
			case NEUTRAL:
				if openGrids.totalNeutrals >= TheConfig.MaxNeutrals {
					DiscordWebhook("Max Neutrals not reached, Skip")
					continue
				}
			}

			errr := placeGrid(*s, invChunk)
			if TheConfig.Paper {

			} else if errr != nil {
				DiscordWebhook(fmt.Sprintf("Error placing grid: %v", errr))
			} else {
				DiscordWebhook(fmt.Sprintf("Placed grid"))
				chunksInt -= 1
				openGrids.existingPairs.Add(s.Symbol)
				time.Sleep(1 * time.Second)
				if chunksInt <= 0 {
					break
				}
			}
		} else {
			log.Infof("Already placed symbol")
		}
		log.Infof("----------------")
	}

	log.Infof("----------------")

	newOpenGrids, err := getOpenGrids()
	if err != nil {
		return err
	}
	for _, newId := range newOpenGrids.existingIds.Difference(openGrids.existingIds).ToSlice() {
		DiscordWebhook(display(m.findById(newId), nil, "Placed", 0, 0))
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
	DiscordService()
	switch TheConfig.Mode {
	case "trading":
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

	case "extract-cookies":

	}
}
