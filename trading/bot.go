package trading

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"botcoin/api"
	"botcoin/config"
)

type TradingProcess struct {
	alreadyInitialized bool
	mu                 sync.Mutex
	Symbol             string
	SellTargetPercent  float64
	BuyOrders          []BuyOrder
	SellOrder          *SellOrder
}

func (tp *TradingProcess) OrderWithIdExists(orderId string) bool {
	for _, buyOrder := range tp.BuyOrders {
		if buyOrder.OrderId == orderId {
			return true
		}
		if tp.SellOrder != nil && tp.SellOrder.OrderId == orderId {
			return true
		}
	}
	return false
}

type BuyOrder struct {
	OrderId     string
	CoinPrice   float64
	OrderAmount float64
}

type SellOrder struct {
	OrderId     string
	CoinPrice   float64
	OrderAmount float64
}

type Bot struct {
	client           *api.Client
	ws               *api.WebsocketClient
	config           *config.Config
	tradingProcesses map[string]*TradingProcess
	mu               sync.Mutex
	isRunning        bool
}

func NewBot(cfg *config.Config) (*Bot, error) {
	client := api.NewClient(cfg.APIKey, cfg.SecretKey, cfg.PassPhrase, cfg.IsDemoTrading)
	ws, err := api.NewWebsocketClient(cfg.APIKey, cfg.SecretKey, cfg.PassPhrase, cfg.IsDemoTrading)
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket client: %w", err)
	}

	bot := &Bot{
		client:           client,
		ws:               ws,
		config:           cfg,
		tradingProcesses: make(map[string]*TradingProcess),
	}

	// Initialize trading processes
	// ToDo(ME-07.02.25): Sync bot state with bitget state
	// for each trading process
	// create trading process object
	// get all orders for symbol
	// append all buy orders
	// check for position and sell order -> potentially create new sell order
	// if any order appended, set alreadyInitialized to true

	// for each trading process
	// tradingProcess := b.fetchCurrentTradingProcess(symbol)
	// if tradingProcess.alreadyInitialized
	// b.tradingProcesses[symbol] = tradingProcess
	// continue
	// regular initialization

	// https://www.bitget.com/api-doc/contract/trade/Get-Orders-Pending
	for _, tradingProcessConfig := range cfg.TradingProcesses {
		tradingProcess := &TradingProcess{
			Symbol:            tradingProcessConfig.Symbol,
			SellTargetPercent: tradingProcessConfig.SellTargetPercent,
		}
		for _, buyOrderConfig := range tradingProcessConfig.BuyOrders {
			coinPrice := buyOrderConfig.CoinPrice
			if buyOrderConfig.CoinPriceBelowPercent > 0 {
				// Get current price for symbol
				currentPrice, err := client.GetCurrentPrice(tradingProcess.Symbol)
				if err != nil {
					return nil, fmt.Errorf("failed to get current price for %s: %w", tradingProcess.Symbol, err)
				}
				coinPrice = currentPrice * (1 - buyOrderConfig.CoinPriceBelowPercent/100)
			}
			tradingProcess.BuyOrders = append(tradingProcess.BuyOrders, BuyOrder{
				CoinPrice:   coinPrice,
				OrderAmount: buyOrderConfig.OrderAmount,
			})
		}
		bot.tradingProcesses[tradingProcess.Symbol] = tradingProcess
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

	// Register handler dealing with order updates for all trading pairs
	b.ws.RegisterHandler(b.handleOrderUpdate)
	log.Println("Registered order update handler")

	// Start trading for all pairs
	for symbol, process := range b.tradingProcesses {
		if process.alreadyInitialized {
			log.Printf("Trading process for %s already initialized", symbol)
			continue
		}
		if err := b.placeBuyOrders(symbol, process); err != nil {
			log.Printf("failed to place initial buy order for %s: %v", symbol, err)
		}
	}

	return nil
}

