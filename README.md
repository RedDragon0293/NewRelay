# Relay - 手机短信验证码同步

将手机收到的短信验证码自动同步到 Windows 电脑：系统通知 + 剪贴板复制 + 历史管理。

## 架构

```
手机 (TCP) ──→ Linux 服务器 ──WebSocket──→ Windows 电脑
               (relay-server)              (relay-client)
```

## 目录

```
NewRelay/
├── server/             # Linux 中转服务器 (Go)
│   ├── main.go
│   ├── config.go
│   ├── hub.go
│   ├── tcp_server.go
│   ├── ws_server.go
│   └── config.yaml
├── client/             # Windows 客户端 (Go)
│   ├── main.go
│   ├── config.go
│   ├── storage.go
│   ├── websocket.go
│   ├── systray.go
│   ├── clipboard.go
│   ├── notify.go
│   ├── web_server.go
│   ├── web/            # 内嵌 Web UI
│   │   ├── index.html
│   │   ├── style.css
│   │   └── app.js
│   └── icon.ico
└── README.md
```

---

## 1. 服务器部署 (Linux)

### 编译

```bash
cd server
GOOS=linux GOARCH=amd64 go build -o relay-server .
```

### 配置

编辑 `config.yaml`：

```yaml
server:
  tcp_port: 9090       # 手机 TCP 连接端口
  ws_port: 9091        # 电脑 WebSocket 端口
  auth_token: "你的密钥" # 客户端认证 token
```

### 运行

```bash
./relay-server config.yaml
```

### systemd 部署（推荐）

项目根目录提供了 `relay-server.service` 文件：

```bash
# 1. 创建目录并放入文件
sudo mkdir -p /opt/relay
sudo cp relay-server config.yaml /opt/relay/
sudo cp relay-server.service /etc/systemd/system/

# 2. 编辑配置（修改 auth_token）
sudo nano /opt/relay/config.yaml

# 3. 启动服务
sudo systemctl daemon-reload
sudo systemctl enable relay-server --now

# 4. 查看状态 / 日志
sudo systemctl status relay-server
sudo journalctl -u relay-server -f
```

---

## 2. 电脑客户端 (Windows)

### 编译

```bash
cd client
go build -o relay-client.exe .
```

### 配置

首次运行会在 exe 同目录生成 `config.json`：

```json
{
  "server_host": "你的服务器IP",
  "ws_port": 9091,
  "auth_token": "你的密钥",
  "web_port": 19800
}
```

也可通过托盘菜单 → 打开窗口 → 设置页面修改。

### 运行

双击 `relay-client.exe`，系统托盘会出现 Relay 图标。

---

## 3. 手机端对接

手机端通过 TCP Socket 连接到服务器的 `tcp_port`，发送单行 JSON：

```json
{"msg":"【xx银行】您的转账验证码为123456，请勿泄露。","sms_code":"123456"}
```

| 字段 | 说明 |
|------|------|
| `msg` | 短信原文 |
| `sms_code` | 提取的验证码 |

### Android 示例 (Tasker)

在 Tasker 中创建一个任务：
1. **Event** → Phone → Received Text（收到短信时触发）
2. **Action** → Net → HTTP Request：
   - Method: `POST`（或用 TCP 插件）
   - 或使用 "TCP Client" 插件发送 JSON

也可使用 MacroDroid、Automate 等自动化工具，或专门的短信转发 App。

---

## 4. 使用说明

1. 启动 Linux 服务器
2. 启动 Windows 客户端（系统托盘出现图标）
3. 手机收到验证码 → 自动发送到服务器 → 电脑收到

电脑端：
- **系统通知**：Windows Toast 弹窗
- **剪贴板**：验证码自动复制，直接 Ctrl+V 粘贴
- **托盘**：显示最新验证码和连接状态
- **窗口**：点击"打开窗口"查看历史记录、搜索、手动复制、调整设置

---

## 依赖

| 库 | 用途 |
|---|---|
| `gorilla/websocket` | WebSocket 通信 |
| `gopkg.in/yaml.v3` | 服务器配置解析 |
| `getlantern/systray` | Windows 系统托盘 |
| `modernc.org/sqlite` | 纯 Go SQLite 存储 |
| `golang.design/x/clipboard` | 系统剪贴板 |
| `github.com/gen2brain/beeep` | Windows Toast 通知 |
| `github.com/pkg/browser` | 打开默认浏览器 |

## 端口一览

| 端口 | 方向 | 协议 |
|------|------|------|
| 9090 | 手机 → 服务器 | TCP |
| 9091 | 客户端 → 服务器 | WebSocket |
| 19800 | 浏览器 → 客户端 | HTTP (仅本地) |
