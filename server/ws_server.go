package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

const (
	writeWait      = 10 * time.Second
	pingWait       = 2 * time.Minute // client pings every 1 min; dead if silent for this long
	maxMessageSize = 4096
)

func StartWSServer(port int, authToken string, hub *Hub) error {
	addr := fmt.Sprintf("0.0.0.0:%d", port)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWS(w, r, authToken, hub)
	})
	log.Printf("[ws] listening on %s", addr)
	return http.ListenAndServe(addr, nil)
}

func handleWS(w http.ResponseWriter, r *http.Request, authToken string, hub *Hub) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade error: %v", err)
		return
	}
	defer conn.Close()

	// Authenticate with a short timeout.
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		log.Printf("[ws] read auth: %v", err)
		return
	}
	var authMsg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &authMsg); err != nil {
		preview := raw
		if len(preview) > 80 {
			preview = preview[:80]
		}
		log.Printf("[ws] bad auth json from %s: %v (raw: %q)", r.RemoteAddr, err, string(preview))
		conn.WriteJSON(WSMsg{Type: "auth_fail"})
		return
	}
	if authMsg.Type != "auth" || authMsg.Token != authToken {
		conn.WriteJSON(WSMsg{Type: "auth_fail"})
		log.Printf("[ws] auth failed from %s", r.RemoteAddr)
		return
	}
	conn.WriteJSON(WSMsg{Type: "auth_ok"})
	log.Printf("[ws] client authenticated: %s", r.RemoteAddr)

	// Sync any messages buffered while the client was offline.
	syncMsgs := hub.DrainBuffer()
	if len(syncMsgs) > 0 {
		syncData, _ := json.Marshal(syncMsgs)
		conn.WriteJSON(WSMsg{Type: "sync", Data: syncData})
		log.Printf("[ws] synced %d buffered messages to %s", len(syncMsgs), r.RemoteAddr)
	}

	client := &WSClient{conn: &wsConnAdapter{conn: conn}}
	hub.Register(client)
	defer hub.Unregister(client)

	// Heartbeat: the client sends a ping every 1 minute; reply with a pong
	// and reset the read deadline. Connections silent for pingWait are closed.
	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pingWait))
	conn.SetPingHandler(func(appData string) error {
		conn.SetReadDeadline(time.Now().Add(pingWait))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(writeWait))
	})

	// Block reading until the connection closes.
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[ws] read error: %v", err)
			}
			return
		}
	}
}

// wsConnAdapter makes *websocket.Conn satisfy WriteCloser.
type wsConnAdapter struct {
	conn *websocket.Conn
}

func (a *wsConnAdapter) WriteJSON(v interface{}) error {
	return a.conn.WriteJSON(v)
}

func (a *wsConnAdapter) Close() error {
	return a.conn.Close()
}
