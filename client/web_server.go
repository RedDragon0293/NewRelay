package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
)

//go:embed web/*
var webFiles embed.FS

type apiHandler struct {
	store  *MessageStore
	config *ClientConfig
	ws     *WSClient
}

func StartWebServer(port int, st *MessageStore, cfg *ClientConfig, ws *WSClient) (*http.Server, error) {
	h := &apiHandler{store: st, config: cfg, ws: ws}

	mux := http.NewServeMux()

	// API endpoints.
	mux.HandleFunc("/api/messages", h.handleMessages)
	mux.HandleFunc("/api/settings", h.handleSettings)
	mux.HandleFunc("/api/status", h.handleStatus)
	mux.HandleFunc("/api/events", h.handleEvents)

	// Static files (embedded).
	webFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	srv := &http.Server{
		Addr:    "127.0.0.1:" + strconv.Itoa(port),
		Handler: mux,
	}
	go func() {
		log.Printf("[web] listening on http://%s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[web] error: %v", err)
		}
	}()
	return srv, nil
}

func (h *apiHandler) handleMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if page < 1 {
			page = 1
		}
		if limit < 1 || limit > 100 {
			limit = 20
		}
		msgs, total := h.store.ListMessages(page, limit)
		if msgs == nil {
			msgs = []Message{}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"messages": msgs,
			"total":    total,
			"page":     page,
			"limit":    limit,
		})
	case http.MethodDelete:
		h.store.ClearMessages()
		w.Write([]byte(`{"ok":true}`))
	default:
		http.Error(w, `{"error":"method not allowed"}`, 405)
	}
}

func (h *apiHandler) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		json.NewEncoder(w).Encode(h.config)
	case http.MethodPut:
		var newCfg ClientConfig
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, `{"error":"bad json"}`, 400)
			return
		}
		// Only update connection-related fields.
		if newCfg.ServerHost != "" {
			h.config.ServerHost = newCfg.ServerHost
		}
		if newCfg.WSPort > 0 {
			h.config.WSPort = newCfg.WSPort
		}
		if newCfg.AuthToken != "" {
			h.config.AuthToken = newCfg.AuthToken
		}
		if err := SaveClientConfig(*h.config); err != nil {
			http.Error(w, `{"error":"save failed"}`, 500)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"ok": "saved, restart to apply"})
	default:
		http.Error(w, `{"error":"method not allowed"}`, 405)
	}
}

func (h *apiHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"connected": connected,
		"server":    h.config.ServerHost + ":" + strconv.Itoa(h.config.WSPort),
	})
}

// handleEvents streams server-sent events to the UI: a "message" event is
// pushed whenever a new SMS arrives, so the page updates without manual
// refresh. An initial event is sent on connect, which also covers the
// browser's automatic SSE reconnect after a gap (missed events are caught up).
func (h *apiHandler) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming unsupported"}`, http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "event: message\ndata: {}\n\n")
	flusher.Flush()

	notify := h.store.Notify()
	for {
		select {
		case <-notify:
			fmt.Fprintf(w, "event: message\ndata: {}\n\n")
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
