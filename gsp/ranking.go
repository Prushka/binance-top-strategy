package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sql"
	"BinanceTopStrategies/utils"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
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
	UserTotalRoi   float64 `db:"total_roi"`
	UserInput      float64 `db:"original_input"`
	UserTotalInput float64 `db:"total_original_input"`
	UserStrategies int     `db:"strategy_count"`
}

type StrategyDB struct {
	Symbol             string    `db:"symbol"`
	CopyCount          int       `db:"copy_count"`
	ROI                float64   `db:"roi"`
	PNL                float64   `db:"pnl"`
	RunningTime        int       `db:"running_time"`
	StrategyID         int64     `db:"strategy_id"` // Use int64 for BIGINT
	StrategyType       int       `db:"strategy_type"`
	Direction          int       `db:"direction"`
	UserID             int64     `db:"user_id"` // Use int64 for BIGINT
	PriceDifference    float64   `db:"price_difference"`
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

func (db *StrategyDB) ToStrategy() *Strategy {
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
  AND rois_fetched_at <= NOW() - INTERVAL '35 minutes'
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
	for _, s := range strategies {
		err = sql.SimpleTransaction(func(tx pgx.Tx) error {
			log.Info("Fetching Roi: ", s.StrategyID)
			utils.ResetTime()
			rois, err := getStrategyRois(s.StrategyID, s.UserID)
			if err != nil {
				return err
			}
			s.RoisFetchedAt = time.Now()
			for _, r := range rois {
				_, err := tx.Exec(context.Background(),
					`INSERT INTO bts.roi (
				strategy_id,
				roi,
				pnl,
				time
			 ) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
					s.StrategyID,
					r.Roi,
					r.Pnl,
					time.Unix(r.Time, 0),
				)
				if err != nil {
					return err
				}
			}
			_, err = tx.Exec(context.Background(),
				`UPDATE bts.strategy SET rois_fetched_at = $1 WHERE strategy_id = $2`,
				s.RoisFetchedAt,
				s.StrategyID,
			)
			if err != nil {
				return err
			}
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
			fetchedCount++
			return nil
		})
		if err != nil && strings.Contains(err.Error(), "unexpected end of JSON input") {
			break
		}
	}
	discord.Infof("Concluded %d strategies, Fetched %d strategies", concludedCount, fetchedCount)
	return err
}

func (s *Strategy) addToRankingStore() error {
	return sql.SimpleTransaction(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO bts.b_user (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
			s.UserID)
		if err != nil {
			return err
		}
		s.TimeDiscovered = time.Now()
		_, err = tx.Exec(context.Background(),
			`INSERT INTO bts.strategy (
            symbol,
            copy_count,
            roi,
            pnl,
            running_time,
            strategy_id,
            strategy_type,
            direction,
            user_id,
            price_difference,
            time_discovered,
            rois_fetched_at,
            type,
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
            min_investment
         ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28) ON CONFLICT DO NOTHING`,
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
		)
		return err
	})
}

type UserMetrics struct {
	UserId       int
	Strategies   map[int]*Strategy `json:"-"`
	NegativeOnes int
	TotalRois    float64
	TotalPnl     float64
	MinRuntime   time.Duration
	MaxRuntime   time.Duration
}

func (u UserMetrics) String() string {
	return fmt.Sprintf("UserId: %d, NegativeOnes: %d, Rois: %.2f, Pnl: %.2f, TotalStrategies: %d, MinRuntime: %s, MaxRuntime: %s",
		u.UserId, u.NegativeOnes, u.TotalRois*100, u.TotalPnl, len(u.Strategies), u.MinRuntime, u.MaxRuntime,
	)
}

// use roi and pnl from the latest roi and pnl in roi table, as roi and pnl in strategy table may not be up to date
