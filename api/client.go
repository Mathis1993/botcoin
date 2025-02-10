package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL = "https://api.bitget.com"
	apiPath = "/api/v2/mix"
	//wsEndpoint             = "wss://ws.bitget.com/mix/v1/stream"
	wsEndpoint             = "wss://ws.bitget.com/v2/ws/private"
	productTypeDemoFutures = "susdt-futures"
	productTypeLiveFutures = "usdt-futures"
	marginCoinDemo         = "SUSDT"
	marginCoinLive         = "USDT"
)

// validateSymbol checks if the symbol format matches the trading mode
func (c *Client) validateSymbol(symbol string) error {
	if c.isDemoTrading {
		if !strings.HasPrefix(symbol, "S") {
			return fmt.Errorf("demo trading requires symbols with 'S' prefix, got: %s", symbol)
		}
	} else {
		if strings.HasPrefix(symbol, "S") {
			return fmt.Errorf("live trading requires symbols without 'S' prefix, got: %s", symbol)
		}
	}
	return nil
}

// getProductType returns the product type based on the trading mode
func (c *Client) getProductType() string {
	if c.isDemoTrading {
		return productTypeDemoFutures
	}
	return productTypeLiveFutures
}

func (c *Client) getMarginCoin() string {
	if c.isDemoTrading {
		return marginCoinDemo
	}
	return marginCoinLive
}

type Client struct {
	apiKey        string
	secretKey     string
	passphrase    string
	isDemoTrading bool
	httpClient    *http.Client
}

func NewClient(apiKey, secretKey, passphrase string, isDemoTrading bool) *Client {
	return &Client{
		apiKey:        apiKey,
		secretKey:     secretKey,
		passphrase:    passphrase,
		isDemoTrading: isDemoTrading,
		httpClient: &http.Client{
			Timeout: time.Second * 10,
		},
	}
}

func (c *Client) sign(timestamp, method, requestPath, body string) string {
	message := timestamp + method + requestPath + body
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *Client) getHeaders(method, requestPath, body string) map[string]string {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sign := c.sign(timestamp, method, requestPath, body)

	headers := map[string]string{
		"ACCESS-KEY":        c.apiKey,
		"ACCESS-SIGN":       sign,
		"ACCESS-TIMESTAMP":  timestamp,
		"ACCESS-PASSPHRASE": c.passphrase,
		"Content-Type":      "application/json",
		"locale":            "en-US",
	}

	return headers
}

func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyStr string
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyStr = string(bodyBytes)
	}

	fullURL := baseURL + apiPath + path
	req, err := http.NewRequest(method, fullURL, bytes.NewBuffer([]byte(bodyStr)))
	if err != nil {
		return nil, err
	}

	headers := c.getHeaders(method, apiPath+path, bodyStr)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *Client) GetCurrentPrice(symbol string) (float64, error) {
	if err := c.validateSymbol(symbol); err != nil {
		return 0, err
	}

	path := fmt.Sprintf("/market/ticker?productType=%s&symbol=%s", c.getProductType(), symbol)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return 0, err
	}

	var tickerResp TickerResponse
	if err := json.Unmarshal(respBody, &tickerResp); err != nil {
		return 0, err
	}

	if len(tickerResp.Data) == 0 {
		return 0, fmt.Errorf("no price data available for %s", symbol)
	}

	price, err := strconv.ParseFloat(tickerResp.Data[0].LastPrice, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price: %w", err)
	}

	return price, nil
}

func (c *Client) GetPosition(symbol string) (*Position, error) {
	if err := c.validateSymbol(symbol); err != nil {
		return nil, err
	}

	productType := c.getProductType()
	marginCoin := c.getMarginCoin()

	path := fmt.Sprintf("/position/single-position?symbol=%s&productType=%s&marginCoin=%s", symbol, productType, marginCoin)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var positionResp PositionResponse
	if err := json.Unmarshal(respBody, &positionResp); err != nil {
		return nil, err
	}

	if positionResp.Code != "00000" {
		return nil, fmt.Errorf("order placement failed: %s", positionResp.Msg)
	}

	if len(positionResp.Data) > 0 {
		return &positionResp.Data[0], nil
	}

	return nil, errors.New("no position data available")
}

func (c *Client) GetAllPositions() ([]Position, error) {
	productType := c.getProductType()
	marginCoin := c.getMarginCoin()

	path := fmt.Sprintf("/position/all-position?productType=%s&marginCoin=%s", productType, marginCoin)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var positionResp PositionResponse
	if err := json.Unmarshal(respBody, &positionResp); err != nil {
		return nil, err
	}

	if positionResp.Code != "00000" {
		return nil, fmt.Errorf("order placement failed: %s", positionResp.Msg)
	}

	return positionResp.Data, nil
}

func (c *Client) GetPendingOrders(symbol string) ([]Order, error) {
	if err := c.validateSymbol(symbol); err != nil {
		return nil, err
	}

	productType := c.getProductType()

	path := fmt.Sprintf("/order/orders-pending?symbol=%s&productType=%s", symbol, productType)
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var ordersListResponse OrderListResponse
	if err := json.Unmarshal(respBody, &ordersListResponse); err != nil {
		return nil, err
	}

	return ordersListResponse.Data.EntrustedList, nil
}

func (c *Client) PlaceLimitOrder(symbol string, side string, price float64, size float64) (string, error) {
	if err := c.validateSymbol(symbol); err != nil {
		return "", err
	}

	productType := c.getProductType()
	marginCoin := c.getMarginCoin()

	orderReq := OrderRequest{
		Symbol:      symbol,
		ProductType: productType,
		MarginMode:  "isolated",
		MarginCoin:  marginCoin,
		Size:        strconv.FormatFloat(size, 'f', 8, 64),
		Price:       strconv.FormatFloat(price, 'f', 1, 64),
		Side:        side,
		// ToDo(ME-01.02.25): Use in hedge mode
		//TradeSide:   "open",
		OrderType:  "limit",
		Force:      "gtc",
		ReduceOnly: "NO",
	}

	respBody, err := c.doRequest("POST", "/order/place-order", orderReq)
	if err != nil {
		return "", err
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(respBody, &orderResp); err != nil {
		return "", err
	}

	if orderResp.Code != "00000" {
		return "", fmt.Errorf("order placement failed: %s", orderResp.Msg)
	}

	return orderResp.Data.OrderId, nil
}

func (c *Client) CancelOrder(symbol string, orderId string) error {
	if err := c.validateSymbol(symbol); err != nil {
		return err
	}

	cancelOrderReq := CancelOrderRequest{
		Symbol:      symbol,
		ProductType: c.getProductType(),
		MarginCoin:  c.getMarginCoin(),
		OrderID:     orderId,
	}
	path := fmt.Sprintf("/order/cancel-order")
	respBody, err := c.doRequest("POST", path, cancelOrderReq)
	if err != nil {
		return err
	}

	cancelOrderResp := CancelOrderResponse{}
	if err := json.Unmarshal(respBody, &cancelOrderResp); err != nil {
		return err
	}

	if cancelOrderResp.Code != "00000" {
		return fmt.Errorf("order cancellation failed: %s", cancelOrderResp.Msg)
	}

	if cancelOrderResp.Data.OrderID != orderId {
		return fmt.Errorf("order cancellation failed: order ID mismatch, expected %s, got %s", orderId, cancelOrderResp.Data.OrderID)
	}

	return nil
}
