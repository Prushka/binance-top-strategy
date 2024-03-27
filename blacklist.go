package main

import (
	"fmt"
	"time"
)

type Blacklist struct {
	BySID             map[int]time.Time
	BySymbolDirection map[string]time.Time
}

var blacklist = &Blacklist{BySID: make(map[int]time.Time), BySymbolDirection: make(map[string]time.Time)}

func addSymbolDirectionToBlacklist(symbol, direction string, d time.Duration) {
	blacklist.BySymbolDirection[symbol+direction] = time.Now().Add(d)
	DiscordWebhook(fmt.Sprintf("**Add blacklist:** %s, %s, %s", symbol, direction, d))
}

func SymbolDirectionBlacklisted(symbol, direction string) (bool, time.Time) {
	if t, ok := blacklist.BySymbolDirection[symbol+direction]; ok {
		if time.Now().Before(t) {
			return true, t
		} else {
			delete(blacklist.BySymbolDirection, symbol+direction)
		}
	}
	return false, time.Time{}
}

func addSIDToBlacklist(id int, d time.Duration) {
	blacklist.BySID[id] = time.Now().Add(d)
	DiscordWebhook(fmt.Sprintf("**Add blacklist:** %d, %s", id, d))
}

func SIDBlacklisted(id int) (bool, time.Time) {
	if t, ok := blacklist.BySID[id]; ok {
		if time.Now().Before(t) {
			return true, t
		} else {
			delete(blacklist.BySID, id)
		}
	}
	return false, time.Time{}
}
