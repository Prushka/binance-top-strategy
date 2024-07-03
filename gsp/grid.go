package gsp

import (
	"BinanceTopStrategies/discord"
	"BinanceTopStrategies/sdk"
	"BinanceTopStrategies/sql"
	"context"
	"fmt"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/jackc/pgx/v5"
	"strconv"
	"time"
)

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

type TrackedGrids struct {
	TotalGridInitial map[string]float64
	TotalGridPnl     map[string]float64
	Shorts           mapset.Set[int]
	Longs            mapset.Set[int]
	Neutrals         mapset.Set[int]
	ExistingSIDs     mapset.Set[int]
	ExistingSymbols  mapset.Set[string]
	GridsByGid       GridsMap
}

type GridsMap map[int]*Grid

func (grid *Grid) IsQuote(quote string) bool {
	token := grid.Symbol[len(grid.Symbol)-len(quote):]
	return token == quote
}

func (gm GridsMap) GetChunks(quote string) int {
	chunks := 0
	for _, g := range gm {
		if g.IsQuote(quote) {
			chunks++
		}
	}
	return chunks
}

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

func (tracked *TrackedGrids) GetGridBySID(sid int) *Grid {
	for _, g := range tracked.GridsByGid {
		if g.SID == sid {
			return g
		}
	}
	return nil
}

func (tracked *TrackedGrids) calcPnl(g *Grid, multiplier float64) {
	if g.IsQuote("USDT") {
		tracked.TotalGridInitial["USDT"] += multiplier * g.InitialValue
		tracked.TotalGridPnl["USDT"] += multiplier * g.LastPnl
	} else if g.IsQuote("USDC") {
		tracked.TotalGridInitial["USDC"] += multiplier * g.InitialValue
		tracked.TotalGridPnl["USDC"] += multiplier * g.LastPnl
	}
}

func (tracked *TrackedGrids) remove(id int) {
	g, ok := tracked.GridsByGid[id]
	if !ok {
		return
	}
	tracked.ExistingSymbols.Remove(g.Symbol)
	tracked.ExistingSIDs.Remove(g.SID)
	if g.Direction == DirectionMap[LONG] {
		tracked.Longs.Remove(g.GID)
	} else if g.Direction == DirectionMap[SHORT] {
		tracked.Shorts.Remove(g.GID)
	} else {
		tracked.Neutrals.Remove(g.GID)
	}
	tracked.calcPnl(g, -1)
	delete(tracked.GridsByGid, g.GID)
}

func (tracked *TrackedGrids) add(g *Grid) {
	tracked.ExistingSymbols.Add(g.Symbol)
	tracked.ExistingSIDs.Add(g.SID)

	if g.Direction == DirectionMap[LONG] {
		tracked.Longs.Add(g.GID)
	} else if g.Direction == DirectionMap[SHORT] {
		tracked.Shorts.Add(g.GID)
	} else {
		tracked.Neutrals.Add(g.GID)
	}
	initial, _ := strconv.ParseFloat(g.GridInitialValue, 64)
	g.LastRealizedPnl, _ = strconv.ParseFloat(g.GridProfit, 64)
	fundingFee, _ := strconv.ParseFloat(g.FundingFee, 64)
	position, _ := strconv.ParseFloat(g.GridPosition, 64)
	entryPrice, _ := strconv.ParseFloat(g.GridEntryPrice, 64)
	marketPrice, _ := sdk.GetSessionSymbolPrice(g.Symbol)
	lowerLimit, _ := strconv.ParseFloat(g.GridLowerLimit, 64)
	upperLimit, _ := strconv.ParseFloat(g.GridUpperLimit, 64)
	g.BelowLowerLimit = marketPrice < lowerLimit
	g.AboveUpperLimit = marketPrice > upperLimit
	g.InitialValue = initial / float64(g.InitialLeverage)
	g.LastPnl = g.LastRealizedPnl + fundingFee + position*(marketPrice-entryPrice) // position is negative for short
	g.LastRoi = g.LastPnl / g.InitialValue
	g.LastRealizedRoi = g.LastRealizedPnl / g.InitialValue
	prevG, ok := tracked.GridsByGid[g.GID]
	tracked.calcPnl(g, 1)
	if ok {
		tracked.calcPnl(prevG, -1)
	}
	tracked.GridsByGid[g.GID] = g
	err := sql.SimpleTransaction(func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(),
			`INSERT INTO bts.grid (gid, roi, realized_roi, time) VALUES ($1, $2, $3, $4)`,
			g.GID, g.LastRoi, g.LastRealizedRoi, time.Now())
		return err
	})
	if err != nil {
		discord.Errorf("Error inserting grid: %v", err)
	}
	g.Lowest, g.Highest = g.GetLH()
}

type GridDB struct {
	GID         int       `db:"gid"`
	Roi         float64   `db:"roi"`
	RealizedRoi float64   `db:"realized_roi"`
	Time        time.Time `db:"time"`
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
