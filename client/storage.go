package main

import (
	"sync"
	"sync/atomic"
)

type Message struct {
	ID         int64  `json:"id"`
	Title      string `json:"title,omitempty"`
	Msg        string `json:"msg"`
	SmsCode    string `json:"sms_code"`
	ReceivedAt string `json:"received_at"`
}

// MessageStore holds messages received during this session (in-memory only).
type MessageStore struct {
	mu       sync.RWMutex
	messages []Message
	nextID   atomic.Int64
	notify   chan struct{}
}

func NewMessageStore() *MessageStore {
	return &MessageStore{
		notify: make(chan struct{}, 1),
	}
}

// Notify returns a channel that receives a signal whenever a new message
// arrives. Buffer 1 + non-blocking send means signals may be coalesced,
// which is fine for the UI's full-refresh-on-event model.
func (s *MessageStore) Notify() <-chan struct{} {
	return s.notify
}

func (s *MessageStore) InsertMessage(title, msg, smsCode, receivedAt string) {
	s.mu.Lock()
	id := s.nextID.Add(1)
	s.messages = append(s.messages, Message{
		ID:         id,
		Title:      title,
		Msg:        msg,
		SmsCode:    smsCode,
		ReceivedAt: receivedAt,
	})
	s.mu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func (s *MessageStore) ListMessages(page, limit int) ([]Message, int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := len(s.messages)
	// Reverse order: newest first.
	result := make([]Message, 0, limit)
	startIdx := total - (page-1)*limit - 1
	endIdx := startIdx - limit + 1
	if startIdx < 0 {
		return result, total
	}
	if endIdx < 0 {
		endIdx = 0
	}
	for i := startIdx; i >= endIdx; i-- {
		result = append(result, s.messages[i])
	}
	return result, total
}

func (s *MessageStore) ClearMessages() {
	s.mu.Lock()
	s.messages = nil
	s.nextID.Store(0)
	s.mu.Unlock()
}

// LatestMessage returns the most recently received message, if any.
func (s *MessageStore) LatestMessage() *Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.messages) == 0 {
		return nil
	}
	m := s.messages[len(s.messages)-1]
	return &m
}
