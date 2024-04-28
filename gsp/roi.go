package gsp

import (
	"BinanceTopStrategies/cache"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/request"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

var RoisCache = cache.CreateMapCache[[]*Roi](
	func(key string) ([]*Roi, error) {
		split := strings.Split(key, "-")
		SID, _ := strconv.Atoi(split[0])
		UserId, _ := strconv.Atoi(split[1])
		roi, err := getStrategyRois(int64(SID), int64(UserId))
		if err != nil {
			return nil, err
		}
		return roi, nil
	},
	func(rois []*Roi) bool {
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
