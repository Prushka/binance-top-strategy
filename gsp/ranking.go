package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/sql"
	"BinanceTopStrategies/utils"
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2/futures"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

type RoiDB struct {
	StrategyID int     `db:"strategy_id"`
	Roi        float64 `db:"roi"`
	Pnl        float64 `db:"pnl"`
	Time       int64   `db:"time"`
}

type ChosenStrategyDB struct {
	StrategyDB
	UserMetricsDB
}

type UserMetricsDB struct {
	UserTotalRoi   float64 `db:"total_roi"`
	UserInput      float64 `db:"original_input"`
	UserTotalInput float64 `db:"total_original_input"`
	UserStrategies int     `db:"strategy_count"`
}

type UserStrategy struct {
	StrategyDB
	UserMetricsDB
	UserInput float64 `db:"original_input"`
}

type StrategyDB struct {
	Symbol             string  `db:"symbol"`
	CopyCount          int     `db:"copy_count"`
	ROI                float64 `db:"roi"`
	rois               StrategyRoi
	PNL                float64   `db:"pnl"`
	RunningTime        int       `db:"running_time"`
	StrategyID         int64     `db:"strategy_id"` // Use int64 for BIGINT
	StrategyType       int       `db:"strategy_type"`
	Direction          int       `db:"direction"`
	UserID             int64     `db:"user_id"` // Use int64 for BIGINT
	TimeDiscovered     time.Time `db:"time_discovered"`
	RoisFetchedAt      time.Time `db:"rois_fetched_at"`
	Type               string    `db:"type"`
	LowerLimit         float64   `db:"lower_limit"`
	UpperLimit         float64   `db:"upper_limit"`
	GridCount          int       `db:"grid_count"`
	TriggerPrice       *float64  `db:"trigger_price"` // Use pointer for nullable columns
	StopLowerLimit     *float64  `db:"stop_lower_limit"`
	StopUpperLimit     *float64  `db:"stop_upper_limit"`
	BaseAsset          *string   `db:"base_asset"` // Use pointer for nullable columns
	QuoteAsset         *string   `db:"quote_asset"`
	Leverage           *int      `db:"leverage"`
	TrailingUp         *bool     `db:"trailing_up"`
	TrailingDown       *bool     `db:"trailing_down"`
	TrailingType       *string   `db:"trailing_type"`
	LatestMatchedCount *int      `db:"latest_matched_count"`
	MatchedCount       *int      `db:"matched_count"`
	MinInvestment      *float64  `db:"min_investment"`
	Concluded          *bool     `db:"concluded"`
	PriceMetrics
}

type PriceMetrics struct {
	StartTime             *time.Time `db:"start_time"`
	EndTime               *time.Time `db:"end_time"`
	StartPrice            *float64   `db:"start_price"`
	EndPrice              *float64   `db:"end_price"`
	StartPriceExact       *float64   `db:"start_price_exact"`
	EndPriceExact         *float64   `db:"end_price_exact"`
	LowPrice              *float64   `db:"low_price"`
	HighPrice             *float64   `db:"high_price"`
	StartPrice30MinBefore *float64   `db:"start_price_30m_before"`
	EndPrice30MinBefore   *float64   `db:"end_price_30m_before"`
}

func floatPtrToStringPtr(f *float64) *string {
	if f == nil {
		return nil
	}
	s := fmt.Sprintf("%f", *f)
	return &s
}

func ToStrategies(dbSs []*ChosenStrategyDB) Strategies {
	ss := make(Strategies, 0)
	for _, dbS := range dbSs {
		ss = append(ss, dbS.ToStrategy())
	}
	return ss
}

