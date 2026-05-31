# tools monorepo

## 项目结构

```
clipboard-sync/      -- Go 局域网剪贴板同步工具（主要项目）
  ├── main.go          入口，组装所有模块
  ├── config.go        配置常量
  ├── dedup/           SHA256 去重模块
  ├── sync/            协议编解码 + TCP 连接池管理
  ├── clipboard/       系统剪贴板读写/监听（CGo）
  ├── discovery/       mDNS 注册与发现
  ├── third_party/     golang.design/x/clipboard 内联
  ├── build.sh         跨平台构建脚本
  ├── install.sh       macOS LaunchAgent 安装脚本
  └── install.ps1      Windows Task Scheduler 安装脚本
```

## 开发命令

```bash
# 构建所有平台（默认）
./build.sh              # 输出到 dist/

# 仅构建当前平台（调试用）
go build -o dist/clipboardsync .
```

**产物说明**：
- `dist/ClipboardSync_macOS_<version>.zip` — macOS 安装包，解压后运行 `./install.sh install`
- `dist/ClipboardSync_Windows_<version>.zip` — Windows 安装包，解压后右键 `install.bat` 以管理员身份运行

**注意**：所有构建产物必须放到 `dist/` 目录下，不要留在项目根目录。

**注意**：`clipboard` 和 `discovery` 包没有自动化测试（需要系统剪贴板和真实 mDNS 网络）。

## 架构要点

- **拓扑**：对等网状网络，无中心节点。每台设备同时是 TCP 服务端和客户端。
- **同步规则**：每台设备只广播自己的剪贴板变化，收到远程消息后不转发，从根本上避免同步循环。
- **服务发现**：mDNS（dns-sd）+ UDP 组播（239.255.0.42:8921），服务类型 `_clipboardsync._tcp`，端口 8920。
- **传输协议**：按需 TCP 连接（Dial+Send+Close），4 字节大端长度前缀 + JSON 消息体。
- **组播可靠性**：发送端持久 UDP socket + 写失败自动重建；接收端监控 lastReceived + 15s 超时自动重建。
- **去重**：基于 SHA256 哈希，5 分钟 TTL 自动清理。

## 依赖管理

- `golang.design/x/clipboard` 内联在 `third_party/`，`go.mod` 中有 `replace` 指令。不要删除或更新该 replace 块，否则 macOS/Windows 编译会失败。
- Go 版本：1.22.3。不要降级。

## 代码约定

- `main.go` 中已定义 `itoa()` 辅助函数（strings 包替代品）。不要引入 `strconv`。
- 日志统一使用 `log.Ltime | log.Lshortfile` 格式，通过 `log.SetFlags` 在 `main()` 开头设置。
- `PeerManager` 是纯信息存储（map[UUID]→PeerInfo），不持有连接。剪贴板同步使用按需 TCP（Dial+Send+Close）。
- 新加包请参考现有 4 个子包的 `package` 声明风格（无额外注释）。

## 测试注意事项

- `sync` 包测试使用 `net.Pipe()` 模拟 TCP 连接（管道是无缓冲的）。
- `TestTTLExpiry`：`Dedup` 用 1ms TTL + 10ms sleep。如果测试环境慢导致 flaky，可适当增大 sleep 时间。
- 新增测试不要依赖真实网络或系统剪贴板。

## .claude/settings.local.json

已配置的权限规则允许运行 brainstorming 脚本、读取 Go 工具链路径、以及 `git commit`/`git add`。新增工具需要添加对应的权限条目。

## 后台运行 & 开机自启

```bash
# macOS
./install.sh install      # 安装并启动（需 sudo 复制二进制）
./install.sh uninstall    # 卸载
./install.sh status       # 查看状态

# Windows (PowerShell)
.\install.ps1 -Command install   # 安装（需管理员权限）
.\install.ps1 -Command uninstall # 卸载
.\install.ps1 -Command status    # 查看状态
```

日志位置：
- macOS: `~/Library/Logs/clipboardsync.log`
- Windows: 无日志文件，输出到任务历史记录

构建后运行 `./install.sh install` 或 `.\install.ps1 -Command install` 即可完成安装。

## macOS 构建后安装流程

每次构建完成后，执行以下步骤重新安装：

```bash
# 1. 构建
cd /Users/Thirteen/trae/tools/clipboard-sync && ./build.sh

# 2. 停掉旧进程
launchctl bootout gui/$(id -u)/com.clipboardsync 2>/dev/null
pkill -f clipboardsync 2>/dev/null

# 3. 安装（需要管理员权限，因为 /usr/local/bin 下的文件 owner 是 root）
osascript -e 'do shell script "cp /Users/Thirteen/trae/tools/clipboard-sync/dist/clipboardsync /usr/local/bin/clipboardsync" with administrator privileges'

# 4. 验证安装
clipboardsync --help
```
