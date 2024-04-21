package gsp

import (
	"BinanceTopStrategies/config"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (s Strategy) addToRankingStore() {
	exists := true
	path := fmt.Sprintf(config.TheConfig.DataFolder+"/strategies/%d.json", s.SID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		exists = false
	}
	if !exists {
		s.TimeDiscovered = time.Now()
	} else {
		b, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		ori := &Strategy{}
		err = json.Unmarshal(b, ori)
		if err != nil {
			panic(err)
		}
		s.TimeDiscovered = ori.TimeDiscovered
	}

	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(path, b, 0666)
	if err != nil {
		panic(err)
	}
}

type UserMetrics struct {
	UserId   int
	TotalRoi float64
}

func Elect() []UserMetrics {
	root := config.TheConfig.DataFolder + "/strategies"
	selected := make([]UserMetrics, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			s := &Strategy{}
			err = json.Unmarshal(b, s)
			if err != nil {
				return err
			}
			score := 0.0
			if s.TimeNotFound.IsZero() || len(s.Rois) == 0 {
				return nil
			}
			latestTime := time.Unix(s.Rois[0].Time, 0)
			if time.Now().Sub(latestTime) <= 65*time.Minute {
				return nil
			}
			if s.Roi < 0 {
				score = -1
				return nil
			} else {
				score += s.Roi
			}
			if score > 0 {
				selected = append(selected, UserMetrics{UserId: s.UserID, TotalRoi: score})
			}
		}
		return nil
	})
	if err != nil {
		panic(err)

	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].TotalRoi > selected[j].TotalRoi
	})
	return selected
}
