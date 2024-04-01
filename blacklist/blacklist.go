package blacklist

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/persistence"
	"fmt"
	log "github.com/sirupsen/logrus"
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

var blacklist = &blacklists{BySID: make(map[int]*content), BySymbolDirection: make(map[string]*content), BySymbol: make(map[string]*content)}

func persistBlacklist() {
	err := persistence.Save(blacklist, persistence.BlacklistFileName)
	if err != nil {
		discord.Infof("Error saving blacklist: %v", err)
	}
}

func Init() {
	err := persistence.Load(blacklist, persistence.BlacklistFileName)
	if err != nil {
		log.Fatalf("Error loading blacklist: %v", err)
	}
}

func BlockTrading(d time.Duration, reason string) {
	blacklist.Global = newContent(blacklist.Global, d, reason)
	discord.Infof(fmt.Sprintf("**Global block:** %s, %s", d, reason))
	persistBlacklist()
}

func AddSymbolDirection(symbol, direction string, d time.Duration, reason string) {
	blacklist.BySymbolDirection[symbol+direction] = newContent(blacklist.BySymbolDirection[symbol+direction], d, reason)
	discord.Infof(fmt.Sprintf("**Add blacklist:** %s, %s, %s, %s", symbol, direction, d, reason))
	persistBlacklist()
}

func AddSID(id int, d time.Duration, reason string) {
	blacklist.BySID[id] = newContent(blacklist.BySID[id], d, reason)
	discord.Infof(fmt.Sprintf("**Add blacklist:** %d, %s, %s", id, d, reason), discord.DefaultWebhook)
	persistBlacklist()
}

func AddSymbol(symbol string, d time.Duration, reason string) {
	blacklist.BySymbol[symbol] = newContent(blacklist.BySymbol[symbol], d, reason)
	discord.Infof(fmt.Sprintf("**Add blacklist:** %s, %s, %s", symbol, d, reason), discord.DefaultWebhook)
	persistBlacklist()
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
	if blacklist.Global != nil {
		if time.Now().Before(blacklist.Global.Till) {
			return true, blacklist.Global.Till
		} else {
			blacklist.Global = nil
		}
	}
	return false, time.Time{}
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
