package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	APIKey     string `json:"api_key"`
	SecretKey  string `json:"secret_key"`
	PassPhrase string `json:"passphrase"`
	// ToDo(ME-01.02.25): We only want to support one way mode
	HedgeMode        bool                   `json:"hedge_mode"`        // is the account using hedge mode or one way mode
	IsDemoTrading    bool                   `json:"is_demo_trading"`   // use demo trading
	TradingProcesses []TradingProcessConfig `json:"trading_processes"` // multiple trading processes
}

type TradingProcessConfig struct {
	Symbol            string           `json:"symbol"`
	SellTargetPercent float64          `json:"sell_target_percent"`
	BuyOrders         []BuyOrderConfig `json:"buy_orders"`
}

type BuyOrderConfig struct {
	CoinPrice             float64 `json:"coin_price"`
	CoinPriceBelowPercent float64 `json:"coin_price_below_percent"`
	OrderAmount           float64 `json:"order_amount"`
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
