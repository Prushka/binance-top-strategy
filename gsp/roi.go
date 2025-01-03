package gsp

import (
	"BinanceTopStrategies/cache"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/request"
	"BinanceTopStrategies/sql"
	"BinanceTopStrategies/utils"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"math"
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
		if time.Now().Sub(latestTime) > 60*time.Minute {
			return true
		}
		return false
	},
)

type UserWL struct {
	UpdatedAt   time.Time `json:"updatedAt"`
	DirectionWL map[int]*WL
	UserId      int `json:"userId"`
}

type WL struct {
	TotalWL           float64
	Total             float64
	Win               float64
	WinRatio          float64
	ShortRunning      float64
	ShortRunningRatio float64
	EarliestTime      time.Time
	Id                string
}

const (
	WlVersion = 1
)

func (wl UserWL) insert() {
	err := sql.SimpleTransaction(func(tx pgx.Tx) error {
		for _, w := range wl.DirectionWL {
			err := w.insert(wl.UserId, wl.UpdatedAt, tx)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		discord.Errorf("Error inserting WL: %v", err)
	}
}

func (wl WL) insert(userId int, updatedAt time.Time, tx pgx.Tx) error {
	if wl.Total == 0 {
		return nil
	}
	_, err := tx.Exec(context.Background(),
		`INSERT INTO bts.wl (user_id, direction, total, total_wl, win, win_ratio, short_running, short_running_ratio, earliest, time_updated, version) 
    			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (user_id, direction) DO UPDATE
    			SET total = EXCLUDED.total,
    			    total_wl = EXCLUDED.total_wl,
    			    win = EXCLUDED.win,
    			    win_ratio = EXCLUDED.win_ratio,
    			    short_running = EXCLUDED.short_running,
    			    short_running_ratio = EXCLUDED.short_running_ratio,
    			    earliest = EXCLUDED.earliest,
    			    time_updated = EXCLUDED.time_updated,
    			    version = EXCLUDED.version;`,
		userId, wl.Id, wl.Total, wl.TotalWL, wl.Win, wl.WinRatio, wl.ShortRunning, wl.ShortRunningRatio, wl.EarliestTime, updatedAt, WlVersion)
	if err != nil {
		discord.Errorf("Error inserting WL: %v", err)
	}
	return err
}

func (wl UserWL) String() string {
	return fmt.Sprintf("User %d - %s %s %s %s",
		wl.UserId, wl.DirectionWL[TOTAL],
		wl.DirectionWL[NEUTRAL], wl.DirectionWL[LONG], wl.DirectionWL[SHORT])
}

func (wl WL) String() string {
	if wl.Total == 0 {
		return ""
	} else {
		return fmt.Sprintf("%s: [%.1f%% (%.1f/%.1f)|Short: %.1f%% (%.1f/%.1f)|%v]",
			wl.Id, wl.WinRatio*100, wl.Win, wl.TotalWL,
			wl.ShortRunningRatio*100, wl.ShortRunning, wl.Total, wl.EarliestTime)
	}
}

var UserWLCache = cache.CreateMapCache[UserWL](
	func(key string) (UserWL, error) {
		user, _ := strconv.Atoi(key)
		strategies := make([]*UserStrategy, 0)
		err := sql.GetDB().Scan(&strategies,
			`WITH Pool AS (
    SELECT * FROM bts.strategy WHERE user_id = $1 AND concluded=true AND high_price IS NOT NULL AND strategy_type = 2
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
),
     FilteredStrategies AS (
         SELECT
             l.strategy_id,
             l.roi,
             l.pnl,
             l.pnl / NULLIF(l.roi, 0) as original_input
         FROM
             LatestRoi l
         WHERE
             l.rn = 1
     )SELECT
          f.roi as roi, f.pnl as pnl, f.original_input,
          p.start_time, p.end_time, p.start_price, p.end_price,
          p.high_price, p.low_price,
          p.symbol, p.copy_count, p.strategy_id, p.strategy_type, p.direction, p.time_discovered,
          p.user_id, p.rois_fetched_at, p.type, p.lower_limit, p.upper_limit,
          p.grid_count, p.trigger_price, p.stop_lower_limit, p.stop_upper_limit, p.base_asset, p.quote_asset,
          p.leverage, p.trailing_down, p.trailing_up, p.trailing_type, p.latest_matched_count, p.matched_count, p.min_investment,
          p.concluded
FROM FilteredStrategies f JOIN Pool p ON f.strategy_id = p.strategy_id
WHERE f.original_input > 349;`, user)
		if err != nil {
			return UserWL{}, err
		}
		directionWL := map[int]*WL{
			TOTAL:   {Id: "TOTAL"},
			LONG:    {Id: "LONG"},
			SHORT:   {Id: "SHORT"},
			NEUTRAL: {Id: "NEUTRAL"},
		}
		wl := UserWL{
			UpdatedAt:   time.Now(),
			DirectionWL: directionWL,
			UserId:      user}
		for _, s := range strategies {
			start := *s.StartPrice
			end := *s.EndPrice
			high := *s.HighPrice
			low := *s.LowPrice
			s.RunningTime = int(s.EndTime.Sub(*s.StartTime).Seconds())
			if err != nil {
				return UserWL{}, err
			}
			priceDiffPct := math.Abs((end - start) / start)
			smlChange := priceDiffPct < 0.006
			shortRunning := s.RunningTime <= 3600*2
			w := directionWL[s.Direction]
			w.Total++
			if shortRunning {
				w.ShortRunning++
			}
			if !(shortRunning && smlChange) {
				w.TotalWL++
			} else {
				continue
			}
			if w.EarliestTime.IsZero() || s.StartTime.Before(w.EarliestTime) {
				w.EarliestTime = *s.StartTime
			}
			switch s.Direction {
			case LONG:
				if end > start && s.ROI > 0 {
					modifier := 1.0
					if low <= s.LowerLimit {
						modifier *= 0.1
					}
					lowDiff := (start - low) / start
					if lowDiff > 0.1 {
						modifier *= 0.1
					}
					if smlChange {
						modifier *= 0.5
					}
					w.Win += modifier * 1
				} else if !smlChange {
					w.Win -= 1
				}
			case SHORT:
				if end < start && s.ROI > 0 {
					modifier := 1.0
					if high >= s.UpperLimit {
						modifier *= 0.1
					}
					highDiff := (high - start) / start
					if highDiff > 0.1 {
						modifier *= 0.1
					}
					if smlChange {
						modifier *= 0.5
					}
					w.Win += modifier * 1
				} else if !smlChange {
					w.Win -= 1
				}
			case NEUTRAL:
				threshold := 0.065
				lossThreshold := 0.15
				mid := (s.LowerLimit + s.UpperLimit) / 2
				if end < s.UpperLimit && end > s.LowerLimit && s.ROI > 0 {
					modifier := 1.0
					if low <= s.LowerLimit || high >= s.UpperLimit {
						modifier *= 0
					}
					if utils.InRange(end, start, threshold) {
						modifier *= 1
					} else if utils.InRange(end, mid, threshold) {
						modifier *= 0.8
					} else if utils.InRange(end, start, lossThreshold) {
						modifier *= 0.4
					} else {
						modifier *= 0.1
					}
					w.Win += modifier
				} else {
					w.Win -= 12
				}
			}
			log.Debugf("Symbol: %s, Direction: %d, Start: %.5f, End: %.5f, %v (%.5f, %.5f)",
				s.Symbol, s.Direction, start, end, time.Duration(s.RunningTime)*time.Second, s.LowerLimit, s.UpperLimit)
		}
		directionWL[LONG].WinRatio = directionWL[LONG].Win / directionWL[LONG].TotalWL
		directionWL[LONG].ShortRunningRatio = directionWL[LONG].ShortRunning / directionWL[LONG].Total
		directionWL[SHORT].WinRatio = directionWL[SHORT].Win / directionWL[SHORT].TotalWL
		directionWL[SHORT].ShortRunningRatio = directionWL[SHORT].ShortRunning / directionWL[SHORT].Total
		directionWL[NEUTRAL].WinRatio = directionWL[NEUTRAL].Win / directionWL[NEUTRAL].TotalWL
		directionWL[NEUTRAL].ShortRunningRatio = directionWL[NEUTRAL].ShortRunning / directionWL[NEUTRAL].Total
		directionWL[TOTAL] = &WL{
			Id:      "TOTAL",
			TotalWL: directionWL[LONG].TotalWL + directionWL[SHORT].TotalWL + directionWL[NEUTRAL].TotalWL,
			Total:   directionWL[LONG].Total + directionWL[SHORT].Total + directionWL[NEUTRAL].Total,
			Win:     directionWL[LONG].Win + directionWL[SHORT].Win + directionWL[NEUTRAL].Win,
			WinRatio: (directionWL[LONG].Win + directionWL[SHORT].Win + directionWL[NEUTRAL].Win) /
				(directionWL[LONG].TotalWL + directionWL[SHORT].TotalWL + directionWL[NEUTRAL].TotalWL),
			ShortRunning: directionWL[LONG].ShortRunning + directionWL[SHORT].ShortRunning + directionWL[NEUTRAL].ShortRunning,
			ShortRunningRatio: (directionWL[LONG].ShortRunning + directionWL[SHORT].ShortRunning + directionWL[NEUTRAL].ShortRunning) /
				(directionWL[LONG].Total + directionWL[SHORT].Total + directionWL[NEUTRAL].Total),
			EarliestTime: utils.MinTime(directionWL[LONG].EarliestTime, directionWL[SHORT].EarliestTime, directionWL[NEUTRAL].EarliestTime),
		}
		wl.insert()
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

func (rois StrategyRoi) lastNRecords(n int) string {
	n += 1
	if len(rois) < n {
		n = len(rois)
	}
	var ss []string
	for i := 0; i < n; i++ {
		ss = append(ss, fmt.Sprintf("%.2f%%", rois[i].Roi*100))
	}
	slices.Reverse(ss)
	return strings.Join(ss, ", ")
}

func (rois StrategyRoi) GetRoiChange(t time.Duration) float64 {
	latestTimestamp := rois[0].Time
	latestRoi := rois[0].Roi
	l := latestTimestamp - int64(t.Seconds())
	for _, r := range rois {
		if r.Time <= l {
			return latestRoi - r.Roi
		}
	}
	return latestRoi - rois[len(rois)-1].Roi
}

func (rois StrategyRoi) GetRoiPerHr(t time.Duration) float64 {
	latestTimestamp := rois[0].Time
	latestRoi := rois[0].Roi
	l := latestTimestamp - int64(t.Seconds())
	hrs := float64(t.Seconds()) / 3600
	for _, r := range rois {
		if r.Time <= l {
			return (latestRoi - r.Roi) / hrs
		}
	}
	return (latestRoi - rois[len(rois)-1].Roi) / (float64(rois[0].Time-rois[len(rois)-1].Time) / 3600)
}

func (rois StrategyRoi) AllPositive(t time.Duration, cutoff float64) bool {
	latestTimestamp := rois[0].Time
	l := latestTimestamp - int64(t.Seconds())
	for c, r := range rois {
		if r.Time < l {
			return true
		}
		if c > 0 && rois[c-1].Roi-r.Roi < cutoff {
			return false
		}
	}
	return true
}
