# Botcoin

An automated trading bot for cryptocurrency futures using the Bitget API. The bot implements a parallel trading strategy:
1. Monitors multiple cryptocurrency pairs simultaneously
2. For each pair, maintains multiple concurrent orders (configurable)
3. Places limit buy orders at X% below current price
4. When a buy order is filled, automatically places a sell order at Y% above the buy price
5. After a sell order completes, places a new buy order if below the maximum order limit
6. Tracks all orders using order IDs to maintain proper buy/sell relationships

## Features

- Real-time price monitoring
- Automated order placement
- Configurable buy/sell percentages
- Configurable order amounts
- Support for demo trading
- WebSocket integration for instant order notifications
- Graceful shutdown handling

## Prerequisites

- Go 1.23.4 or higher
- Bitget API credentials (API key, secret key, and passphrase)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/botcoin.git
cd botcoin
```

2. Install dependencies:
```bash
go mod download
```

3. Copy the example configuration file and fill in your settings:
```bash
cp sample-config.json config.json
```

## Configuration

The bot can operate in both demo and live trading modes, controlled by the `is_demo_trading` flag in the configuration. When enabled, the bot automatically uses demo trading endpoints and sets the appropriate productType for USDT-M Futures trading.

### API Configuration

The bot automatically configures the appropriate parameters based on the `is_demo_trading` setting:

Demo Trading:
- Symbol prefix: 'S' (e.g., "SBTCSUSDT")
- Product Type: "susdt-futures"
- Margin Coin: "SUSDT"
- WebSocket InstType: "SUSDT-FUTURES"

Live Trading:
- Standard symbols (e.g., "BTCUSDT")
- Product Type: "usdt-futures"
- Margin Coin: "USDT"
- WebSocket InstType: "USDT-FUTURES"

All requests are made to:
- REST API: https://api.bitget.com
- WebSocket: wss://ws.bitget.com/mix/v1/stream

### Configuration File

Create a `config.json` file with your settings:

```json
{
    "api_key": "your-api-key",
    "secret_key": "your-secret-key",
    "passphrase": "your-passphrase",
    "is_demo_trading": true,
    "trading_pairs": [
        {
            "symbol": "SBTCSUSDT",
            "buy_percent": 0.5,
            "sell_percent": 0.8,
            "order_amount": 50.0,
            "max_orders": 2
        },
        {
            "symbol": "SETHSUSDT",
            "buy_percent": 0.6,
            "sell_percent": 1.0,
            "order_amount": 30.0,
            "max_orders": 2
        }
    ]
}
```

Configuration parameters:
- `api_key`: Your Bitget API key
- `secret_key`: Your Bitget API secret key
- `passphrase`: Your Bitget API passphrase
- `is_demo_trading`: Set to true for demo trading, false for real trading
- `trading_pairs`: Array of trading pair configurations:
  - `symbol`: Trading pair symbol (e.g., "SBTCSUSDT" for demo, "BTCUSDT" for live)
  - `buy_percent`: Percentage below current price to place buy orders
  - `sell_percent`: Percentage above buy price to place sell orders
  - `order_amount`: Amount in USDT for each order
  - `max_orders`: Maximum number of concurrent orders for this pair

## Usage

For a complete demo trading example with step-by-step instructions, see [examples/demo-trading](examples/demo-trading).

### Trading Modes

#### Demo Trading
When `is_demo_trading` is set to `true`:
- Uses demo trading parameters (susdt-futures)
- Uses demo symbols (prefixed with 'S', e.g., "SBTCSUSDT")
- Places orders without using real funds
- Perfect for testing strategies

#### Live Trading
When `is_demo_trading` is set to `false`:
- Uses live trading parameters (usdt-futures)
- Uses standard symbols (e.g., "BTCUSDT")
- Places real orders using actual funds
- Use with caution and proper risk management

### Available Trading Pairs

Demo Trading Pairs (prefixed with 'S'):
- SBTCSUSDT (Bitcoin)
- SETHSUSDT (Ethereum)
- SXRPSUSDT (Ripple)
- SADASUSDT (Cardano)
- SDOGESUSDT (Dogecoin)

Live Trading Pairs:
- BTCUSDT
- ETHUSDT
- XRPUSDT
- ADAUSDT
- DOGEUSDT

## Safety Features

- Demo trading support with dedicated test environment
- Configurable parameters per trading pair
- Maximum order limits per trading pair
- Automatic reconnection for WebSocket
- Proper error handling and rate limiting
- Order tracking and state management
- Thread-safe operations with mutex locks
