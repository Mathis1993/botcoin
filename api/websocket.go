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

type WebsocketClient struct {
	conn          *websocket.Conn
	mu            sync.Mutex
	isConnected   bool
	done          chan struct{}
	readerDone    chan struct{}
	apiKey        string
	secretKey     string
	passphrase    string
	isDemoTrading bool
	handlers      map[string]func([]byte)
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
		done:          make(chan struct{}),
		readerDone:    make(chan struct{}),
		apiKey:        apiKey,
		secretKey:     secretKey,
		passphrase:    passphrase,
		isDemoTrading: isDemoTrading,
		handlers:      make(map[string]func([]byte)),
	}

	if err := c.connect(u.String()); err != nil {
		return nil, err
	}

	go c.keepAlive()
	go c.readMessages()

	return c, nil
}

func (c *WebsocketClient) connect(endpoint string) error {
	log.Print("Waiting for mutex...")
	c.mu.Lock()
	log.Print("Acquired mutex")
	defer c.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		return fmt.Errorf("websocket connection failed: %w", err)
	}

	c.conn = conn
	c.isConnected = true

	return c.authenticate()
}

func (c *WebsocketClient) authenticate() error {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
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

	if err := c.conn.WriteJSON(auth); err != nil {
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

func (c *WebsocketClient) Subscribe(handler func([]byte)) error {
	c.handlers["default"] = handler

	return c.subscribe()
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
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if !c.isConnected {
				c.mu.Unlock()
				continue
			}

			err := c.conn.WriteMessage(websocket.PingMessage, []byte("ping"))
			if err != nil {
				log.Printf("ping failed: %v", err)
				c.mu.Unlock()
				c.reconnect()
				continue
			}

			err = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
			if err != nil {
				log.Print("failed to set read deadline")
			}

			_, message, err := c.conn.ReadMessage()
			if err != nil || string(message) != "pong" {
				log.Printf("pong not received: %v", err)
				c.mu.Unlock()
				c.reconnect()
				continue
			}

			if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
				log.Print("failed to clear read deadline")
			}

			c.mu.Unlock()

		case <-c.done:
			return // Exit the keepAlive goroutine when done channel is closed
		}
	}
}

func (c *WebsocketClient) reconnect() {
	c.disconnect()

	c.mu.Lock()
	c.readerDone = make(chan struct{})
	c.mu.Unlock()

	for {
		log.Println("attempting to reconnect...")
		if err := c.connect(wsEndpoint); err != nil {
			log.Println("reconnect failed")
			time.Sleep(5 * time.Second)
			continue
		}
		log.Print("reconnected successfully")

		if err := c.subscribe(); err != nil {
			log.Printf("failed to re-subscribe: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Print("re-subscribed successfully")

		go c.readMessages()
		return
	}
}

func (c *WebsocketClient) disconnect() {
	c.mu.Lock()
	c.isConnected = false
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			log.Printf("failed to close connection: %v", err)
		}
	}
	close(c.readerDone)
	c.mu.Unlock()
}

func (c *WebsocketClient) readMessages() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("read error: %v", err)
			c.reconnect()
			return
		}

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

		if msg.Event == "error" {
			log.Printf("Received error message: %s", string(message))
			// ToDo(ME-29.01.25): Maybe reconnect for severe errors (what are these?)
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

func (c *WebsocketClient) Close() error {
	close(c.done)
	close(c.readerDone)
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.isConnected = false
		return c.conn.Close()
	}
	return nil
}
