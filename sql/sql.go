package sql

import (
	"BinanceTopStrategies/cleanup"
	"BinanceTopStrategies/config"
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/multierr"
	"context"
	"fmt"
	"github.com/georgysavva/scany/v2/pgxscan"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

type PgxDB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type DBScan interface {
	Scan(dst interface{}, query string, args ...interface{}) error
	ScanOne(dst interface{}, query string, args ...interface{}) error
}

type Pgx interface {
	PgxDB
	DBScan
}

type pgxDBImpl struct {
	db PgxDB
}

func GetDB() Pgx {
	return wrappedPgx
}

func GetDBPool() *pgxpool.Pool {
	return dbPool
}

var dbPool *pgxpool.Pool
var wrappedPgx Pgx // wrapped pgx instance

func Init() error {
	var err error
	log.Infof("Connecting to database %s", config.TheConfig.PGUrl)
	pgConfig, err := pgxpool.ParseConfig(config.TheConfig.PGUrl)
	if err != nil {
		log.Fatalf("Failed to parse database URL: %v", err)
	}
	pgConfig.MaxConnIdleTime = 40 * time.Second
	pgConfig.MaxConnLifetime = 50 * time.Second
	//pgConfig.AfterConnect = afterConnect
	dbPool, err = pgxpool.NewWithConfig(context.Background(), pgConfig)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}
	var version string
	err = dbPool.QueryRow(context.Background(), "select version()").Scan(&version)
	if err != nil {
		return fmt.Errorf("query row failed: %w", err)
	}
	log.Infof("Connected to database: %s", version)
	wrappedPgx = NewPgxDB(dbPool)
	cleanup.AddOnStopFunc(func(_ os.Signal) {
		dbPool.Close()
		log.Infof("Closed database connection")
	})
	return nil
}

func NewPgxDB(db PgxDB) Pgx {
	return &pgxDBImpl{db: db}
}

// NormalizePgNumeric Converts pgtype.Numeric to int64 or float64 or string
func NormalizePgNumeric(y interface{}) interface{} {
	if numeric, ok := y.(pgtype.Numeric); ok {
		intValue, err := numeric.Int64Value()
		if err != nil {
			floatValue, err := numeric.Float64Value()
			if err != nil {
				y, _ = numeric.MarshalJSON()
			} else {
				y = floatValue.Float64
			}
		} else {
			y = intValue.Int64
		}
	}
	return y
}

func (w *pgxDBImpl) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return w.db.QueryRow(ctx, sql, args...)
}

func (w *pgxDBImpl) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return w.db.Query(ctx, sql, args...)
}

func (w *pgxDBImpl) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return w.db.Exec(ctx, sql, args...)
}

func (w *pgxDBImpl) Scan(dst interface{}, query string, args ...interface{}) error {
	return Scan(w.db, dst, query, args...)
}

func (w *pgxDBImpl) ScanOne(dst interface{}, query string, args ...interface{}) error {
	return ScanOne(w.db, dst, query, args...)
}

func Scan(tx pgxscan.Querier, dst interface{}, query string, args ...interface{}) error {
	return pgxscan.Select(context.Background(), tx, dst, query, args...)
}

func ScanOne(tx pgxscan.Querier, dst interface{}, query string, args ...interface{}) error {
	return pgxscan.Get(context.Background(), tx, dst, query, args...)
}

func SimpleTransaction(f func(tx pgx.Tx) error) error {
	mErr, fErr := _simpleTransaction(f)
	if mErr != nil && mErr.ToError() != nil {
		mErr.Add(fErr)
		return mErr.ToError()
	}
	return fErr
}

// SimpleTransaction runs the given function in a transaction. If the function returns an error or panics, the transaction is rolled back.
func _simpleTransaction(f func(tx pgx.Tx) error) (mErr *multierr.MultiErr, fErr error) {
	mErr = multierr.NewMultiErr()
	tx, txErr := dbPool.BeginTx(context.Background(), pgx.TxOptions{})
	if txErr != nil {
		mErr.Add(txErr)
		discord.Errorf("Error starting transaction: %v", txErr)
		return
	}

	defer func() {
		if tx != nil {
			r := recover()
			if fErr != nil || r != nil {
				discord.Errorf("%v | %v", r, fErr)
				err := tx.Rollback(context.TODO())
				if err != nil {
					discord.Errorf("Error rolling back transaction: %v", err)
					mErr.Add(err)
				} else {
					log.Infof("Rolled back transaction")
				}
				if r != nil {
					panic(r)
				}
			} else {
				err := tx.Commit(context.TODO())
				if err != nil {
					discord.Errorf("Error committing transaction: %v", err)
					mErr.Add(err)
				} else {
					log.Debugf("Committed transaction")
				}
			}
		}
	}()
	fErr = f(tx)
	return
}
