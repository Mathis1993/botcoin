package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type SubscriptionHandler func([]byte)

type WebsocketClient struct {
	conn                *websocket.Conn
	mu                  sync.Mutex
	isConnected         bool
	keepAliveTicker     *time.Ticker
	lastReceivedTicket  *time.Ticker
	lastReceived        time.Time
	done                chan struct{}
	reconnectInProgress bool
	apiKey              string
	secretKey           string
	passphrase          string
	isDemoTrading       bool
	handlers            map[string]func([]byte)
}

type WSMessage struct {
	Event string          `json:"event"`
	Code  int             `json:"code"`
	Msg   string          `json:"msg"`
	Arg   WSSubscription  `json:"arg"`
	Data  json.RawMessage `json:"data"`
}

type WSSubscription struct {
	InstType string `json:"instType"`
	Channel  string `json:"channel"`
	InstId   string `json:"instId"`
}

func NewWebsocketClient(apiKey, secretKey, passphrase string, isDemoTrading bool) (*WebsocketClient, error) {
	u, err := url.Parse(wsEndpoint)
	if err != nil {
		return nil, err
	}

	c := &WebsocketClient{
		keepAliveTicker:    time.NewTicker(15 * time.Second),
		lastReceivedTicket: time.NewTicker(1 * time.Second),
		lastReceived:       time.Now(),
		done:               make(chan struct{}),
		apiKey:             apiKey,
		secretKey:          secretKey,
		passphrase:         passphrase,
		isDemoTrading:      isDemoTrading,
		handlers:           make(map[string]func([]byte)),
	}

	if err := c.connect(u.String()); err != nil {
		return nil, err
	}

	go c.readLoop()
	go c.keepAlive()
	go c.monitorReceived()

	log.Println("Websocket client created")

	log.Println("Attempting to send subscribe message...")
	if err = c.subscribe(); err != nil {
		return nil, fmt.Errorf("failed to send subscribe message: %w", err)
	}

	return c, nil
}

func (c *WebsocketClient) connect(endpoint string) error {
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.isConnected = true
	c.mu.Unlock()

	return c.authenticate()
}

func (c *WebsocketClient) authenticate() error {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	sign := c.sign(timestamp)

	auth := map[string]interface{}{
		"op": "login",
		"args": []map[string]string{{
			"apiKey":     c.apiKey,
			"passphrase": c.passphrase,
			"timestamp":  timestamp,
			"sign":       sign,
		}},
	}

	authMsg, err := c.toJson(auth)
	if err != nil {
		return err
	}

	if err := c.Send(authMsg); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	return nil
}

func (c *WebsocketClient) sign(timestamp string) string {
	message := timestamp + "GET" + "/user/verify"
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func (c *WebsocketClient) toJson(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal data: %w", err)
	}
	return string(jsonData), nil
}

func (c *WebsocketClient) Send(data string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected {
		return fmt.Errorf("not connected")
	}
	return c.conn.WriteMessage(websocket.TextMessage, []byte(data))
}

func (c *WebsocketClient) RegisterHandler(handler SubscriptionHandler) {
	c.handlers["default"] = handler
}

func (c *WebsocketClient) subscribe() error {
	instType := "USDT-FUTURES"
	if c.isDemoTrading {
		instType = "SUSDT-FUTURES"
	}

	sub := map[string]interface{}{
		"op": "subscribe",
		"args": []WSSubscription{{
			InstType: instType,
			Channel:  "orders",
			InstId:   "default", // all trading pairs
		}},
	}
	return c.conn.WriteJSON(sub)
}

