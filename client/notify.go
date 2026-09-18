package main

import (
	"log"

	"github.com/gen2brain/beeep"
)

// init sets the application name shown on Windows toast notifications.
// beeep forwards it to go-toast, which writes it to the registry key
// HKCU\SOFTWARE\Classes\AppUserModelId\<AppID>\DisplayName — Windows then
// displays that name on the toast (beeep's default is "DefaultAppName").
func init() {
	beeep.AppName = "Relay"
}

// ShowNotification displays a Windows toast notification.
// Title and body depend on the message kind:
//   - "notify": title = the notification's own title ("新通知" fallback),
//     body = its content
//   - otherwise (SMS, incl. messages from older servers without kind):
//     title = "新验证码", body = the code (+ the message text if short)
func ShowNotification(sd SmsData) {
	var title, body string
	switch sd.Kind {
	case "notify":
		title = sd.Title
		if title == "" {
			title = "新通知"
		}
		body = sd.Msg
	default:
		title = "新验证码"
		body = sd.SmsCode
		if sd.Msg != "" && len(sd.Msg) < 100 {
			body = sd.SmsCode + "\n" + sd.Msg
		}
	}
	if err := beeep.Notify(title, body, ""); err != nil {
		log.Printf("[notify] error: %v", err)
	}
}
