# Clipboard Sync

一个局域网剪贴板同步工具，基于 Go 语言实现，采用对等网状网络拓扑，无中心节点。

## 功能特性

- **自动发现** - 基于 mDNS 和 UDP 组播，自动发现局域网内其他设备
- **实时同步** - 剪贴板内容变化后自动广播到所有在线设备
- **去重机制** - SHA256 哈希去重，5 分钟 TTL 自动清理，避免同步循环
- **对等网络** - 无中心节点，每台设备同时作为服务端和客户端
- **跨平台** - 支持 macOS 和 Windows
- **设备别名** - 为设备设置可读别名，便于识别
- **信任管理** - 白名单机制，可交互式管理受信任设备
- **后台运行** - 支持 `start`/`stop` 命令（PID 文件管理）和 LaunchAgent（macOS）开机自启

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
- **传输协议**：按需 TCP 连接（Dial→Send→Close），4 字节大端长度前缀 + JSON 消息体。**仅 TCP 用于传输剪贴板内容**
- **设备发现**：UDP 组播地址 `239.255.0.42:8921`，每 5 秒广播心跳（UUID + 主机名 + TCP 端口），15s 未收到即判定离线。**UDP 仅用于发现，不传输剪贴板内容**
- **Socket 可靠性**：发送端持久 UDP socket（写失败自动重建）；接收端 15s 超时或读错误时自动重建

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
- `dist/ClipboardSync_macOS_<version>.zip` — macOS 安装包
- `dist/ClipboardSync_Windows_<version>.zip` — Windows 安装包

### macOS 安装

```bash
# 解压安装包
unzip ClipboardSync_macOS_0.1.0.zip
cd ClipboardSync_macOS_0.1.0

# 安装（需要管理员权限）
./install.sh install

# 查看状态
./install.sh status

# 卸载
./install.sh uninstall
```

安装后，`clipboardsync` 命令将可用。

**手动运行：**
```bash
# 显示帮助（命令列表）
clipboardsync

# 前台运行（Ctrl+C 停止）
clipboardsync run

# 后台运行（PID 文件管理）
clipboardsync start

# 停止后台运行（精确终止指定 PID）
clipboardsync stop
```

**开机自启（LaunchAgent）：**
```bash
# 安装并启动后台服务
./install.sh install

# 停止服务
launchctl unload ~/Library/LaunchAgents/com.clipboardsync.plist

# 启动服务
launchctl load ~/Library/LaunchAgents/com.clipboardsync.plist
```

日志文件：
- 使用 `clipboardsync start`：`~/.config/clipboardsync/clipboardsync.log`
- 使用 LaunchAgent：`~/Library/Logs/clipboardsync.log`

### Windows 安装

```powershell
# 解压安装包
# 右键 install.bat 以管理员身份运行

# 卸载
# 右键 uninstall.bat 以管理员身份运行
```

**手动运行：**
```cmd
# 显示帮助（命令列表）
clipboardsync.exe

# 前台运行（Ctrl+C 停止）
clipboardsync.exe run

# 后台运行（关闭终端后继续运行）
clipboardsync.exe start

# 停止后台运行
clipboardsync.exe stop
```

**注意：** Windows 上使用 `start` 命令启动的后台进程会在终端关闭后继续运行。服务会自动在登录时启动（通过计划任务）。

## 设备别名

设备别名用于在设备发现时提高可读性。别名仅用于展示，最终映射以 UUID 为准。

```bash
# 查看当前别名
clipboardsync alias show

# 设置别名
clipboardsync alias set "我的MacBook"

# 交互式管理
clipboardsync alias
```

首次运行时，如果未设置别名，会自动使用 hostname 作为默认别名。

**别名显示：** 设备别名会在以下位置显示：
- `clipboardsync trust list` 命令输出
- `clipboardsync trust` 交互式 TUI 界面
- 日志文件中的设备发现信息

## 信任管理

通过命令行管理受信任设备：

```bash
# 交互式 TUI
clipboardsync trust

# 列出所有设备
clipboardsync trust list

# 添加信任
clipboardsync trust add <device-uuid>

# 移除信任
clipboardsync trust remove <device-uuid>
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
├── trust/              设备信任白名单管理 + 设备别名
├── third_party/        golang.design/x/clipboard 内联
├── build.sh            跨平台构建脚本
├── install.sh          macOS 安装/卸载脚本
└── install.ps1         Windows 安装/卸载脚本
```

## 配置

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| 服务端口 | 8920 | TCP 服务监听端口 |
| 组播地址 | 239.255.0.42:8921 | UDP 组播发现地址 |
| 服务类型 | `_clipboardsync._tcp` | mDNS 服务类型 |
| 去重 TTL | 5 分钟 | 哈希缓存自动清理时间 |

配置文件位置：
- macOS: `~/.config/clipboardsync/trusted.json`
- Windows: `%APPDATA%\clipboardsync\trusted.json`

## 常见问题

**Q: 启动后看不到其他设备？**

A: 确保其他设备也在运行本工具，且在同一局域网段。检查防火墙是否允许 8920（TCP）和 8921（UDP）端口。

**Q: 如何切换用户？**

A: 删除 `/tmp/clipboard-sync-uuid` 文件（macOS）或 `%TEMP%\clipboard-sync-uuid` 文件（Windows），重启后会自动生成新 UUID。

**Q: 旧进程占着端口？**

A: 运行 `stop` 命令停止后台进程：
```bash
clipboardsync stop
```

如仍无法停止，找到 PID 后手动终止：
```bash
ps aux | grep clipboardsync | grep -v grep
kill <PID>
```

**Q: 如何查看日志？**

A: 
- 使用 `clipboardsync start` 后台运行：`cat ~/.config/clipboardsync/clipboardsync.log`
- 使用 LaunchAgent 开机自启：`cat ~/Library/Logs/clipboardsync.log`

## 构建说明

- Go 版本：1.25.0
- `golang.design/x/clipboard` 内联在 `third_party/`，`go.mod` 中有 `replace` 指令
- `clipboard` 和 `discovery` 包没有自动化测试（需要系统剪贴板和真实 mDNS 网络）

## 许可证

MIT License
