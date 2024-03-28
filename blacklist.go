package main

import (
	"time"
)

type Blacklist struct {
	BySID             map[int]time.Time
	BySymbolDirection map[string]time.Time
	BySymbol          map[string]time.Time
}

var blacklist = &Blacklist{BySID: make(map[int]time.Time),
	BySymbolDirection: make(map[string]time.Time),
	BySymbol:          make(map[string]time.Time)}

func persistBlacklist() {
	err := save(blacklist, BlacklistFileName)
	if err != nil {
		Discordf("Error saving blacklist: %v", err)
	}
}

func addSymbolDirectionToBlacklist(symbol, direction string, d time.Duration) {
	blacklist.BySymbolDirection[symbol+direction] = time.Now().Add(d)
	Discordf("**Add blacklist:** %s, %s, %s", symbol, direction, d)
	persistBlacklist()
}

func addSIDToBlacklist(id int, d time.Duration) {
	blacklist.BySID[id] = time.Now().Add(d)
	Discordf("**Add blacklist:** %d, %s", id, d)
	persistBlacklist()
}

func addSymbolToBlacklist(symbol string, d time.Duration) {
	blacklist.BySymbol[symbol] = time.Now().Add(d)
	Discordf("**Add blacklist:** %s, %s", symbol, d)
	persistBlacklist()
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

func SymbolBlacklisted(symbol string) (bool, time.Time) {
	if t, ok := blacklist.BySymbol[symbol]; ok {
		if time.Now().Before(t) {
			return true, t
		} else {
			delete(blacklist.BySymbol, symbol)
		}
	}
	return false, time.Time{}
}