func (db *ChosenStrategyDB) ToStrategy() *Strategy {
	s := &Strategy{
		Symbol:             db.Symbol,
		CopyCount:          db.CopyCount,
		RoiStr:             fmt.Sprintf("%f", db.ROI*100),
		PnlStr:             fmt.Sprintf("%f", db.PNL),
		RunningTime:        db.RunningTime,
		SID:                int(db.StrategyID),
		StrategyType:       db.StrategyType,
		Direction:          db.Direction,
		UserID:             int(db.UserID),
		TrailingType:       *db.TrailingType,
		LatestMatchedCount: *db.LatestMatchedCount,
		MatchedCount:       *db.MatchedCount,
		MinInvestment:      *floatPtrToStringPtr(db.MinInvestment),
		UserMetricsDB:      db.UserMetricsDB,
		StrategyParams: StrategyParams{
			Type:           db.Type,
			LowerLimitStr:  fmt.Sprintf("%f", db.LowerLimit),
			UpperLimitStr:  fmt.Sprintf("%f", db.UpperLimit),
			LowerLimit:     db.LowerLimit,
			UpperLimit:     db.UpperLimit,
			GridCount:      db.GridCount,
			TriggerPrice:   floatPtrToStringPtr(db.TriggerPrice),
			StopLowerLimit: floatPtrToStringPtr(db.StopLowerLimit),
			StopUpperLimit: floatPtrToStringPtr(db.StopUpperLimit),
			BaseAsset:      db.BaseAsset,
			QuoteAsset:     db.QuoteAsset,
			Leverage:       *db.Leverage,
			TrailingUp:     *db.TrailingUp,
			TrailingDown:   *db.TrailingDown,
		},
	}
	s.Sanitize()
	return s
}

func GetPrices(symbol string, timeStart int64, timeEnd int64) (*PriceMetrics, error) {
	if timeStart == timeEnd {
		timeEnd = timeStart + 3600*1000
	}
	metrics := &PriceMetrics{}
	log.Infof("Fetching prices for %s: %d, %d", symbol, timeStart, timeEnd)
	startRes, err := sdk.FuturesClient.NewKlinesService().Symbol(symbol).Interval("30m").
		StartTime(timeStart - 30*60*1000).EndTime(timeStart).Limit(4).Do(context.Background())
	if err != nil {
		return nil, err
	}
	endRes, err := sdk.FuturesClient.NewKlinesService().Symbol(symbol).Interval("30m").
		StartTime(timeEnd - 30*60*1000).EndTime(timeEnd).Limit(4).Do(context.Background())
	if err != nil {
		return nil, err
	}
	minMaxRes, err := sdk.FuturesClient.NewKlinesService().Symbol(symbol).Interval("1h").
		StartTime(timeStart).EndTime(timeEnd).Limit(1500).Do(context.Background())
	if err != nil {
		return nil, err
	}
	minMaxRes = minMaxRes[:len(minMaxRes)-1]
	if len(startRes) < 2 || len(endRes) < 2 || len(minMaxRes) < 1 {
		return nil, fmt.Errorf("insufficient data total: (%d/%d/%d)", len(startRes), len(endRes), len(minMaxRes))
	}
	if minMaxRes[0].OpenTime != timeStart || minMaxRes[len(minMaxRes)-1].CloseTime != timeEnd-1 {
		return nil, fmt.Errorf("time mismatch: %d, %d, %d, %d", minMaxRes[0].OpenTime, timeStart, minMaxRes[len(minMaxRes)-1].OpenTime, timeEnd)
	}
	if startRes[0].OpenTime != timeStart-30*60*1000 && startRes[0].OpenTime != timeStart {
		return nil, fmt.Errorf("1: open time mismatch: %d, %d", startRes[0].OpenTime, timeStart-30*60*1000)
	}
	if startRes[1].OpenTime != timeStart {
		return nil, fmt.Errorf("2: open time mismatch: %d, %d", startRes[1].OpenTime, timeStart)
	}
	if startRes[1].CloseTime != timeStart+30*60*1000-1 {
		return nil, fmt.Errorf("3: close time mismatch: %d, %d", startRes[1].CloseTime, timeStart+30*60*1000-1)
	}
	if endRes[0].OpenTime != timeEnd-30*60*1000 {
		return nil, fmt.Errorf("4: open time mismatch: %d, %d", endRes[0].OpenTime, timeEnd-30*60*1000)
	}
	if endRes[1].OpenTime != timeEnd {
		return nil, fmt.Errorf("5: open time mismatch: %d, %d", endRes[1].OpenTime, timeEnd)
	}
	if endRes[1].CloseTime != timeEnd+30*60*1000-1 {
		return nil, fmt.Errorf("6: close time mismatch: %d, %d", endRes[1].CloseTime, timeEnd+30*60*1000-1)
	}
	metrics.StartPrice30MinBefore, err = utils.ParseFloatPointer(startRes[0].Open) // start time - 30 minutes
	if err != nil {
		return nil, err
	}
	metrics.StartPriceExact, err = utils.ParseFloatPointer(startRes[1].Open) // start time
	if err != nil {
		return nil, err
	}
	metrics.StartPrice, err = utils.ParseFloatPointer(startRes[1].Close) // start time + 30 minutes
	if err != nil {
		return nil, err
	}
	metrics.EndPrice30MinBefore, err = utils.ParseFloatPointer(endRes[0].Open) // end time - 30 minutes
	if err != nil {
		return nil, err
	}
	metrics.EndPriceExact, err = utils.ParseFloatPointer(endRes[1].Open) // end time
	if err != nil {
		return nil, err
	}
	metrics.EndPrice, err = utils.ParseFloatPointer(endRes[1].Close) // end time - 30 minutes
	if err != nil {
		return nil, err
	}
	startT := time.Unix(timeStart/1000, 0)
	endT := time.Unix(timeEnd/1000, 0)
	metrics.StartTime = &startT
	metrics.EndTime = &endT
	merged := make([]*futures.Kline, 0)
	merged = append(merged, startRes...)
	merged = append(merged, endRes...)
	merged = append(merged, minMaxRes...)
	for _, k := range merged {
		high, err := strconv.ParseFloat(k.High, 64)
		if err != nil {
			return nil, err
		}
		low, err := strconv.ParseFloat(k.Low, 64)
		if err != nil {
			return nil, err
		}
		if metrics.HighPrice == nil || high > *metrics.HighPrice {
			metrics.HighPrice = &high
		}
		if metrics.LowPrice == nil || low < *metrics.LowPrice {
			metrics.LowPrice = &low
		}
	}
	return metrics, nil
}

