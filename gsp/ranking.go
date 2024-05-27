package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/sql"
	"context"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
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
	PNL                float64    `db:"pnl"`
	RunningTime        int        `db:"running_time"`
	StrategyID         int64      `db:"strategy_id"` // Use int64 for BIGINT
	StrategyType       int        `db:"strategy_type"`
	Direction          int        `db:"direction"`
	UserID             int64      `db:"user_id"` // Use int64 for BIGINT
	PriceDifference    float64    `db:"price_difference"`
	TimeDiscovered     time.Time  `db:"time_discovered"`
	RoisFetchedAt      time.Time  `db:"rois_fetched_at"`
	Type               string     `db:"type"`
	LowerLimit         float64    `db:"lower_limit"`
	UpperLimit         float64    `db:"upper_limit"`
	GridCount          int        `db:"grid_count"`
	TriggerPrice       *float64   `db:"trigger_price"` // Use pointer for nullable columns
	StopLowerLimit     *float64   `db:"stop_lower_limit"`
	StopUpperLimit     *float64   `db:"stop_upper_limit"`
	BaseAsset          *string    `db:"base_asset"` // Use pointer for nullable columns
	QuoteAsset         *string    `db:"quote_asset"`
	Leverage           *int       `db:"leverage"`
	TrailingUp         *bool      `db:"trailing_up"`
	TrailingDown       *bool      `db:"trailing_down"`
	TrailingType       *string    `db:"trailing_type"`
	LatestMatchedCount *int       `db:"latest_matched_count"`
	MatchedCount       *int       `db:"matched_count"`
	MinInvestment      *float64   `db:"min_investment"`
	Concluded          *bool      `db:"concluded"`
	StartTime          *time.Time `db:"start_time"`
	EndTime            *time.Time `db:"end_time"`
	StartPrice         *float64   `db:"start_price"`
	EndPrice           *float64   `db:"end_price"`
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

func PopulatePrices() error {
	strategies := make([]*UserStrategy, 0)
	err := sql.GetDB().Scan(&strategies, `WITH Pool AS (
    SELECT * FROM bts.strategy WHERE concluded=true AND (start_price IS NULL OR end_price is NULL)
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
FROM FilteredStrategies f JOIN Pool p ON f.strategy_id = p.strategy_id WHERE f.original_input IS NOT NULL;`)
	if err != nil {
		return err
	}
	discord.Infof("Populating prices for %d strategies", len(strategies))
	for _, s := range strategies {
		start, end, err := sdk.GetPrices(s.Symbol,
			s.StartTime.UnixMilli(), s.EndTime.UnixMilli())
		if err != nil {
			return err
		}
		_, err = sql.GetDB().Exec(context.Background(), `UPDATE bts.strategy SET start_price = $1, end_price = $2,
                        start_time=$3, end_time=$4 WHERE strategy_id = $5`, start, end, s.StartTime, s.EndTime, s.StrategyID)
		if err != nil {
			return err
		}
	}
	return nil
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
	"price_difference",
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
			s.PriceDifference,
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
    (symbol, copy_count, roi, pnl,
     running_time, strategy_id, strategy_type, direction,
     user_id, price_difference, time_discovered, rois_fetched_at,
     type, lower_limit, upper_limit, grid_count,
     trigger_price, stop_lower_limit, stop_upper_limit, base_asset,
     quote_asset, leverage, trailing_up, trailing_down,
     trailing_type, latest_matched_count, matched_count, min_investment,
     concluded, start_time, end_time, start_price, end_price) 
SELECT * FROM _temp_strategies ON CONFLICT (strategy_id) DO UPDATE SET
  (copy_count,
            roi,
            pnl,
            running_time,
            direction,
            price_difference,
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
            excluded.price_difference,
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
			discord.Errorf("Error inserting strategies: %v", err)
			return err
		}
		return nil
	})
}