func (b *Bot) GetPosition() {
	position, err := b.client.GetPosition("SBTCSUSDT")
	if err != nil || position == nil {
		log.Printf("Failed to get position: %v", err)
		return
	}
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

func (b *Bot) placeBuyOrders(symbol string, process *TradingProcess) error {
	process.mu.Lock()
	defer process.mu.Unlock()

	for i, buyOrder := range process.BuyOrders {
		if buyOrder.OrderId != "" {
			log.Printf("Buy order for %s already placed with id %s", symbol, buyOrder.OrderId)
		}
		price := buyOrder.CoinPrice
		size := buyOrder.OrderAmount / price // Convert EUR amount to crypto amount
		orderId, err := b.client.PlaceLimitOrder(
			symbol,
			"buy",
			price,
			size,
		)
		if err != nil {
			return fmt.Errorf("failed to place buy order: %w", err)
		}
		process.BuyOrders[i].OrderId = orderId
		log.Printf("Placed buy order %s for %s at price %.2f", orderId, symbol, price)
	}

	return nil
}

func (b *Bot) handleOrderUpdate(data []byte) {
	var orders []api.Order
	if err := json.Unmarshal(data, &orders); err != nil {
		log.Printf("Failed to parse order update: %v \n update data: %s", err, string(data))
		return
	}

	for _, order := range orders {
		log.Printf("Received order update: status %s for order with id %s", order.Status, order.OrderId)
		b.handleSingleOrderUpdate(&order)
	}
}

func (b *Bot) handleSingleOrderUpdate(order *api.Order) {
	// ToDo(ME-07.02.25): Handle cancellation
	log.Print("Handling order update")
	process, exists := b.tradingProcesses[order.InstId]
	if !exists {
		log.Printf("Received order update for unknown symbol: %s", order.InstId)
		return
	}

	process.mu.Lock()
	defer process.mu.Unlock()

	// Handle filled buy orders
	price, err := strconv.ParseFloat(order.Price, 64)
	if err != nil {
		log.Printf("Failed to parse order price: %v", err)
		return
	}
	log.Printf("Dealing with order with id %s, status %s and side %s", order.OrderId, order.Status, order.Side)
	if !process.OrderWithIdExists(order.OrderId) {
		log.Printf("Order with id %s is not in configured orders for trading process with symbol %s", order.OrderId, order.InstId)
		return
	}
	if order.Status == "filled" && order.Side == "buy" {
		log.Print("Buy order filled, waiting shortly to ensure the position is updated...")
		time.Sleep(3 * time.Second) // Wait for position to be updated
		position, err := b.client.GetPosition(order.InstId)
		if err != nil || position == nil {
			log.Printf("Failed to get position: %v", err)
			return
		}
		if previousSellOrder := process.SellOrder; previousSellOrder != nil {
			log.Printf("Attempting to cancel existing sell order %s for %s (price: %2.f)", previousSellOrder.OrderId, order.InstId, previousSellOrder.CoinPrice)
			err := b.client.CancelOrder(order.InstId, previousSellOrder.OrderId)
			if err != nil {
				log.Printf("Failed to cancel previous sell order: %v", err)
				return
			}
			log.Print("Successfully cancelled previous sell order")
		}
		avgPrice, err := strconv.ParseFloat(position.OpenPriceAvg, 64)
		if err != nil {
			log.Printf("Failed to parse average price: %v", err)
			return
		}
		size, err := strconv.ParseFloat(position.Total, 64)
		if err != nil {
			log.Printf("Failed to parse position size: %v", err)
			return
		}
		log.Printf("Current position for %s: average price %.2f", order.InstId, avgPrice)
		sellPrice := avgPrice * (1 + process.SellTargetPercent/100)
		log.Printf("Attempting to place sell order for %s at price %.2f", order.InstId, sellPrice)
		// Place sell order
		sellOrderId, err := b.client.PlaceLimitOrder(
			order.InstId,
			"sell",
			sellPrice,
			size,
		)
		if err != nil {
			log.Printf("Failed to place sell order: %v", err)
			return
		}

		log.Printf("Placed sell order %s for %s at price %.2f", sellOrderId, order.InstId, sellPrice)
		// Update order tracking
		sellOrder := &SellOrder{
			OrderId:     sellOrderId,
			CoinPrice:   sellPrice,
			OrderAmount: size,
		}
		process.SellOrder = sellOrder
		log.Printf("Updated sell order in order process")
	}

	// Handle filled sell orders
	if order.Status == "filled" && order.Side == "sell" {
		log.Printf("Sell order %s for %s filled at price %.2f filled", order.OrderId, order.InstId, price)
		// Remove the completed order process
		delete(b.tradingProcesses, order.InstId)
		log.Printf("Trading process for %s completed!", order.InstId)
	}
}