func RefreshTheChosen() error {
	_, err := sql.GetDB().Exec(context.Background(), `REFRESH MATERIALIZED VIEW bts.TheChosen`)
	return err
}

func PopulatePrices() error {
	strategies := make([]*UserStrategy, 0)
	err := sql.GetDB().Scan(&strategies, `WITH Pool AS (
    SELECT * FROM bts.strategy WHERE concluded=true AND high_price IS NULL
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
          p.user_id, p.rois_fetched_at, p.type, p.lower_limit, p.upper_limit,
          p.grid_count, p.trigger_price, p.stop_lower_limit, p.stop_upper_limit, p.base_asset, p.quote_asset,
          p.leverage, p.trailing_down, p.trailing_up, p.trailing_type, p.latest_matched_count, p.matched_count, p.min_investment,
          p.concluded
FROM FilteredStrategies f JOIN Pool p ON f.strategy_id = p.strategy_id WHERE f.original_input IS NOT NULL AND f.runtime < 5400000;`)
	if err != nil {
		return err
	}
	discord.Infof("Populating prices for %d strategies", len(strategies))
	counter := 0
	for _, s := range strategies {
		metrics, err := GetPrices(s.Symbol,
			s.StartTime.UnixMilli(), s.EndTime.UnixMilli())
		if err != nil {
			if strings.Contains(err.Error(), "Too many requests") {
				discord.Errorf("Too many requests: %v", err)
				break
			} else {
				discord.Errorf("Error fetching prices %d: %v", s.StrategyID, err)
				continue
			}
		}
		_, err = sql.GetDB().Exec(context.Background(), `UPDATE bts.strategy SET 
                        start_price = $1, end_price = $2,
                        start_time = $3, end_time = $4,
                        start_price_exact = $5, end_price_exact = $6,
                        low_price = $7, high_price = $8,
                        start_price_30m_before = $9, end_price_30m_before = $10
                        WHERE strategy_id = $11`,
			metrics.StartPrice, metrics.EndPrice,
			metrics.StartTime, metrics.EndTime,
			metrics.StartPriceExact, metrics.EndPriceExact,
			metrics.LowPrice, metrics.HighPrice,
			metrics.StartPrice30MinBefore, metrics.EndPrice30MinBefore,
			s.StrategyID)
		if err != nil {
			break
		}
		counter++
		if counter > 200 {
			break
		}
	}
	discord.Infof("Populated prices for %d strategies", counter)
	return err
}

