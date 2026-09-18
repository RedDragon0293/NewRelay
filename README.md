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
   sms_tcp_port: 1885           # 手机短信验证码 TCP 连接端口
   notify_tcp_port: 1884        # 手机应用通知 TCP 连接端口
   ws_port: 9091                # 客户端 WebSocket 端口
   auth_token: "token"          # 客户端认证 token
   log_file: "relay-server.log" # log 文件名
   max_buffer: 500              # 客户端离线时暂存消息上限
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

## 3. 使用说明

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
