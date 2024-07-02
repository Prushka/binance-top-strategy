package gsp

import (
	"BinanceTopStrategies/sql"
	"context"
	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
)

func GridMarkForRemoval(gid int, maxLoss float64, reason string) {
	err := sql.SimpleTransaction(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO bts.for_removal (gid, max_loss, reason_loss)
			VALUES ($1, $2, $3) ON CONFLICT (gid) DO UPDATE
			SET max_loss = EXCLUDED.max_loss,
			reason_loss = EXCLUDED.reason_loss
			WHERE bts.for_removal.max_loss < EXCLUDED.max_loss;`,
			gid, maxLoss, reason)
		return err
	})
	if err != nil {
		log.Errorf("Error inserting for removal: %v", err)
	}
}

func GetMaxLoss(gid int) *float64 {
	var maxLoss *float64
	err := sql.GetDB().ScanOne(maxLoss, "SELECT max_loss FROM bts.for_removal WHERE gid=$1", gid)
	if err != nil {
		return nil
	}
	return maxLoss
}