// validateSymbol checks if the symbol format matches the trading mode
func (c *WebsocketClient) validateSymbol(symbol string) error {
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

func (c *WebsocketClient) keepAlive() {
	for {
		select {
		case <-c.keepAliveTicker.C:
			if !c.isConnected {
				continue
			}

			err := c.Send("ping")
			if err != nil {
				log.Printf("ping failed: %v", err)
				log.Println("keepAlive: calling reconnect")
				c.reconnect()
				continue
			}
			log.Println("Sent ping")

		case <-c.done:
			log.Println("Done channel closed, exiting keepAlive")
			return // Exit the keepAlive goroutine when done channel is closed
		}
	}
}

func (c *WebsocketClient) monitorReceived() {
	for {
		select {
		case <-c.lastReceivedTicket.C:
			if !c.isConnected {
				continue
			}
			if time.Since(c.lastReceived).Seconds() > 50 { // bitget disconnects after 60 seconds of inactivity
				log.Println("Last received message was more than 50 seconds ago")
				log.Println("monitorReceived: calling reconnect")
				c.reconnect()
			}
		case <-c.done:
			log.Println("Done channel closed, exiting monitorReceived")
			return
		}
	}
}

func (c *WebsocketClient) reconnect() {
	if c.reconnectInProgress {
		log.Println("reconnect already in progress")
		return
	}

	c.mu.Lock()
	c.reconnectInProgress = true
	c.mu.Unlock()

	c.disconnect()

	for {
		log.Println("attempting to reconnect...")
		if err := c.connect(wsEndpoint); err != nil {
			log.Println("reconnect failed")
			time.Sleep(5 * time.Second)
			continue
		}
		log.Print("reconnected successfully")

		c.mu.Lock()
		c.reconnectInProgress = false
		c.lastReceived = time.Now()
		c.mu.Unlock()

		if err := c.subscribe(); err != nil {
			log.Printf("failed to send re-subscribe request: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Print("sent re-subscribe request successfully")

		return
	}
}

func (c *WebsocketClient) disconnect() {
	if err := c.closeConnection(); err != nil {
		log.Printf("failed to close connection: %v", err)
	}
}

func (c *WebsocketClient) closeConnection() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.isConnected = false
		return c.conn.Close()
	}
	return nil
}

func (c *WebsocketClient) readLoop() {
	for {
		select {
		case <-c.done:
			log.Println("Done channel closed, exiting read loop")
			return
		default:
			if !c.isConnected {
				continue
			}
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				log.Printf("read error: %v", err)
				log.Println("readLoop: calling reconnect")
				c.reconnect()
				continue
			}

			c.lastReceived = time.Now()
			var msg WSMessage
			if string(message) == "pong" {
				log.Print("Received pong")
				continue
			}

			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("parse error: %v \n message: %s", err, string(message))
				continue
			}

			if msg.Event == "error" && msg.Code == 30004 && strings.Contains(msg.Msg, "not logged in") {
				if err := c.authenticate(); err != nil {
					log.Fatalf("failed to re-authenticate: %v", err)
				}
				continue
			}

			if msg.Event == "error" && msg.Code == 30006 && strings.Contains(msg.Msg, "request too many") {
				log.Printf("Received error message: %s", string(message))
				log.Println("This sometimes seems to happen on trying to subscribe to a channel")
				log.Println("readLoop: calling subscribe (to ensure we are subscribed)")
				if err := c.subscribe(); err != nil {
					log.Fatalf("failed to send re-subscribe message: %v", err)
				}
				continue

			}

			if msg.Event == "error" {
				log.Printf("Received error message: %s", string(message))
				continue
			}

			if msg.Event == "login" {
				log.Print("Received login event")
				continue
			}

			if msg.Event == "subscribe" {
				log.Printf(string(message))
				log.Printf("Subscribed to %s on channel %s", msg.Arg.InstType, msg.Arg.Channel)
				continue
			}

			log.Printf("Received message: %s", string(message))

			if handler, ok := c.handlers["default"]; ok {
				handler(msg.Data)
			}
		}
	}
}

func (c *WebsocketClient) Close() error {
	c.mu.Lock()
	close(c.done)
	c.mu.Unlock()

	return c.closeConnection()
}
