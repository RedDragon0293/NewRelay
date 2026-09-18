package main

import (
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfgPath := "config.yaml"
	if len(os.Args) > 1 {
		cfgPath = os.Args[1]
	}
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Setup log file. If it fails (e.g. permission denied under systemd),
	// fall back to stdout only — journald will capture it.
	logPath := cfg.ResolveLogPath(cfgPath)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("WARNING: cannot open log file %s: %v (logging to stdout only)", logPath, err)
	} else {
		defer logFile.Close()
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	}

	hub := NewHub(cfg.Server.MaxBuffer)

	if err := StartSmsTCPServer(cfg.Server.SmsTCPPort, hub); err != nil {
		log.Fatalf("sms tcp server: %v", err)
	}
	if err := StartNotifyTCPServer(cfg.Server.NotifyTCPPort, hub); err != nil {
		log.Fatalf("notify tcp server: %v", err)
	}

	go func() {
		if err := StartWSServer(cfg.Server.WSPort, cfg.Server.AuthToken, hub); err != nil {
			log.Fatalf("ws server: %v", err)
		}
	}()

	log.Printf("relay-server started (sms_tcp=%d, notify_tcp=%d, ws=%d)", cfg.Server.SmsTCPPort, cfg.Server.NotifyTCPPort, cfg.Server.WSPort)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down")
}
