package main

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/gsp"
	"fmt"
	"time"
)

func testStrategy(s *gsp.Strategy) (bool, error, string) {
	if s.RunningTime > 60*220 {
		return false, nil, "Running for more than 220 minutes (db test)"
	}
	if s.Roi < 0 {
		return false, nil, "Negative RoI (db test)"
	}
	userWl, err := gsp.UserWLCache.Get(fmt.Sprintf("%d", s.UserID))
	if err != nil {
		return false, err, err.Error()
	}
	wl := userWl.DirectionWL[s.Direction]
	if wl.WinRatio < 0.8 ||
		(wl.ShortRunningRatio > 0.24 && wl.WinRatio < 0.979) ||
		wl.TotalWL < 5 {
		return false, nil, fmt.Sprintf("WL unmet %s", wl)
	}
	if time.Now().Sub(wl.EarliestTime) < 30*24*time.Hour {
		return false, nil, "User has not been active for more than 30 days"
	}
	userStrategies := gsp.GetPool().ByUID()[s.UserID]
	for _, us := range userStrategies {
		if us.Symbol == s.Symbol && us.Direction != s.Direction {
			return false, nil, "Same symbol hedging"
		}
	}
	if len(userStrategies) > 7 {
		return false, nil, fmt.Sprintf("User %d already has %d strategies, Skip", s.UserID, len(userStrategies))
	}
	discord.Infof("%s | %s", gsp.Display(s, nil, "Candidate", 0, 0), wl)
	return true, nil, ""
}
