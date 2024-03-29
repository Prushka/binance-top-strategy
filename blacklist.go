package main

import (
	"fmt"
	"time"
)

type Blacklists struct {
	BySID             map[int]*Blacklist
	BySymbolDirection map[string]*Blacklist
	BySymbol          map[string]*Blacklist
}

type Blacklist struct {
	Till   time.Time
	Reason string
}

var blacklist = &Blacklists{BySID: make(map[int]*Blacklist), BySymbolDirection: make(map[string]*Blacklist), BySymbol: make(map[string]*Blacklist)}

func persistBlacklist() {
	err := save(blacklist, BlacklistFileName)
	if err != nil {
		Discordf("Error saving blacklist: %v", err)
	}
}

func addSymbolDirectionToBlacklist(symbol, direction string, d time.Duration, reason string) {
	blacklist.BySymbolDirection[symbol+direction] = &Blacklist{Till: time.Now().Add(d), Reason: reason}
	DiscordWebhookS(fmt.Sprintf("**Add blacklist:** %s, %s, %s, %s", symbol, direction, d, reason), DefaultWebhook)
	persistBlacklist()
}

func addSIDToBlacklist(id int, d time.Duration, reason string) {
	blacklist.BySID[id] = &Blacklist{Till: time.Now().Add(d), Reason: reason}
	DiscordWebhookS(fmt.Sprintf("**Add blacklist:** %d, %s, %s", id, d, reason), DefaultWebhook)
	persistBlacklist()
}

func addSymbolToBlacklist(symbol string, d time.Duration, reason string) {
	blacklist.BySymbol[symbol] = &Blacklist{Till: time.Now().Add(d), Reason: reason}
	DiscordWebhookS(fmt.Sprintf("**Add blacklist:** %s, %s, %s", symbol, d, reason), DefaultWebhook)
	persistBlacklist()
}

func SymbolDirectionBlacklisted(symbol, direction string) (bool, time.Time) {
	if t, ok := blacklist.BySymbolDirection[symbol+direction]; ok {
		if time.Now().Before(t.Till) {
			return true, t.Till
		} else {
			delete(blacklist.BySymbolDirection, symbol+direction)
		}
	}
	return false, time.Time{}
}

func SIDBlacklisted(id int) (bool, time.Time) {
	if t, ok := blacklist.BySID[id]; ok {
		if time.Now().Before(t.Till) {
			return true, t.Till
		} else {
			delete(blacklist.BySID, id)
		}
	}
	return false, time.Time{}
}

func SymbolBlacklisted(symbol string) (bool, time.Time) {
	if t, ok := blacklist.BySymbol[symbol]; ok {
		if time.Now().Before(t.Till) {
			return true, t.Till
		} else {
			delete(blacklist.BySymbol, symbol)
		}
	}
	return false, time.Time{}
}
