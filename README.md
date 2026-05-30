# Clipboard Sync

一个局域网剪贴板同步工具，基于 Go 语言实现，采用对等网状网络拓扑，无中心节点。

## 功能特性

- **自动发现** - 基于 mDNS 和 UDP 组播，自动发现局域网内其他设备
- **实时同步** - 剪贴板内容变化后自动广播到所有在线设备
- **去重机制** - SHA256 哈希去重，5 分钟 TTL 自动清理，避免同步循环
- **对等网络** - 无中心节点，每台设备同时作为服务端和客户端
- **跨平台** - 支持 macOS（.app 应用包）和 Windows
- **信任管理** - 白名单机制，可交互式管理受信任设备
- **后台运行** - 支持 LaunchAgent（macOS）和 Task Scheduler（Windows）开机自启

## 架构概览

```
┌─────────────┐    mDNS / UDP 组播    ┌─────────────┐
│  Device A   │◄──────────────────────►│  Device B   │
│  TCP :8920  │                        │  TCP :8920  │
└──────┬──────┘                        └──────┬──────┘
       │                                      │
       ▼                                      ▼
  剪贴板监听                               剪贴板监听
```

- **拓扑**：对等网状网络，每台设备只广播自己的剪贴板变化，收到远程消息后不转发
- **服务发现**：mDNS（dns-sd），服务类型 `_clipboardsync._tcp`，端口 8920
- **传输协议**：按需 TCP 连接（Dial→Send→Close），4 字节大端长度前缀 + JSON 消息体
- **组播**：UDP 组播地址 `239.255.0.42:8921`，发送端持久 UDP socket，接收端 15s 超时自动重建

## 快速开始

### 从源码构建

```bash
git clone https://github.com/ThirteenR/clipboard-sync.git
cd clipboard-sync

# 构建所有平台
./build.sh

# 或仅构建当前平台（调试用）
go build -o dist/clipboardsync .
```

产物位于 `dist/` 目录：
- `dist/ClipboardSync.app` — macOS 应用包（通用二进制）
- `dist/ClipboardSync_Windows_<version>.zip` — Windows 安装包

### macOS 安装

```bash
# 构建
./build.sh

# 安装 .app 到 /Applications
osascript -e 'do shell script "rm -rf /Applications/ClipboardSync.app && cp -R dist/ClipboardSync.app /Applications/" with administrator privileges'

# 启动
open /Applications/ClipboardSync.app
```

### 后台运行 & 开机自启

```bash
# 安装 LaunchAgent
./install.sh install

# 查看状态
./install.sh status

# 卸载
./install.sh uninstall
```

日志文件：`~/Library/Logs/clipboardsync.log`

### Windows 安装

```powershell
# 以管理员身份运行
.\install.ps1 -Command install
```

## 信任管理

通过命令行管理受信任设备：

```bash
# 交互式 TUI
./clipboardsync trust

# 列出所有设备
./clipboardsync trust list

# 添加信任
./clipboardsync trust add <device-uuid>

# 移除信任
./clipboardsync trust remove <device-uuid>
```

`trust list` 命令会显示在线状态（● 表示在线），已信任设备标记为 `[x]`。

## 项目结构

```
clipboard-sync/
├── main.go             入口，组装所有模块
├── config.go           配置常量
├── clipboard/          系统剪贴板读写/监听（CGo）
├── sync/               协议编解码 + TCP 连接池管理
├── discovery/          mDNS 注册与发现
├── dedup/              SHA256 去重模块
├── trust/              设备信任白名单管理
├── third_party/        golang.design/x/clipboard 内联
├── build.sh            跨平台构建脚本
├── install.sh          macOS LaunchAgent 安装脚本
└── install.ps1         Windows Task Scheduler 安装脚本
```

## 配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| 服务端口 | 8920 | TCP 服务监听端口 |
| 组播地址 | 239.255.0.42:8921 | UDP 组播发现地址 |
| 服务类型 | `_clipboardsync._tcp` | mDNS 服务类型 |
| 去重 TTL | 5 分钟 | 哈希缓存自动清理时间 |

## 常见问题

**Q: 启动后看不到其他设备？**

A: 确保其他设备也在运行本工具，且在同一局域网段。检查防火墙是否允许 8920（TCP）和 8921（UDP）端口。

**Q: 如何切换用户？**

A: 删除 `~/.clipboardsync/` 目录下的 `uuid` 文件，重启后会自动生成新 UUID。

**Q: 旧进程占着端口？**

A: 运行 `launchctl bootout gui/$(id -u)/com.clipboardsync` 停掉旧进程再启动。

## 构建说明

- Go 版本：1.22.3
- `golang.design/x/clipboard` 内联在 `third_party/`，`go.mod` 中有 `replace` 指令
- `clipboard` 和 `discovery` 包没有自动化测试（需要系统剪贴板和真实 mDNS 网络）

## 许可证

MIT License
