package main

import (
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Setup log file next to the executable.
	exePath, _ := os.Executable()
	logPath := filepath.Join(filepath.Dir(exePath), "relay-client.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Fatalf("open log file %s: %v", logPath, err)
	}
	defer logFile.Close()
	log.SetOutput(logFile)

	// Load config.
	cfg, err := LoadClientConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Setup in-memory message store.
	msgs := NewMessageStore()

	// Init clipboard.
	if err := InitClipboard(); err != nil {
		log.Printf("[main] clipboard init failed (non-fatal): %v", err)
	}

	// Create WS client.
	ws := NewWSClient(cfg)

	// Wire up SMS handler.
	ws.SetOnSms(func(sd SmsData) {
		// Store in memory.
		msgs.InsertMessage(sd.Title, sd.Msg, sd.SmsCode, sd.ReceivedAt)
		// System notification.
		ShowNotification(sd)
		// Copy to clipboard.
		CopyToClipboard(sd.SmsCode)
		// Update tray.
		UpdateTrayLatest(sd.SmsCode)
	})

	// Wire up sync handler (offline messages replayed on reconnect).
	ws.SetOnSync(func(batch []SmsData) {
		for _, sd := range batch {
			msgs.InsertMessage(sd.Title, sd.Msg, sd.SmsCode, sd.ReceivedAt)
		}
		// Summary notification for the latest one.
		if len(batch) > 0 {
			last := batch[len(batch)-1]
			ShowNotification(last)
			CopyToClipboard(last.SmsCode)
			UpdateTrayLatest(last.SmsCode)
		}
	})

	// Wire up status handler.
	ws.SetOnStatus(func(ok bool) {
		UpdateTrayStatus(ok)
	})

	// Connect in background.
	ws.Connect()

	// Start local web server.
	webSrv, err := StartWebServer(cfg.WebPort, msgs, &cfg, ws)
	if err != nil {
		log.Fatalf("web server: %v", err)
	}

	// Handle graceful shutdown signals.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("[main] shutting down...")
		ws.Close()
		webSrv.Close()
		os.Exit(0)
	}()

	// Block on systray (runs until Quit).
	StartTray(cfg.WebPort, nil, func() {
		ws.Close()
		webSrv.Close()
	})
}
