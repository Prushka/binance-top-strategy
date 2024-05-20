package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sql"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type StrategyDB struct {
	Symbol             string     `db:"symbol"`
	CopyCount          int        `db:"copy_count"`
	ROI                float64    `db:"roi"`
	PNL                float64    `db:"pnl"`
	RunningTime        int        `db:"running_time"`
	StrategyID         int64      `db:"strategy_id"` // Use int64 for BIGINT
	StrategyType       int        `db:"strategy_type"`
	Direction          int        `db:"direction"`
	UserID             int64      `db:"user_id"` // Use int64 for BIGINT
	PriceDifference    float64    `db:"price_difference"`
	TimeDiscovered     time.Time  `db:"time_discovered"`
	TimeNotFound       *time.Time `db:"time_not_found"`
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
	LatestRoi          *float64   `db:"latest_roi"`
	LatestPnl          *float64   `db:"latest_pnl"`
	LatestTime         *time.Time `db:"latest_roi_time"`
}

func PopulateRoi() error {
	strategies := make([]*StrategyDB, 0)
	err := sql.GetDB().Scan(&strategies, `SELECT
    s.*,
    r.roi as latest_roi,
    r.pnl as latest_pnl,
    r.time as latest_roi_time
FROM
    bts.strategy s
        LEFT JOIN (
        SELECT
            roi.strategy_id,
            roi.roi,
            roi.pnl,
            roi.time,
            ROW_NUMBER() OVER (PARTITION BY roi.strategy_id ORDER BY roi.time DESC) AS rn
        FROM
            bts.roi
    ) r ON s.strategy_id = r.strategy_id AND r.rn = 1
WHERE
    (s.concluded = FALSE OR s.concluded IS NULL) ORDER BY s.time_discovered;`)
	if err != nil {
		return err
	}
	discord.Infof("Populating roi for %d strategies", len(strategies))
	for _, s := range strategies {
		if time.Now().Sub(s.RoisFetchedAt) > 30*time.Minute {
			err = sql.SimpleTransaction(func(tx pgx.Tx) error {
				log.Info("Fetching Roi: ", s.StrategyID)
				rois, err := getStrategyRois(s.StrategyID, s.UserID)
				if err != nil {
					return err
				}
				s.RoisFetchedAt = time.Now()
				for _, r := range rois {
					_, err := tx.Exec(context.Background(),
						`INSERT INTO bts.roi (
				root_user_id,
				strategy_id,
				roi,
				pnl,
				time
			 ) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
						r.RootUserID,
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
					_, err := tx.Exec(context.Background(),
						`UPDATE bts.strategy SET concluded = $1 WHERE strategy_id = $2`,
						true,
						s.StrategyID,
					)
					if err != nil {
						return err
					}
					discord.Infof("Concluded: %d", s.StrategyID)
				}
				return nil
			})
		}
		if err != nil && strings.Contains(err.Error(), "unexpected end of JSON input") {
			break
		}
	}
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
            time_not_found,
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
         ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29) ON CONFLICT DO NOTHING`,
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
			s.TimeNotFound,
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
