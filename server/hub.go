package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"
)

// PhoneMsg is the JSON message received from the phone via TCP.
// Title is server-side only (filled by the notify channel parser);
// json:"-" keeps the phone's TCP JSON format unaffected.
type PhoneMsg struct {
	Msg     string `json:"msg"`
	SmsCode string `json:"sms_code"`
	Title   string `json:"-"`
}

// WSMsg is the JSON envelope for server↔client WebSocket communication.
type WSMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// SmsData is the payload sent to the desktop client.
// Kind is "sms" (verification-code SMS) or "notify" (generic notification);
// Title carries the original title for "notify" messages.
type SmsData struct {
	Kind       string `json:"kind,omitempty"`
	Title      string `json:"title,omitempty"`
	Msg        string `json:"msg"`
	SmsCode    string `json:"sms_code"`
	ReceivedAt string `json:"received_at"`
}

// MakeSmsData creates an SmsData from a phone message.
// kind is "sms" or "notify" — the source channel of the phone message.
func MakeSmsData(pm PhoneMsg, kind string) SmsData {
	return SmsData{
		Kind:       kind,
		Title:      pm.Title,
		Msg:        pm.Msg,
		SmsCode:    pm.SmsCode,
		ReceivedAt: time.Now().Format(time.RFC3339),
	}
}

// MakeSmsMsg constructs a WSMsg with type "sms".
func MakeSmsMsg(sd SmsData) WSMsg {
	data, _ := json.Marshal(sd)
	return WSMsg{Type: "sms", Data: data}
}

// WSClient represents an authenticated desktop client connection.
type WSClient struct {
	conn WriteCloser
	mu   sync.Mutex
}

type WriteCloser interface {
	WriteJSON(v interface{}) error
	Close() error
}

// Hub manages connected desktop clients and offline message buffering.
type Hub struct {
	mu        sync.Mutex
	clients   map[*WSClient]struct{}
	buffer    []SmsData
	maxBuffer int
}

func NewHub(maxBuffer int) *Hub {
	if maxBuffer <= 0 {
		maxBuffer = 500
	}
	return &Hub{
		clients:   make(map[*WSClient]struct{}),
		maxBuffer: maxBuffer,
	}
}

func (h *Hub) Register(c *WSClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
	log.Printf("[hub] client registered (total: %d)", len(h.clients))
}

func (h *Hub) Unregister(c *WSClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	log.Printf("[hub] client unregistered (total: %d)", len(h.clients))
}

// HasClients returns true if at least one client is connected.
func (h *Hub) HasClients() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients) > 0
}

// BroadcastOrBuffer sends a message to every connected client, or buffers it
// if none are online. kind is "sms" or "notify" (source channel).
func (h *Hub) BroadcastOrBuffer(pm PhoneMsg, kind string) {
	sd := MakeSmsData(pm, kind)
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.clients) > 0 {
		msg := MakeSmsMsg(sd)
		for c := range h.clients {
			c.mu.Lock()
			err := c.conn.WriteJSON(msg)
			c.mu.Unlock()
			if err != nil {
				log.Printf("[hub] write error: %v", err)
			}
		}
	} else {
		// No clients online — buffer for later sync.
		h.buffer = append(h.buffer, sd)
		if len(h.buffer) > h.maxBuffer {
			// Drop oldest.
			drop := len(h.buffer) - h.maxBuffer
			h.buffer = h.buffer[drop:]
			log.Printf("[hub] buffer overflow, dropped %d oldest", drop)
		}
		log.Printf("[hub] buffered sms (buffer: %d/%d)", len(h.buffer), h.maxBuffer)
	}
}

// DrainBuffer returns all buffered messages and clears the buffer.
func (h *Hub) DrainBuffer() []SmsData {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.buffer) == 0 {
		return nil
	}
	drained := h.buffer
	h.buffer = nil
	log.Printf("[hub] drained %d buffered messages", len(drained))
	return drained
}
