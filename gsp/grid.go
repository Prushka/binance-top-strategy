package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/sql"
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"strconv"
	"time"
)

type Grid struct {
	InitialValue           float64
	LastPnl                float64
	LastRoi                float64
	LastRealizedRoi        float64
	LastRealizedPnl        float64
	BelowLowerLimit        bool
	AboveUpperLimit        bool
	Lowest                 *GridDB
	Highest                *GridDB
	GID                    int    `json:"strategyId"`
	RootUserID             int    `json:"rootUserId"`
	StrategyUserID         int    `json:"strategyUserId"`
	StrategyAccountID      int    `json:"strategyAccountId"`
	Symbol                 string `json:"symbol"`
	StrategyStatus         string `json:"strategyStatus"`
	BookTime               int64  `json:"bookTime"`
	TriggerTime            int64  `json:"triggerTime"`
	UpdateTime             int64  `json:"updateTime"`
	GridInitialValue       string `json:"gridInitialValue"`
	InitialLeverage        int    `json:"initialLeverage"`
	GridProfit             string `json:"gridProfit"`
	Direction              string `json:"direction"`
	MatchedPnl             string `json:"matchedPnl"`
	UnmatchedAvgPrice      string `json:"unmatchedAvgPrice"`
	UnmatchedQty           string `json:"unmatchedQty"`
	UnmatchedFee           string `json:"unmatchedFee"`
	GridEntryPrice         string `json:"gridEntryPrice"`
	GridPosition           string `json:"gridPosition"`
	Version                int    `json:"version"`
	CopyCount              int    `json:"copyCount"`
	SID                    int    `json:"copiedStrategyId"`
	Sharing                bool   `json:"sharing"`
	IsSubAccount           bool   `json:"isSubAccount"`
	StrategyAmount         string `json:"strategyAmount"`
	TrailingUp             bool   `json:"trailingUp"`
	TrailingDown           bool   `json:"trailingDown"`
	TrailingStopUpperLimit bool   `json:"trailingStopUpperLimit"`
	TrailingStopLowerLimit bool   `json:"trailingStopLowerLimit"`
	OrderCurrency          string `json:"orderCurrency"`
	GridUpperLimit         string `json:"gridUpperLimit"`
	GridLowerLimit         string `json:"gridLowerLimit"`
	MatchedCount           int    `json:"matchedCount"`
	GridCount              int    `json:"gridCount"`
	PerGridQuoteQty        string `json:"perGridQuoteQty"`
	PerGridQty             string `json:"perGridQty"`
	FundingFee             string `json:"fundingFee"`
	MarginType             string `json:"marginType"`
}

type GridDB struct {
	GID         int       `db:"gid"`
	Roi         float64   `db:"roi"`
	RealizedRoi float64   `db:"realized_roi"`
	Time        time.Time `db:"time"`
}

func (grid *Grid) sanitize() {
	initial, _ := strconv.ParseFloat(grid.GridInitialValue, 64)
	grid.LastRealizedPnl, _ = strconv.ParseFloat(grid.GridProfit, 64)
	fundingFee, _ := strconv.ParseFloat(grid.FundingFee, 64)
	position, _ := strconv.ParseFloat(grid.GridPosition, 64)
	entryPrice, _ := strconv.ParseFloat(grid.GridEntryPrice, 64)
	marketPrice, _ := sdk.GetSessionSymbolPrice(grid.Symbol)
	lowerLimit, _ := strconv.ParseFloat(grid.GridLowerLimit, 64)
	upperLimit, _ := strconv.ParseFloat(grid.GridUpperLimit, 64)
	grid.BelowLowerLimit = marketPrice < lowerLimit
	grid.AboveUpperLimit = marketPrice > upperLimit
	grid.InitialValue = initial / float64(grid.InitialLeverage)
	grid.LastPnl = grid.LastRealizedPnl + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
	grid.LastRoi = grid.LastPnl / grid.InitialValue
	grid.LastRealizedRoi = grid.LastRealizedPnl / grid.InitialValue
	err := sql.SimpleTransaction(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO bts.grid (gid, roi, realized_roi, time) VALUES ($1, $2, $3, $4)`,
			grid.GID, grid.LastRoi, grid.LastRealizedRoi, time.Now())
		return err
	})
	if err != nil {
		discord.Errorf("Error inserting grid: %v", err)
	}
	grid.Lowest, grid.Highest = grid.GetLH()
}

func (grid *Grid) GetLocalWithin(duration time.Duration) (*GridDB, *GridDB) {
	earliest := time.Now().Add(-duration)
	lowest := &GridDB{}
	highest := &GridDB{}
	err := sql.GetDB().ScanOne(lowest, `SELECT * FROM bts.grid WHERE gid = $1 AND time >= $2 ORDER BY roi LIMIT 1`, grid.GID, earliest)
	if err != nil {
		discord.Errorf("Error getting lowest roi: %v", err)
	}
	err = sql.GetDB().ScanOne(highest, `SELECT * FROM bts.grid WHERE gid = $1 AND time >= $2 ORDER BY roi DESC LIMIT 1`, grid.GID, earliest)
	if err != nil {
		discord.Errorf("Error getting highest roi: %v", err)
	}
	return lowest, highest
}

func (grid *Grid) GetLH() (*GridDB, *GridDB) {
	highest := &GridDB{}
	lowest := &GridDB{}
	err := sql.GetDB().ScanOne(highest, `SELECT * FROM bts.grid WHERE gid = $1 ORDER BY roi DESC LIMIT 1`, grid.GID)
	if err != nil {
		discord.Errorf("Error getting highest roi: %v", err)
	}
	err = sql.GetDB().ScanOne(lowest, `SELECT * FROM bts.grid WHERE gid = $1 ORDER BY roi LIMIT 1`, grid.GID)
	if err != nil {
		discord.Errorf("Error getting lowest roi: %v", err)
	}
	return lowest, highest
}

func (grid *Grid) IsQuote(quote string) bool {
	token := grid.Symbol[len(grid.Symbol)-len(quote):]
	return token == quote
}

func (grid *Grid) GetRunTime() time.Duration {
	return time.Duration(time.Now().Unix()-grid.BookTime/1000) * time.Second
}

func (grid *Grid) MarketPriceWithinRange() bool {
	marketPrice, _ := sdk.GetSessionSymbolPrice(grid.Symbol)
	lowerLimit, _ := strconv.ParseFloat(grid.GridLowerLimit, 64)
	upperLimit, _ := strconv.ParseFloat(grid.GridUpperLimit, 64)
	return marketPrice > lowerLimit && marketPrice < upperLimit
}

func (grid *Grid) String() string {
	extendedProfit := fmt.Sprintf("[%.2f%% (%s), %.2f%% (%s)]",
		grid.Highest.Roi*100,
		time.Since(grid.Highest.Time).Round(time.Minute),
		grid.Lowest.Roi*100,
		time.Since(grid.Lowest.Time).Round(time.Minute))

	outOfRange := ""
	if !grid.MarketPriceWithinRange() {
		outOfRange = "**[OOR]** "
	}
	return fmt.Sprintf("*%d*, Realized: %.2f, **%.2f%%**, Total: %.2f, %s**%.2f%%**%s",
		grid.GID,
		grid.LastRealizedPnl, grid.LastRealizedRoi*100, grid.LastPnl, outOfRange, grid.LastRoi*100, extendedProfit)
}
