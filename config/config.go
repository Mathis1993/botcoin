package config

import (
	"encoding/json"
	"os"
)

// TradingPairConfig holds configuration for a single trading pair
type TradingPairConfig struct {
	Symbol      string  `json:"symbol"`       // e.g., "BTCUSDT"
	BuyPercent  float64 `json:"buy_percent"`  // percentage below current price
	SellPercent float64 `json:"sell_percent"` // percentage above buy price
	OrderAmount float64 `json:"order_amount"` // amount in euros
	MaxOrders   int     `json:"max_orders"`   // maximum number of concurrent orders
}

type Config struct {
	APIKey        string              `json:"api_key"`
	SecretKey     string              `json:"secret_key"`
	PassPhrase    string              `json:"passphrase"`
	IsDemoTrading bool                `json:"is_demo_trading"` // use demo trading
	TradingPairs  []TradingPairConfig `json:"trading_pairs"`   // multiple trading pairs
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