func PopulateRoi() error {
	strategies := make([]*StrategyDB, 0)
	err := sql.GetDB().Scan(&strategies, `SELECT
    s.*
FROM
    bts.strategy s
WHERE
    (s.concluded = FALSE OR s.concluded IS NULL)
  AND
    strategy_type = 2
  AND rois_fetched_at <= NOW() - INTERVAL '45 minutes'
    ORDER BY s.rois_fetched_at, s.time_discovered;`)
	if err != nil {
		return err
	}
	discord.Infof("Populating roi for %d strategies", len(strategies))
	if len(strategies) > 0 {
		discord.Infof("Earliest strategy: %s", strategies[0].TimeDiscovered)
	}
	concludedCount := 0
	fetchedCount := 0
	populatedStrategies := make(map[int64]*StrategyDB)
	rRows := make([][]interface{}, 0)
	for _, s := range strategies {
		log.Info("Fetching Roi: ", s.StrategyID)
		rois, err := getStrategyRois(s.StrategyID, s.UserID)
		if err != nil {
			discord.Errorf("Error fetching roi: %v", err)
			break
		}
		s.rois = rois
		s.RoisFetchedAt = time.Now()
		populatedStrategies[s.StrategyID] = s
		fetchedCount++
		for _, r := range s.rois {
			rRows = append(rRows, []interface{}{s.StrategyID,
				r.Roi,
				r.Pnl,
				time.Unix(r.Time, 0)})
		}
	}
	err = sql.SimpleTransaction(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `CREATE TEMPORARY TABLE _temp_roi (LIKE bts.roi INCLUDING ALL) ON COMMIT DROP`)
		if err != nil {
			return err
		}
		rows, err := tx.CopyFrom(context.Background(), pgx.Identifier{"_temp_roi"},
			roiColumns, pgx.CopyFromRows(rRows))
		if err != nil {
			return err
		}
		discord.Infof("Inserted %d rois", rows)
		_, err = tx.Exec(context.Background(), `INSERT INTO bts.roi (strategy_id, roi, pnl, time) SELECT * FROM _temp_roi ON CONFLICT DO NOTHING`)
		if err != nil {
			return err
		}
		for _, s := range populatedStrategies {
			_, err = tx.Exec(context.Background(),
				`UPDATE bts.strategy SET rois_fetched_at = $1 WHERE strategy_id = $2`,
				s.RoisFetchedAt,
				s.StrategyID,
			)
			if err != nil {
				return err
			}
			rois := s.rois
			if len(rois) != 0 && s.RoisFetchedAt.Sub(time.Unix(rois[0].Time, 0)) > 130*time.Minute {
				// concluded: if no new roi fetched in 2 hours
				_, err := tx.Exec(context.Background(),
					`UPDATE bts.strategy SET concluded = $1 WHERE strategy_id = $2`,
					true,
					s.StrategyID,
				)
				if err != nil {
					return err
				}
				log.Infof("Concluded: %d", s.StrategyID)
				concludedCount++
			}
		}
		return nil
	})
	discord.Infof("Concluded %d strategies, Fetched %d strategies", concludedCount, fetchedCount)
	return err
}

var roiColumns = []string{
	"strategy_id",
	"roi",
	"pnl",
	"time",
}

var strategyColumns = []string{
	"symbol",
	"copy_count",
	"roi",
	"pnl",
	"running_time",
	"strategy_id",
	"strategy_type",
	"direction",
	"user_id",
	"time_discovered",
	"rois_fetched_at",
	"type",
	"lower_limit",
	"upper_limit",
	"grid_count",
	"trigger_price",
	"stop_lower_limit",
	"stop_upper_limit",
	"base_asset",
	"quote_asset",
	"leverage",
	"trailing_up",
	"trailing_down",
	"trailing_type",
	"latest_matched_count",
	"matched_count",
	"min_investment",
}

