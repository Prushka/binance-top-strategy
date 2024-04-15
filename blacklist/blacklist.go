package blacklist

import (
	"BinanceTopStrategies/discord"
	"fmt"
	"time"
)

type blacklists struct {
	BySID             map[int]*content
	BySymbolDirection map[string]*content
	BySymbol          map[string]*content
	Global            *content
}

type content struct {
	Till   time.Time
	Reason string
}

var TheBlacklist = &blacklists{BySID: make(map[int]*content), BySymbolDirection: make(map[string]*content), BySymbol: make(map[string]*content)}

func BlockTrading(d time.Duration, reason string) {
	TheBlacklist.Global = newContent(TheBlacklist.Global, d, reason)
	discord.Blacklistf(fmt.Sprintf("**Global block:** %s, %s", d, reason))
}

func AddSymbolDirection(symbol, direction string, d time.Duration, reason string) {
	TheBlacklist.BySymbolDirection[symbol+direction] = newContent(TheBlacklist.BySymbolDirection[symbol+direction], d, reason)
	discord.Blacklistf(fmt.Sprintf("**Add blacklist:** %s, %s, %s, %s", symbol, direction, d, reason))
}

func AddSID(id int, d time.Duration, reason string) {
	TheBlacklist.BySID[id] = newContent(TheBlacklist.BySID[id], d, reason)
	discord.Blacklistf(fmt.Sprintf("**Add blacklist:** %d, %s, %s", id, d, reason))
}

func AddSymbol(symbol string, d time.Duration, reason string) {
	TheBlacklist.BySymbol[symbol] = newContent(TheBlacklist.BySymbol[symbol], d, reason)
	discord.Blacklistf(fmt.Sprintf("**Add blacklist:** %s, %s, %s", symbol, d, reason))
}

func newContent(c *content, d time.Duration, reason string) *content {
	curr := time.Now().Add(d)
	if c != nil {
		if curr.Before(c.Till) {
			curr = c.Till
		}
	}
	return &content{Till: curr, Reason: reason}
}

func IsTradingBlocked() (bool, time.Time) {
	if TheBlacklist.Global != nil {
		if time.Now().Before(TheBlacklist.Global.Till) {
			return true, TheBlacklist.Global.Till
		} else {
			TheBlacklist.Global = nil
		}
	}
	return false, time.Time{}
}

func SymbolDirectionBlacklisted(symbol, direction string) (bool, time.Time) {
	if t, ok := TheBlacklist.BySymbolDirection[symbol+direction]; ok {
		if time.Now().Before(t.Till) {
			return true, t.Till
		} else {
			delete(TheBlacklist.BySymbolDirection, symbol+direction)
		}
	}
	return false, time.Time{}
}

func SIDBlacklisted(id int) (bool, time.Time) {
	if t, ok := TheBlacklist.BySID[id]; ok {
		if time.Now().Before(t.Till) {
			return true, t.Till
		} else {
			delete(TheBlacklist.BySID, id)
		}
	}
	return false, time.Time{}
}

func SymbolBlacklisted(symbol string) (bool, time.Time) {
	if t, ok := TheBlacklist.BySymbol[symbol]; ok {
		if time.Now().Before(t.Till) {
			return true, t.Till
		} else {
			delete(TheBlacklist.BySymbol, symbol)
		}
	}
	return false, time.Time{}
}
