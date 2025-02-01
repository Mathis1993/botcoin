package trading

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"

	"botcoin/api"
	"botcoin/config"
)

type Response struct {
	Action string  `json:"action"`
	Arg    ArgData `json:"arg"`
	Data   []Order `json:"data"`
	Ts     int64   `json:"ts"`
}

type ArgData struct {
	InstType string `json:"instType"`
	Channel  string `json:"channel"`
	InstId   string `json:"instId"`
}

type FeeDetail struct {
	FeeCoin string `json:"feeCoin"`
	Fee     string `json:"fee"`
}

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

type OrderPair struct {
	BuyOrderId  string
	SellOrderId string
	Symbol      string
	BuyPrice    float64
	Size        float64
}

type TradingPair struct {
	config       config.TradingPairConfig
	activeOrders map[string]*OrderPair // key: buyOrderId
	orderCount   int
	mu           sync.Mutex
}

type Bot struct {
	client       *api.Client
	ws           *api.WebsocketClient
	config       *config.Config
	tradingPairs map[string]*TradingPair // key: symbol
	mu           sync.Mutex
	isRunning    bool
}

func NewBot(cfg *config.Config) (*Bot, error) {
	client := api.NewClient(cfg.APIKey, cfg.SecretKey, cfg.PassPhrase, cfg.IsDemoTrading)
	ws, err := api.NewWebsocketClient(cfg.APIKey, cfg.SecretKey, cfg.PassPhrase, cfg.IsDemoTrading)
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket client: %w", err)
	}

	bot := &Bot{
		client:       client,
		ws:           ws,
		config:       cfg,
		tradingPairs: make(map[string]*TradingPair),
	}

	// Initialize trading pairs
	for _, pairConfig := range cfg.TradingPairs {
		bot.tradingPairs[pairConfig.Symbol] = &TradingPair{
			config:       pairConfig,
			activeOrders: make(map[string]*OrderPair),
		}
	}

	return bot, nil
}

func (b *Bot) Start() error {
	b.mu.Lock()
	if b.isRunning {
		b.mu.Unlock()
		return fmt.Errorf("bot is already running")
	}
	b.isRunning = true
	b.mu.Unlock()

	// Subscribe to order updates for all trading pairs
	if err := b.ws.Subscribe(b.handleOrderUpdate); err != nil {
		return fmt.Errorf("failed to request to subscribe to order updates for: %w", err)
	}
	log.Print("requested to subscribe to order updates")

	// Start trading for all pairs
	for symbol, pair := range b.tradingPairs {
		if err := b.placeBuyOrder(symbol, pair); err != nil {
			log.Printf("failed to place initial buy order for %s: %v", symbol, err)
		}
	}

	return nil
}

func (b *Bot) Stop() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isRunning {
		return nil
	}

	b.isRunning = false
	return b.ws.Close()
}

func (b *Bot) placeBuyOrder(symbol string, pair *TradingPair) error {
	pair.mu.Lock()
	defer pair.mu.Unlock()

	if pair.orderCount >= pair.config.MaxOrders {
		return fmt.Errorf("maximum number of orders reached for %s", symbol)
	}

	// Get current price
	currentPrice, err := b.client.GetCurrentPrice(symbol)
	if err != nil {
		return fmt.Errorf("failed to get current price: %w", err)
	}

	// Calculate buy price x% below current price
	buyPrice := currentPrice * (1 - pair.config.BuyPercent/100)
	size := pair.config.OrderAmount / buyPrice // Convert EUR amount to crypto amount

	// Place limit buy order
	orderId, err := b.client.PlaceLimitOrder(
		symbol,
		"buy",
		buyPrice,
		size,
	)
	if err != nil {
		return fmt.Errorf("failed to place buy order: %w", err)
	}

	// Track the order
	pair.activeOrders[orderId] = &OrderPair{
		BuyOrderId: orderId,
		Symbol:     symbol,
		BuyPrice:   buyPrice,
		Size:       size,
	}
	pair.orderCount++

	log.Printf("Placed buy order %s for %s at price %.2f", orderId, symbol, buyPrice)
	return nil
}

func (b *Bot) handleOrderUpdate(data []byte) {
	var orders []Order
	if err := json.Unmarshal(data, &orders); err != nil {
		log.Printf("Failed to parse order update: %v \n update data: %s", err, string(data))
		return
	}

	for _, order := range orders {
		log.Printf("Received order update: status %s for order with id %s", order.Status, order.OrderId)
		b.handleSingleOrderUpdate(&order)
	}
}

func (b *Bot) handleSingleOrderUpdate(order *Order) {
	log.Print("Handling order update")
	pair, exists := b.tradingPairs[order.InstId]
	if !exists {
		log.Printf("Received order update for unknown symbol: %s", order.InstId)
		return
	}
	log.Print("Found trading pair")

	pair.mu.Lock()
	defer pair.mu.Unlock()

	// Handle filled buy orders
	price, err := strconv.ParseFloat(order.Price, 64)
	if err != nil {
		log.Printf("Failed to parse order price: %v", err)
		return
	}
	log.Printf("Dealing with order with id %s, status %s and side %s", order.OrderId, order.Status, order.Side)
	for _, orderPair := range pair.activeOrders {
		log.Print("----------------- Current active order pais start -----------------")
		log.Printf("Active order pair: buy order %s, sell order %s", orderPair.BuyOrderId, orderPair.SellOrderId)
		log.Print("-----------------  Current active order pais end  -----------------")
	}
	orderPair, exists := pair.activeOrders[order.OrderId]
	if exists && order.Status == "filled" && order.Side == "buy" {
		log.Print("Attempting to place sell order")
		// Place sell order
		sellPrice := price * (1 + pair.config.SellPercent/100)
		sellOrderId, err := b.client.PlaceLimitOrder(
			order.InstId,
			"sell",
			sellPrice,
			orderPair.Size,
		)
		if err != nil {
			log.Printf("Failed to place sell order: %v", err)
			return
		}

		// Update order tracking
		orderPair.SellOrderId = sellOrderId
		log.Printf("Placed sell order %s for %s at price %.2f", sellOrderId, order.InstId, sellPrice)
	}

	if !exists && order.Side == "buy" {
		log.Printf("Order with id %s, status %s and side %s is not in actie orders", order.OrderId, order.Status, order.Side)
	}

	// Handle filled sell orders
	if order.Status == "filled" && order.Side == "sell" {
		log.Printf("Sell order %s for %s filled at price %.2f filled", order.OrderId, order.InstId, price)
		// Find and remove the completed order pair
		for buyOrderId, orderPair := range pair.activeOrders {
			if orderPair.SellOrderId == order.OrderId {
				delete(pair.activeOrders, buyOrderId)
				pair.orderCount--
				log.Printf("Completed order pair for %s: buy order %s, sell order %s", order.InstId, buyOrderId, order.OrderId)
				// Place new buy order if under limit
				if pair.orderCount < pair.config.MaxOrders {
					if err := b.placeBuyOrder(order.InstId, pair); err != nil {
						log.Printf("Failed to place new buy order: %v", err)
					}
				}
				break
			}
		}
	}
}
