package main

import (
	_ "embed"
	"log"
	"os"
	"strconv"

	"github.com/getlantern/systray"
	"github.com/pkg/browser"
)

//go:embed icon.ico
var trayIcon []byte

var (
	trayWebPort  int
	webOpenFn    func()
	quitFn       func()
	mStatusConn  *systray.MenuItem
	mStatusLabel *systray.MenuItem
	connected    bool
)

func StartTray(webPort int, onOpenWeb, onQuit func()) {
	trayWebPort = webPort
	webOpenFn = onOpenWeb
	quitFn = onQuit
	systray.Run(onReady, onExit)
}

func onReady() {
	systray.SetIcon(trayIcon)
	systray.SetTitle("Relay")
	systray.SetTooltip("Relay - 验证码同步")

	mOpen := systray.AddMenuItem("打开窗口", "打开验证码管理窗口")
	systray.AddSeparator()

	// Connection status indicator.
	mStatusConn = systray.AddMenuItem("连接: ● 断开", "")
	mStatusConn.Disable()

	// Latest SMS code preview.
	mStatusLabel = systray.AddMenuItem("暂无验证码", "")
	mStatusLabel.Disable()

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出程序")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				if webOpenFn != nil {
					webOpenFn()
				}
				browser.OpenURL("http://127.0.0.1:" + strconv.Itoa(trayWebPort))
			case <-mQuit.ClickedCh:
				systray.Quit()
				if quitFn != nil {
					quitFn()
				}
				os.Exit(0)
			}
		}
	}()
}

func onExit() {
	log.Println("[tray] exiting")
}

// UpdateTrayStatus updates the connection status shown in the tray.
func UpdateTrayStatus(isConnected bool) {
	connected = isConnected
	if mStatusConn == nil {
		return
	}
	if isConnected {
		mStatusConn.SetTitle("连接: ● 已连接")
	} else {
		mStatusConn.SetTitle("连接: ● 断开")
	}
}

// UpdateTrayLatest updates the latest SMS code preview.
func UpdateTrayLatest(smsCode string) {
	if mStatusLabel == nil {
		return
	}
	if smsCode == "" {
		mStatusLabel.SetTitle("暂无验证码")
	} else {
		mStatusLabel.SetTitle("最新: " + smsCode)
	}
}
