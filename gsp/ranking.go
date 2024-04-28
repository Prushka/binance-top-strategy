package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sql"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (s *Strategy) addToRankingStore() error {
	path := fmt.Sprintf(config.TheConfig.DataFolder+"/strategies/%d.json", s.SID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		s.TimeDiscovered = time.Now()
		err := s.saveToStore()
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Strategy) loadFromFile(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, s)
	if err != nil {
		return err
	}
	return nil
}

func (s *Strategy) saveToStore() error {
	path := fmt.Sprintf(config.TheConfig.DataFolder+"/strategies/%d.json", s.SID)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	err = os.WriteFile(path, b, 0666)
	if err != nil {
		return err
	}
	return nil
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

func ToSQL() error {
	root := config.TheConfig.DataFolder + "/strategies"

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			s := &Strategy{}
			err = s.loadFromFile(path)
			if err != nil {
				return err
			}
			s.Sanitize()
			log.Infof("%d, %s", s.SID, s.Symbol)
			mErr, err := sql.SimpleTransaction(func(tx pgx.Tx) error {
				_, err := tx.Exec(context.Background(),
					`INSERT INTO bts.b_user (user_id) VALUES ($1) ON CONFLICT DO NOTHING`,
					s.UserID)
				if err != nil {
					return err
				}
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
				if err != nil {
					return err
				}
				if len(s.Rois) == 0 && !s.RoisFetchedAt.IsZero() && time.Now().Sub(s.RoisFetchedAt) < 100*time.Minute {
					return nil
				}
				if len(s.Rois) == 0 || s.RoisFetchedAt.Sub(time.Unix(s.Rois[0].Time, 0)) < 100*time.Minute {
					log.Info("Fetching Roi: ", s.SID)
					err = s.populateRois()
					if err != nil {
						return err
					}
					s.RoisFetchedAt = time.Now()
					err = s.saveToStore()
					if err != nil {
						return err
					}
					_, err := tx.Exec(context.Background(),
						`UPDATE bts.strategy SET rois_fetched_at = $1 WHERE strategy_id = $2`,
						s.RoisFetchedAt,
						s.SID,
					)
					if err != nil {
						return err
					}
				}

				for _, r := range s.Rois {
					_, err := tx.Exec(context.Background(),
						`INSERT INTO bts.roi (
				root_user_id,
				strategy_id,
				roi,
				pnl,
				time
			 ) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
						r.RootUserID,
						s.SID,
						r.Roi,
						r.Pnl,
						time.Unix(r.Time, 0),
					)
					if err != nil {
						return err
					}
				}
				return nil
			})
			if mErr.ToError() != nil {
				return mErr.ToError()
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func Elect() ([]UserMetrics, error) {
	root := config.TheConfig.DataFolder + "/strategies"

	byUser := make(map[int]*UserMetrics)
	count := 0
	runningOnes := 0
	negativeOnes := 0
	timeFoundMin := 99
	timeFoundMax := 0
	withNoRois := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".json" {
			count++
			s := &Strategy{}
			err = s.loadFromFile(path)
			if err != nil {
				return err
			}
			if len(s.Rois) == 0 && !s.RoisFetchedAt.IsZero() {
				withNoRois++
				return nil
			}
			lastestTime := time.Unix(s.Rois[0].Time, 0)
			if len(s.Rois) == 0 || s.RoisFetchedAt.Sub(lastestTime) < 100*time.Minute {
				log.Info("No rois for strategy: ", s.SID)
				err = s.populateRois()
				if err != nil {
					return err
				}
				s.RoisFetchedAt = time.Now()
				err = s.saveToStore()
				if err != nil {
					return err
				}
			}
			if len(s.Rois) == 0 && !s.RoisFetchedAt.IsZero() {
				withNoRois++
				return nil
			}
			timeFound := s.TimeDiscovered.Minute()
			if timeFound < timeFoundMin {
				timeFoundMin = timeFound
			}
			if timeFound > timeFoundMax {
				timeFoundMax = timeFound
			}
			if s.isRunning() {
				runningOnes++
				return nil
			}
			s.Sanitize()
			if _, ok := byUser[s.UserID]; !ok {
				byUser[s.UserID] = &UserMetrics{
					UserId:     s.UserID,
					Strategies: make(map[int]*Strategy),
					MinRuntime: 99 * time.Hour,
				}
			}
			metrics := byUser[s.UserID]
			metrics.Strategies[s.SID] = s
			if s.Roi < 0 {
				metrics.NegativeOnes++
			}
			metrics.TotalRois += s.Roi
			metrics.TotalPnl += s.Pnl
			r := time.Duration(s.RunningTime) * time.Second
			if metrics.MinRuntime > r {
				metrics.MinRuntime = r
			}
			if metrics.MaxRuntime < r {
				metrics.MaxRuntime = r
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	selected := make([]UserMetrics, 0)
	for _, v := range byUser {
		if v.NegativeOnes <= 0 {
			selected = append(selected, *v)
		}
	}
	discord.Infof("Total strategies: %d, running: %d, negative: %d, users: %d, timeFound: %d/%d, withNoRois: %d",
		count, runningOnes, negativeOnes, len(byUser), timeFoundMin, timeFoundMax, withNoRois)
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].TotalPnl > selected[j].TotalPnl
	})
	return selected[:10], nil
}
