package gsp

import (
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"encoding/json"
	"fmt"
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
