package api

type Order struct {
	AccBaseVolume          string      `json:"accBaseVolume"`
	CTime                  string      `json:"cTime"`
	ClientOId              string      `json:"clientOId"`
	FeeDetail              []FeeDetail `json:"feeDetail"`
	FillFee                string      `json:"fillFee"`
	FillFeeCoin            string      `json:"fillFeeCoin"`
	FillNotionalUsd        string      `json:"fillNotionalUsd"`
	FillPrice              string      `json:"fillPrice"`
	BaseVolume             string      `json:"baseVolume"`
	FillTime               string      `json:"fillTime"`
	Force                  string      `json:"force"`
	InstId                 string      `json:"instId"`
	Leverage               string      `json:"leverage"`
	MarginCoin             string      `json:"marginCoin"`
	MarginMode             string      `json:"marginMode"`
	NotionalUsd            string      `json:"notionalUsd"`
	OrderId                string      `json:"orderId"`
	OrderType              string      `json:"orderType"`
	Pnl                    string      `json:"pnl"`
	PosMode                string      `json:"posMode"`
	PosSide                string      `json:"posSide"`
	Price                  string      `json:"price"`
	PriceAvg               string      `json:"priceAvg"`
	ReduceOnly             string      `json:"reduceOnly"`
	StpMode                string      `json:"stpMode"`
	Side                   string      `json:"side"`
	Size                   string      `json:"size"`
	EnterPointSource       string      `json:"enterPointSource"`
	Status                 string      `json:"status"`
	TradeScope             string      `json:"tradeScope"`
	TradeId                string      `json:"tradeId"`
	TradeSide              string      `json:"tradeSide"`
	PresetStopSurplusPrice string      `json:"presetStopSurplusPrice"`
	TotalProfits           string      `json:"totalProfits"`
	PresetStopLossPrice    string      `json:"presetStopLossPrice"`
	UTime                  string      `json:"uTime"`
}

type Position struct {
	Symbol           string `json:"symbol"`           // Trading pair name
	MarginCoin       string `json:"marginCoin"`       // Margin coin
	HoldSide         string `json:"holdSide"`         // Position direction (long/short)
	OpenDelegateSize string `json:"openDelegateSize"` // Amount to be filled of the current order
	MarginSize       string `json:"marginSize"`       // Margin amount
	Available        string `json:"available"`        // Available amount for positions
	Locked           string `json:"locked"`           // Frozen amount in the position
	Total            string `json:"total"`            // Total amount of all positions
	Leverage         string `json:"leverage"`         // Leverage
	AchievedProfits  string `json:"achievedProfits"`  // Realized PnL
	OpenPriceAvg     string `json:"openPriceAvg"`     // Average entry price
	MarginMode       string `json:"marginMode"`       // Margin mode (isolated/crossed)
	PosMode          string `json:"posMode"`          // Position mode (one_way_mode/hedge_mode)
	UnrealizedPL     string `json:"unrealizedPL"`     // Unrealized PnL
	LiquidationPrice string `json:"liquidationPrice"` // Estimated liquidation price
	KeepMarginRate   string `json:"keepMarginRate"`   // Tiered maintenance margin rate
	MarkPrice        string `json:"markPrice"`        // Mark price
	MarginRatio      string `json:"marginRatio"`      // Maintenance margin rate
	BreakEvenPrice   string `json:"breakEvenPrice"`   // Position breakeven price
	TotalFee         string `json:"totalFee"`         // Funding fee
	DeductedFee      string `json:"deductedFee"`      // Deducted transaction fees
	CTime            string `json:"cTime"`            // Creation time
	AssetMode        string `json:"assetMode"`        // Asset mode (single/union)
	UTime            string `json:"uTime"`            // Last updated time
	AutoMargin       string `json:"autoMargin"`       // Auto Margin (on/off)
}

type TickerResponse struct {
	Code string `json:"code"`
	Data []struct {
		Symbol     string `json:"symbol"`
		LastPrice  string `json:"lastPr"`
		AskPrice   string `json:"askPr"`
		BidPrice   string `json:"bidPr"`
		IndexPrice string `json:"indexPrice"`
		Open24h    string `json:"open24h"`
		MarkPrice  string `json:"markPrice"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type FeeDetail struct {
	FeeCoin string `json:"feeCoin"`
	Fee     string `json:"fee"`
}

type OrderRequest struct {
	Symbol      string `json:"symbol"`
	ProductType string `json:"productType"`
	MarginMode  string `json:"marginMode"`
	MarginCoin  string `json:"marginCoin"`
	Size        string `json:"size"`
	Price       string `json:"price"`
	Side        string `json:"side"`
	TradeSide   string `json:"tradeSide"`
	OrderType   string `json:"orderType"`
	Force       string `json:"force"`
	ReduceOnly  string `json:"reduceOnly"`
}

type OrderResponse struct {
	Code string `json:"code"`
	Data struct {
		OrderId string `json:"orderId"`
	} `json:"data"`
	Msg string `json:"msg"`
}

type OrderListResponse struct {
	Code string `json:"code"`
	Data struct {
		EntrustedList []Order `json:"entrustedList"`
		EndId         string  `json:"endId"`
	}
	Msg string `json:"msg"`
}

type PositionResponse struct {
	Code string     `json:"code"`
	Data []Position `json:"data"`
	Msg  string     `json:"msg"`
}

type CancelOrderRequest struct {
	Symbol      string `json:"symbol"`      // Trading pair
	ProductType string `json:"productType"` // Product type (USDT-FUTURES, COIN-FUTURES, etc.)
	MarginCoin  string `json:"marginCoin"`  // Optional: Margin coin in capital letters
	OrderID     string `json:"orderId"`     // Optional: Order ID
}

type CancelOrderResponse struct {
	Code string `json:"code"`
	Data struct {
		OrderID   string `json:"orderId"`
		ClientOID string `json:"clientOid"`
	} `json:"data"`
	Msg         string `json:"msg"`
	RequestTime int64  `json:"requestTime"`
}
