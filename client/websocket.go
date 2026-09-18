package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Heartbeat: the client sends a ping every pingPeriod; the server replies
// with a pong that refreshes the read deadline. pongWait must exceed
// pingPeriod so a pong always arrives in time.
const (
	pingPeriod = 1 * time.Minute
	pongWait   = 2 * time.Minute
	writeWait  = 10 * time.Second
)

// WSMsg mirrors the server's WSMsg type.
type WSMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// SmsData is the payload for "sms"/"sync" messages.
// Kind mirrors the server: "sms" (verification SMS) or "notify" (generic
// notification); Title carries the original title for "notify" messages.
type SmsData struct {
	Kind       string `json:"kind,omitempty"`
	Title      string `json:"title,omitempty"`
	Msg        string `json:"msg"`
	SmsCode    string `json:"sms_code"`
	ReceivedAt string `json:"received_at"`
}

type WSClient struct {
	cfg        ClientConfig
	conn       *websocket.Conn
	mu         sync.Mutex
	onSms      func(SmsData)
	onSync     func([]SmsData)
	onStatus   func(bool)
	reconnectC chan struct{}
	closed     bool
}

func NewWSClient(cfg ClientConfig) *WSClient {
	return &WSClient{
		cfg:        cfg,
		reconnectC: make(chan struct{}, 1),
	}
}

func (w *WSClient) SetOnSms(fn func(SmsData))   { w.onSms = fn }
func (w *WSClient) SetOnSync(fn func([]SmsData)) { w.onSync = fn }
func (w *WSClient) SetOnStatus(fn func(bool))    { w.onStatus = fn }

func (w *WSClient) Connect() {
	go w.loop()
}

func (w *WSClient) loop() {
	backoff := 1 * time.Second
	for {
		w.mu.Lock()
		if w.closed {
			w.mu.Unlock()
			return
		}
		w.mu.Unlock()

		if err := w.dial(); err != nil {
			log.Printf("[ws] connect: %v (retry in %v)", err, backoff)
			w.setStatus(false)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			continue
		}
		backoff = 1 * time.Second
		w.setStatus(true)

		w.readLoop()

		w.setStatus(false)
		w.mu.Lock()
		if w.conn != nil {
			w.conn.Close()
			w.conn = nil
		}
		w.mu.Unlock()

		time.Sleep(1 * time.Second)
	}
}

func (w *WSClient) dial() error {
	u := url.URL{
		Scheme: "ws",
		Host:   fmt.Sprintf("%s:%d", w.cfg.ServerHost, w.cfg.WSPort),
		Path:   "/ws",
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	// Authenticate — send type & token at top level (matching server's flat parse).
	authMsg := struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}{Type: "auth", Token: w.cfg.AuthToken}
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("send auth: %w", err)
	}
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		conn.Close()
		return fmt.Errorf("read auth resp: %w", err)
	}
	var resp WSMsg
	if err := json.Unmarshal(raw, &resp); err != nil || resp.Type != "auth_ok" {
		conn.Close()
		return fmt.Errorf("auth failed: %s", string(raw))
	}
	// Heartbeat setup: a pong from the server refreshes the read deadline
	// (pings are sent by readLoop's ticker). Also answer pings from a
	// legacy server just in case.
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	conn.SetPingHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeWait))
	})
	w.mu.Lock()
	w.conn = conn
	w.mu.Unlock()
	log.Printf("[ws] connected to %s", u.Host)
	return nil
}

func (w *WSClient) readLoop() {
	w.mu.Lock()
	conn := w.conn
	w.mu.Unlock()
	if conn == nil {
		return
	}

	// Heartbeat: send a ping every pingPeriod; the server replies with a
	// pong that refreshes the read deadline. A failed ping write means the
	// connection is dead — close it so loop() reconnects.
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(pingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
					log.Printf("[ws] ping write error: %v", err)
					conn.Close()
					return
				}
			case <-done:
				return
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[ws] read: %v", err)
			return
		}
		var msg WSMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("[ws] bad json: %v", err)
			continue
		}
		switch msg.Type {
		case "sms":
			var sd SmsData
			if err := json.Unmarshal(msg.Data, &sd); err != nil {
				log.Printf("[ws] bad sms data: %v", err)
				continue
			}
			if w.onSms != nil {
				w.onSms(sd)
			}
		case "sync":
			var batch []SmsData
			if err := json.Unmarshal(msg.Data, &batch); err != nil {
				log.Printf("[ws] bad sync data: %v", err)
				continue
			}
			log.Printf("[ws] received sync batch: %d messages", len(batch))
			if w.onSync != nil {
				w.onSync(batch)
			}
		default:
			log.Printf("[ws] unknown type: %s", msg.Type)
		}
	}
}

func (w *WSClient) setStatus(connected bool) {
	if w.onStatus != nil {
		w.onStatus(connected)
	}
}

func (w *WSClient) Close() {
	w.mu.Lock()
	w.closed = true
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
	w.mu.Unlock()
}
