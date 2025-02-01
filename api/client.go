package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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

const (
	baseURL = "https://api.bitget.com"
	apiPath = "/api/v2/mix"
	//wsEndpoint             = "wss://ws.bitget.com/mix/v1/stream"
	wsEndpoint             = "wss://ws.bitget.com/v2/ws/private"
	productTypeDemoFutures = "susdt-futures"
	productTypeLiveFutures = "usdt-futures"
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

func (c *Client) PlaceLimitOrder(symbol string, side string, price float64, size float64) (string, error) {
	if err := c.validateSymbol(symbol); err != nil {
		return "", err
	}

	productType := c.getProductType()
	marginCoin := "USDT"
	if c.isDemoTrading {
		productType = "susdt-futures"
		marginCoin = "SUSDT"
	}

	orderReq := OrderRequest{
		Symbol:      symbol,
		ProductType: productType,
		MarginMode:  "isolated",
		MarginCoin:  marginCoin,
		Size:        strconv.FormatFloat(size, 'f', 8, 64),
		Price:       strconv.FormatFloat(price, 'f', 1, 64),
		Side:        side,
		TradeSide:   "open",
		OrderType:   "limit",
		Force:       "gtc",
		ReduceOnly:  "NO",
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

	path := fmt.Sprintf("/order/cancel-order?symbol=%s&orderId=%s", symbol, orderId)
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return err
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return err
	}

	if resp.Code != "00000" {
		return fmt.Errorf("order cancellation failed: %s", resp.Msg)
	}

	return nil
}
