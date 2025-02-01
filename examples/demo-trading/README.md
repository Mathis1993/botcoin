# Demo Trading Example

This example demonstrates how to use the bot with Bitget's demo trading environment. Demo trading allows you to test strategies without using real funds.

## Setup

1. Create a Bitget account if you don't have one:
   - Go to [Bitget](https://www.bitget.com/)
   - Sign up for an account
   - Complete email verification

2. Get API credentials for demo trading:
   - Log in to your Bitget account
   - Go to API Management
   - Create a new API key
   - Save the API key, secret key, and passphrase

3. Update `config.json` with your credentials:
   - Replace `your-passphrase` with your API passphrase
   - The example includes sample API and secret keys that should work with demo trading

## Configuration Explanation

```json
{
    "is_demo_trading": true,  // Enables demo trading mode
    "trading_pairs": [
        {
            "symbol": "SBTCSUSDT",     // Bitcoin USDT-M futures demo symbol
            "buy_percent": 0.5,        // Buy 0.5% below market price
            "sell_percent": 0.8,       // Sell 0.8% above buy price
            "order_amount": 50.0,      // Trade size in USDT
            "max_orders": 2            // Maximum concurrent orders
        },
        {
            "symbol": "SETHSUSDT",     // Ethereum USDT-M futures demo symbol
            "buy_percent": 0.6,        // Buy 0.6% below market price
            "sell_percent": 1.0,       // Sell 1.0% above buy price
            "order_amount": 30.0,      // Trade size in USDT
            "max_orders": 2            // Maximum concurrent orders
        }
    ]
}
```

## Technical Details

The bot automatically handles the differences between demo and live trading:
- Uses demo trading endpoints when `is_demo_trading` is true
- Sets `productType` to "sdmcbl" for USDT-M Futures Demo trading
- Uses demo trading symbols (prefixed with 'S')

## Running the Example

1. From the project root directory:
```bash
go run main.go -config examples/demo-trading/config.json
```

2. The bot will:
   - Connect to Bitget's demo trading API
   - Monitor BTC and ETH futures prices
   - Place limit orders according to the configuration
   - Log all actions and order status changes

3. Expected output:
```
2025/01/27 22:18:23 Connected to Bitget WebSocket
2025/01/27 22:18:23 Subscribed to SBTCSUSDT orders
2025/01/27 22:18:23 Subscribed to SETHSUSDT orders
2025/01/27 22:18:24 Current SBTCSUSDT price: 40000.00
2025/01/27 22:18:24 Placed buy order 12345 for SBTCSUSDT at price 39800.00
...
```

## Testing Different Scenarios

1. Price Movement Testing:
   - Watch how the bot places buy orders below market price
   - Observe order execution when price drops to the buy level
   - See automatic sell order placement after buys are filled

2. Multiple Order Testing:
   - The bot maintains up to 2 concurrent orders per pair
   - When a sell order completes, a new buy order is placed
   - Test how the bot handles multiple orders simultaneously

3. Error Handling:
   - Try disconnecting your internet to test reconnection
   - Check how the bot handles API errors
   - Observe WebSocket reconnection behavior

## Monitoring Orders

You can monitor your demo trading orders:
1. Log in to your Bitget account
2. Go to Futures Trading
3. Switch to Demo Trading mode
4. Check the Open Orders and Order History tabs

## Available Demo Trading Pairs

Demo trading pairs are prefixed with 'S' and end with 'SUSDT'. Common pairs include:
- SBTCSUSDT (Bitcoin)
- SETHSUSDT (Ethereum)
- SXRPSUSDT (Ripple)
- SADASUSDT (Cardano)
- SDOGESUSDT (Dogecoin)

## Next Steps

After testing with demo trading, you can:
1. Adjust the trading parameters
2. Try different trading pairs
3. Test various market conditions
4. Monitor the bot's performance

When you're satisfied with the results, you can switch to live trading by:
1. Setting `is_demo_trading` to `false`
2. Using real API credentials
3. Updating symbols to remove the 'S' prefix (e.g., "BTCUSDT" instead of "SBTCSUSDT")
