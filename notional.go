package main

type NotionalBracket struct {
	BracketSeq                   int     `json:"bracketSeq"`
	BracketNotionalFloor         int     `json:"bracketNotionalFloor"`
	BracketNotionalCap           int     `json:"bracketNotionalCap"`
	BracketMaintenanceMarginRate float64 `json:"bracketMaintenanceMarginRate"`
	CumFastMaintenanceAmount     int     `json:"cumFastMaintenanceAmount"`
	MinOpenPosLeverage           int     `json:"minOpenPosLeverage"`
	MaxOpenPosLeverage           int     `json:"maxOpenPosLeverage"`
}

type NotionalSymbol struct {
	Symbol        string             `json:"symbol"`
	UpdateTime    int64              `json:"updateTime"`
	NotionalLimit int                `json:"notionalLimit"`
	RiskBrackets  []*NotionalBracket `json:"riskBrackets"`
}

type NotionalResponse struct {
	Symbols struct {
		Brackets []*NotionalSymbol `json:"brackets"`
	} `json:"data"`
	BinanceBaseResponse
}

func getBrackets() (*NotionalResponse, error) {
	resp, _, err := request("https://www.binance.com/bapi/futures/v1/friendly/future/common/brackets",
		"{}", &NotionalResponse{})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