var userColumns = []string{
	"user_id",
}

var strategyCL = `symbol, copy_count, roi, pnl,
     running_time, strategy_id, strategy_type, direction,
     user_id, time_discovered, rois_fetched_at,
     type, lower_limit, upper_limit, grid_count,
     trigger_price, stop_lower_limit, stop_upper_limit, base_asset,
     quote_asset, leverage, trailing_up, trailing_down,
     trailing_type, latest_matched_count, matched_count, min_investment`

func addToRankingStore(ss Strategies) error {
	sRows := make([][]interface{}, 0)
	uRows := make([][]interface{}, 0)
	users := mapset.NewSet[int]()
	for _, s := range ss {
		users.Add(s.UserID)
		s.TimeDiscovered = time.Now()
		sRows = append(sRows, []interface{}{
			s.Symbol,
			s.CopyCount,
			s.Roi,
			s.Pnl,
			s.RunningTime,
			s.SID,
			s.StrategyType,
			s.Direction,
			s.UserID,
			s.TimeDiscovered,
			s.RoisFetchedAt,
			s.StrategyParams.Type,
			s.StrategyParams.LowerLimit,
			s.StrategyParams.UpperLimit,
			s.StrategyParams.GridCount,
			s.StrategyParams.TriggerPrice,
			s.StrategyParams.StopLowerLimit,
			s.StrategyParams.StopUpperLimit,
			s.StrategyParams.BaseAsset,
			s.StrategyParams.QuoteAsset,
			s.StrategyParams.Leverage,
			s.StrategyParams.TrailingUp,
			s.StrategyParams.TrailingDown,
			s.TrailingType,
			s.LatestMatchedCount,
			s.MatchedCount,
			s.MinInvestment,
		})
	}
	for u := range users.Iter() {
		uRows = append(uRows, []interface{}{u})
	}
	return sql.SimpleTransaction(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `CREATE TEMPORARY TABLE _temp_b_users (LIKE bts.b_user INCLUDING ALL) ON COMMIT DROP`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(context.Background(), `CREATE TEMPORARY TABLE _temp_strategies (LIKE bts.strategy INCLUDING ALL) ON COMMIT DROP`)
		if err != nil {
			return err
		}
		rows, err := tx.CopyFrom(context.Background(), pgx.Identifier{"_temp_b_users"},
			userColumns, pgx.CopyFromRows(uRows))
		if err != nil {
			return err
		}
		discord.Infof("Inserted %d users", rows)
		rows, err = tx.CopyFrom(context.Background(), pgx.Identifier{"_temp_strategies"},
			strategyColumns, pgx.CopyFromRows(sRows))
		if err != nil {
			return err
		}
		discord.Infof("Inserted %d strategies", rows)
		_, err = tx.Exec(context.Background(), `INSERT INTO bts.b_user (user_id) SELECT user_id FROM _temp_b_users ON CONFLICT DO NOTHING`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(context.Background(), `INSERT INTO bts.strategy 
    (`+strategyCL+`)
SELECT `+strategyCL+` FROM _temp_strategies ON CONFLICT (strategy_id) DO UPDATE SET
  (copy_count,
            roi,
            pnl,
            running_time,
            direction,
            lower_limit,
            upper_limit,
            grid_count,
            trigger_price,
            stop_lower_limit,
            stop_upper_limit,
            base_asset,
            quote_asset,
            leverage,
            trailing_up,
            trailing_down,
            trailing_type,
            latest_matched_count,
            matched_count,
            min_investment) = (excluded.copy_count,
            excluded.roi,
            excluded.pnl,
            excluded.running_time,
            excluded.direction,
            excluded.lower_limit,
            excluded.upper_limit,
            excluded.grid_count,
            excluded.trigger_price,
            excluded.stop_lower_limit,
            excluded.stop_upper_limit,
            excluded.base_asset,
            excluded.quote_asset,
            excluded.leverage,
            excluded.trailing_up,
            excluded.trailing_down,
            excluded.trailing_type,
            excluded.latest_matched_count,
            excluded.matched_count,
            excluded.min_investment)`)
		if err != nil {
			return err
		}
		return nil
	})
}
