package main

import (
	"log"

	"golang.design/x/clipboard"
)

func InitClipboard() error {
	return clipboard.Init()
}

// CopyToClipboard writes text to the system clipboard.
func CopyToClipboard(text string) {
	if text == "" {
		return
	}
	clipboard.Write(clipboard.FmtText, []byte(text))
	log.Printf("[clipboard] copied: %s", text)
}
