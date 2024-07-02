package blacklist

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sql"
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"time"
)

func writeKey(key string, d time.Duration, reason string) {
	err := sql.SimpleTransaction(func(tx pgx.Tx) error {
		till := time.Now().Add(d)
		_, err := tx.Exec(context.Background(),
			`INSERT INTO bts.blacklist (key, till, reason) VALUES ($1, $2, $3) ON CONFLICT (key) DO UPDATE
SET till = EXCLUDED.till,
    reason = EXCLUDED.reason
WHERE bts.blacklist.till < EXCLUDED.till;`,
			key, till, reason)
		return err
	})
	if err != nil {
		log.Errorf("Error inserting blacklist: %v", err)
	}
}

const (
	GLOBAL = "global_block"
)

func BlockTrading(d time.Duration, reason string) {
	writeKey(GLOBAL, d, reason)
	discord.Blacklistf(fmt.Sprintf("**Global block:** %s, %s", d, reason))
}

func AddSymbolDirection(symbol, direction string, d time.Duration, reason string) {
	writeKey(symbol+direction, d, reason)
	discord.Blacklistf(fmt.Sprintf("**Add blacklist:** %s, %s, %s, %s", symbol, direction, d, reason))
}

func AddSymbol(symbol string, d time.Duration, reason string) {
	writeKey(symbol, d, reason)
	discord.Blacklistf(fmt.Sprintf("**Add blacklist:** %s, %s, %s", symbol, d, reason))
}

type TillStruct struct {
	Till   time.Time `db:"till"`
	Key    string    `db:"key"`
	Reason string    `db:"reason"`
}

func IsTradingBlocked(symbol, direction string) (bool, time.Time) {
	till := make([]TillStruct, 0)
	err := sql.GetDB().Scan(&till, "SELECT * FROM bts.blacklist WHERE key=$1 OR key=$2 OR key=$3",
		symbol+direction, symbol, GLOBAL)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		discord.Errorf("Error scanning blacklist: %v", err)
	}
	for _, t := range till {
		if t.Till.After(time.Now()) {
			return true, t.Till
		}
	}
	return false, time.Time{}
}
