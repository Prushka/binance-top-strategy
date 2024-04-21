package gsp

import (
	"sort"
	"time"
)

type RankingStore struct {
	StrategiesByUser map[int]map[int]*Strategy
}

var TheRankingStore = &RankingStore{StrategiesByUser: make(map[int]map[int]*Strategy)}

func (s Strategy) addToRankingStore() {
	if _, ok := TheRankingStore.StrategiesByUser[s.UserID]; !ok {
		TheRankingStore.StrategiesByUser[s.UserID] = make(map[int]*Strategy)
	}
	ori, ok := TheRankingStore.StrategiesByUser[s.UserID][s.SID]
	if !ok {
		s.TimeDiscovered = time.Now()
	} else {
		s.TimeDiscovered = ori.TimeDiscovered
	}
	TheRankingStore.StrategiesByUser[s.UserID][s.SID] = &s
}

func (s Strategy) removeFromRankingStore() {
	ori, ok := TheRankingStore.StrategiesByUser[s.UserID][s.SID]
	if ok {
		ori.TimeNotFound = time.Now()
	}
}

type UserMetrics struct {
	UserId   int
	TotalRoi float64
}

func Elect() []UserMetrics {
	selected := make([]UserMetrics, 0)
	for user, strats := range TheRankingStore.StrategiesByUser {
		score := 0.0
		for _, s := range strats {
			if s.TimeNotFound.IsZero() {
				continue
			}
			if s.Roi < 0 {
				score = -1
				break
			} else {
				score += s.Roi
			}
		}
		if score > 0 {
			selected = append(selected, UserMetrics{UserId: user, TotalRoi: score})
		}
	}
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].TotalRoi > selected[j].TotalRoi
	})
	return selected
}
