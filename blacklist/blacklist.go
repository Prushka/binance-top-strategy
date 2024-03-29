package blacklist

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/persistence"
	"fmt"
	log "github.com/sirupsen/logrus"
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

func AddSymbolDirection(symbol, direction string, d time.Duration, reason string) {
	blacklist.BySymbolDirection[symbol+direction] = &Blacklist{Till: time.Now().Add(d), Reason: reason}
	discord.Infof(fmt.Sprintf("**Add blacklist:** %s, %s, %s, %s", symbol, direction, d, reason))
	persistBlacklist()
}

func AddSID(id int, d time.Duration, reason string) {
	blacklist.BySID[id] = &Blacklist{Till: time.Now().Add(d), Reason: reason}
	discord.Infof(fmt.Sprintf("**Add blacklist:** %d, %s, %s", id, d, reason), discord.DefaultWebhook)
	persistBlacklist()
}

func AddSymbol(symbol string, d time.Duration, reason string) {
	blacklist.BySymbol[symbol] = &Blacklist{Till: time.Now().Add(d), Reason: reason}
	discord.Infof(fmt.Sprintf("**Add blacklist:** %s, %s, %s", symbol, d, reason), discord.DefaultWebhook)
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
