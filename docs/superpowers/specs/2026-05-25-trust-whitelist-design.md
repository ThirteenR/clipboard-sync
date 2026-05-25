# 剪贴板同步受信设备白名单系统

## 概述

为 clipboard-sync 增加受信设备白名单机制。LAN 中非受信设备发来的剪贴板内容被自动丢弃，用户通过 TUI 界面管理受信设备列表。

## 架构

### 新增文件

```
clipboard-sync/
└── trust/
    ├── trust.go    -- TrustStore 核心
    └── tui.go      -- TUI 管理界面
```

### 修改文件

| 文件 | 改动 |
|---|---|
| `main.go` | 初始化 TrustStore；onMessage 添加信任检查；添加 `trust` CLI 子命令 |
| `go.mod` | 添加 `github.com/charmbracelet/bubbletea` 依赖 |
| `install.sh` | 安装时创建 `~/.config/clipboardsync/trusted.json` |
| `install.ps1` | 安装时创建 `%APPDATA%\clipboardsync\trusted.json` |

**不修改**: `sync/`, `clipboard/`, `discovery/`, `dedup/` — 完全向后兼容。

## TrustStore (`trust/trust.go`)

### 数据文件

`~/.config/clipboardsync/trusted.json`（macOS/Linux）
`%APPDATA%\clipboardsync\trusted.json`（Windows）

结构：
```json
{
  "trusted_uuids": ["uuid-1111", "uuid-2222"],
  "devices": {
    "uuid-1111": { "hostname": "MacBook-Pro", "last_seen": "2026-05-25T10:00:00Z" },
    "uuid-2222": { "hostname": "DESKTOP-PC", "last_seen": "2026-05-25T09:30:00Z" }
  }
}
```

### 接口

```go
func New() (*TrustStore, error)                    // 加载或创建
func (ts *TrustStore) IsTrusted(uuid string) bool  // 检查是否受信
func (ts *TrustStore) Add(uuid, hostname string)   // 添加受信设备
func (ts *TrustStore) Remove(uuid string)          // 移除受信设备
func (ts *TrustStore) List() []DeviceInfo          // 列出所有已知设备及信任状态
```

## TUI 管理界面 (`trust/tui.go`)

依赖 `github.com/charmbracelet/bubbletea`。

### 功能

- 显示 LAN 上已发现设备列表（通过 mDNS 发现）
- 每行显示：`[*] MacBook-Pro (uuid: ...)` 或 `[ ] DESKTOP-PC`
- `↑/↓` 移动光标，`<space>` 切换信任，`s` 保存，`q` 退出

### CLI 子命令

```
clipboardsync                # 启动 daemon（后台运行）
clipboardsync trust          # 打开 TUI 管理界面
clipboardsync trust list     # 文本列表模式
clipboardsync trust add <uuid>  # 直接添加信任
clipboardsync trust remove <uuid> # 直接移除信任
```

## 数据流

```
mDNS发现 → 连接 → readLoop → onMessage
  → TrustStore.IsTrusted(msg.Sender)?
    → 是: dedup检查 → clipboard.Write()
    → 否: log.Printf("skipped untrusted device %s", ...)
```

## 安装

安装脚本新增步骤：创建配置目录、写入默认 `trusted.json`。
