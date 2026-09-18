package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

// StartSmsTCPServer listens for SMS verification code messages on the given port.
func StartSmsTCPServer(port int, hub *Hub) error {
	return startTCP(port, "sms", func(line []byte) (PhoneMsg, error) {
		var pm PhoneMsg
		if err := json.Unmarshal(line, &pm); err != nil {
			return pm, err
		}
		if pm.Msg == "" {
			return pm, fmt.Errorf("empty msg")
		}
		return pm, nil
	}, hub)
}

// StartNotifyTCPServer listens for notification messages on the given port.
// Expects JSON: {"title":"...", "content":"..."}
// Title and content stay separate so the desktop client can show a proper
// toast title; msg carries the content only.
func StartNotifyTCPServer(port int, hub *Hub) error {
	return startTCP(port, "notify", func(line []byte) (PhoneMsg, error) {
		var nm struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(line, &nm); err != nil {
			return PhoneMsg{}, err
		}
		if nm.Title == "" && nm.Content == "" {
			return PhoneMsg{}, fmt.Errorf("empty title and content")
		}
		return PhoneMsg{Msg: nm.Content, SmsCode: "", Title: nm.Title}, nil
	}, hub)
}

// startTCP is a generic TCP listener that parses lines with the given parser
// and broadcasts/buffers the result via the Hub.
func startTCP(port int, label string, parse func([]byte) (PhoneMsg, error), hub *Hub) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("[%s] tcp listen: %w", label, err)
	}
	log.Printf("[%s] listening on %s", label, addr)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Printf("[%s] accept error: %v", label, err)
				continue
			}
			log.Printf("[%s] new connection from %s", label, conn.RemoteAddr())
			go handleTCPConn(conn, label, parse, hub)
		}
	}()
	return nil
}

func handleTCPConn(conn net.Conn, label string, parse func([]byte) (PhoneMsg, error), hub *Hub) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if line[0] != '{' {
			preview := line
			if len(preview) > 40 {
				preview = preview[:40]
			}
			log.Printf("[%s] non-json data from %s: %q", label, conn.RemoteAddr(), string(preview))
			continue
		}
		pm, err := parse(line)
		if err != nil {
			log.Printf("[%s] bad data from %s: %v", label, conn.RemoteAddr(), err)
			continue
		}
		log.Printf("[%s] msg from %s: code=%s", label, conn.RemoteAddr(), pm.SmsCode)
		hub.BroadcastOrBuffer(pm, label)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[%s] read error from %s: %v", label, conn.RemoteAddr(), err)
	}
}
