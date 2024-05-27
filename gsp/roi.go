package gsp

import (
	"BinanceTopStrategies/cache"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

var RoisCache = cache.CreateMapCache[StrategyRoi](
	func(key string) (StrategyRoi, error) {
		split := strings.Split(key, "-")
		SID, _ := strconv.Atoi(split[0])
		UserId, _ := strconv.Atoi(split[1])
		roi, err := getStrategyRois(int64(SID), int64(UserId))
		if err != nil {
			return nil, err
		}
		return roi, nil
	},
	func(rois StrategyRoi) bool {
		if len(rois) == 0 {
			return true
		}
		latestTime := time.Unix(rois[0].Time, 0)
		if time.Now().Sub(latestTime) > time.Duration(60+config.TheConfig.ShiftMinutesAfterHour)*time.Minute {
			return true
		}
		return false
	},
)

type UserWL struct {
	Win          int       `json:"wins"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ShortRunning int       `json:"shortRunning"`
	Total        int       `json:"total"`
}

var UserWLCache = cache.CreateMapCache[UserWL](
	func(key string) (UserWL, error) {
		user, _ := strconv.Atoi(key)
		strategies := make([]*UserStrategy, 0)
		err := sql.GetDB().Scan(&strategies,
			`WITH Pool AS (
    SELECT * FROM bts.strategy WHERE user_id = $1 AND concluded=true
), LatestRoi AS (
    SELECT
        r.strategy_id,
        r.roi as roi,
        r.pnl,
        r.time,
        ROW_NUMBER() OVER (PARTITION BY r.strategy_id ORDER BY time DESC) AS rn
    FROM
        bts.roi r
            JOIN Pool ON Pool.strategy_id = r.strategy_id
), EarliestRoi AS (
    SELECT
        r.strategy_id,
        r.time,
        ROW_NUMBER() OVER (PARTITION BY r.strategy_id ORDER BY time) AS rn
    FROM
        bts.roi r
            JOIN Pool ON Pool.strategy_id = r.strategy_id
),
     FilteredStrategies AS (
         SELECT
             l.strategy_id,
             l.roi,
             l.pnl,
             l.pnl / NULLIF(l.roi, 0) as original_input,
             EXTRACT(EPOCH FROM (l.time - e.time)) as runtime,
			 l.time as end_time,
			 e.time as start_time
         FROM
             LatestRoi l
                 JOIN
             EarliestRoi e ON l.strategy_id = e.strategy_id
         WHERE
             l.rn = 1 AND e.rn = 1
     )SELECT
          f.roi as roi, f.pnl as pnl, f.original_input, f.runtime as running_time,
		  f.start_time, f.end_time,
          p.symbol, p.copy_count, p.strategy_id, p.strategy_type, p.direction, p.time_discovered,
          p.user_id, p.price_difference, p.rois_fetched_at, p.type, p.lower_limit, p.upper_limit,
          p.grid_count, p.trigger_price, p.stop_lower_limit, p.stop_upper_limit, p.base_asset, p.quote_asset,
          p.leverage, p.trailing_down, p.trailing_up, p.trailing_type, p.latest_matched_count, p.matched_count, p.min_investment,
          p.concluded
FROM FilteredStrategies f JOIN Pool p ON f.strategy_id = p.strategy_id WHERE f.original_input > 498;`, user)
		if err != nil {
			return UserWL{}, err
		}
		wl := UserWL{Win: 0, Total: len(strategies), ShortRunning: 0, UpdatedAt: time.Now()}
		for _, s := range strategies {
			start, end, err := sdk.GetPrices(s.Symbol,
				s.StartTime.UnixMilli(), s.EndTime.UnixMilli())
			if err != nil {
				return UserWL{}, err
			}
			prefix := "lost "
			switch s.Direction {
			case LONG:
				if end > start {
					wl.Win++
					prefix = "won "
				}
			case SHORT:
				if end < start {
					wl.Win++
					prefix = "won "
				}
			case NEUTRAL:
				if end < s.UpperLimit && end > s.LowerLimit {
					wl.Win++
					prefix = "won "
				}
			}
			if s.RunningTime <= 3600*4 {
				wl.ShortRunning++
			}
			log.Debugf("%sSymbol: %s, Direction: %d, Start: %.5f, End: %.5f, %v (%.5f, %.5f)",
				prefix, s.Symbol, s.Direction, start, end, time.Duration(s.RunningTime)*time.Second, s.LowerLimit, s.UpperLimit)
		}
		discord.Infof("Total wins: %d/%d (%.2f)", wl.Win, len(strategies), float64(wl.Win)/float64(wl.Total))
		return wl, nil
	},
	func(wl UserWL) bool {
		return time.Now().Sub(wl.UpdatedAt) > 1*time.Hour
	})

type StrategyRoi []*Roi

type Roi struct {
	RootUserID int     `json:"rootUserId"`
	StrategyID int     `json:"strategyId"`
	Roi        float64 `json:"roi"`
	Pnl        float64 `json:"pnl"`
	Time       int64   `json:"time"`
}

type StrategyRoiResponse struct {
	Data StrategyRoi `json:"data"`
	request.BinanceBaseResponse
}

type QueryStrategyRoi struct {
	RootUserID           int64  `json:"rootUserId"`
	StrategyID           int64  `json:"strategyId"`
	StreamerStrategyType string `json:"streamerStrategyType"`
}

func getStrategyRois(strategyID int64, rootUserId int64) (StrategyRoi, error) {
	query := &QueryStrategyRoi{
		RootUserID:           rootUserId,
		StrategyID:           strategyID,
		StreamerStrategyType: "UM_GRID",
	}
	roi, _, err := request.Request(
		"https://www.binance.com/bapi/futures/v1/public/future/common/strategy/landing-page/queryRoiChart",
		query, &StrategyRoiResponse{})
	if err != nil {
		return nil, err
	}
	roiData := roi.Data
	for _, r := range roiData {
		r.Time = r.Time / 1000
	}
	sort.Slice(roiData, func(i, j int) bool {
		return roiData[i].Time > roiData[j].Time
	})
	return roiData, nil
}

func (roi StrategyRoi) lastNRecords(n int) string {
	n += 1
	if len(roi) < n {
		n = len(roi)
	}
	var ss []string
	for i := 0; i < n; i++ {
		ss = append(ss, fmt.Sprintf("%.2f%%", roi[i].Roi*100))
	}
	slices.Reverse(ss)
	return strings.Join(ss, ", ")
}

func (roi StrategyRoi) GetRoiChange(t time.Duration) float64 {
	latestTimestamp := roi[0].Time
	latestRoi := roi[0].Roi
	l := latestTimestamp - int64(t.Seconds())
	for _, r := range roi {
		if r.Time <= l {
			return latestRoi - r.Roi
		}
	}
	return latestRoi - roi[len(roi)-1].Roi
}

func (roi StrategyRoi) GetRoiPerHr(t time.Duration) float64 {
	latestTimestamp := roi[0].Time
	latestRoi := roi[0].Roi
	l := latestTimestamp - int64(t.Seconds())
	hrs := float64(t.Seconds()) / 3600
	for _, r := range roi {
		if r.Time <= l {
			return (latestRoi - r.Roi) / hrs
		}
	}
	return (latestRoi - roi[len(roi)-1].Roi) / (float64(roi[0].Time-roi[len(roi)-1].Time) / 3600)
}

func (roi StrategyRoi) AllPositive(t time.Duration, cutoff float64) bool {
	latestTimestamp := roi[0].Time
	l := latestTimestamp - int64(t.Seconds())
	for c, r := range roi {
		if r.Time < l {
			return true
		}
		if c > 0 && roi[c-1].Roi-r.Roi < cutoff {
			return false
		}
	}
	return true
}
