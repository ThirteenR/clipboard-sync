# 设备别名功能实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为clipboard-sync应用添加设备别名功能，允许用户为设备设置可读的别名，提高设备发现时的可读性。

**Architecture:** 扩展trust包管理别名，在heartbeat中广播别名，通过TUI和命令行参数提供用户交互。

**Tech Stack:** Go, JSON, UDP multicast, TUI (stdin/stdout)

---

## 文件结构

### 创建的文件
- `trust/alias.go` - 别名验证和管理逻辑
- `trust/alias_test.go` - 别名功能单元测试

### 修改的文件
- `trust/store.go` - 扩展数据结构，添加别名字段
- `trust/tui.go` - 添加别名管理TUI界面
- `discovery/multicast.go` - heartbeat消息添加别名字段
- `main.go` - 添加alias子命令和启动时别名检查

---

## Task 1: 扩展trust/store.go数据结构

**Files:**
- Modify: `trust/store.go:11-20`

- [ ] **Step 1: 修改storeData结构体**

```go
type storeData struct {
	TrustedUUIDs []string            `json:"trusted_uuids"`
	Devices      map[string]DeviceInfo `json:"devices"`
	DeviceAlias  string              `json:"device_alias,omitempty"`
	DeviceAliases map[string]string   `json:"device_aliases,omitempty"`
}
```

- [ ] **Step 2: 添加别名相关的错误类型**

在store.go文件开头添加：

```go
import (
	"errors"
	"unicode"
)

var (
	ErrAliasTooLong     = errors.New("别名长度不能超过20字符")
	ErrAliasInvalidChar = errors.New("别名包含无效字符")
	ErrAliasEmpty       = errors.New("别名不能为空")
)
```

- [ ] **Step 3: 添加别名验证函数**

```go
func validateAlias(alias string) error {
	if alias == "" {
		return ErrAliasEmpty
	}
	if len([]rune(alias)) > 20 {
		return ErrAliasTooLong
	}
	for _, r := range alias {
		if r == '\n' || r == '\t' || r == '\0' {
			return ErrAliasInvalidChar
		}
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != ' ' && r != '-' && r != '_' && r != '.' {
			return ErrAliasInvalidChar
		}
	}
	return nil
}
```

- [ ] **Step 4: 添加别名管理方法**

```go
func (ts *TrustStore) GetDeviceAlias() string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.data.DeviceAlias
}

func (ts *TrustStore) SetDeviceAlias(alias string) error {
	if err := validateAlias(alias); err != nil {
		return err
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.data.DeviceAlias = alias
	return ts.save()
}

func (ts *TrustStore) HasDeviceAlias() bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.data.DeviceAlias != ""
}

func (ts *TrustStore) GetPeerAlias(uuid string) string {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	if ts.data.DeviceAliases == nil {
		return ""
	}
	return ts.data.DeviceAliases[uuid]
}

func (ts *TrustStore) SetPeerAlias(uuid, alias string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.data.DeviceAliases == nil {
		ts.data.DeviceAliases = make(map[string]string)
	}
	ts.data.DeviceAliases[uuid] = alias
}
```

- [ ] **Step 5: 添加显示格式函数**

```go
func FormatDisplayName(alias, hostname, uuid string) string {
	if alias != "" && hostname != "" {
		return alias + " (" + hostname + ")"
	}
	if alias != "" {
		return alias + " (" + uuid[:8] + ")"
	}
	if hostname != "" {
		return hostname + " (" + uuid[:8] + ")"
	}
	return uuid
}
```

- [ ] **Step 6: 提交更改**

```bash
git add trust/store.go
git commit -m "feat: extend trust store with device alias support"
```

---

## Task 2: 创建trust/alias.go别名管理模块

**Files:**
- Create: `trust/alias.go`

- [ ] **Step 1: 创建别名管理文件**

```go
package trust

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func (ts *TrustStore) PromptSetDeviceAlias(deviceUUID string) {
	reader := bufio.NewReader(os.Stdin)
	hostname, _ := os.Hostname()
	defaultAlias := hostname

	currentAlias := ts.GetDeviceAlias()
	if currentAlias != "" {
		fmt.Printf("当前设备别名: %s\n", currentAlias)
		fmt.Printf("UUID: %s\n", deviceUUID)
		fmt.Print("是否修改别名？(y/n): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			return
		}
	}

	fmt.Printf("请输入设备别名 (最大20字符，当前默认: %s): ", defaultAlias)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		input = defaultAlias
	}

	if err := ts.SetDeviceAlias(input); err != nil {
		fmt.Printf("设置别名失败: %v\n", err)
		return
	}

	fmt.Printf("设备别名已设置为: %s\n", input)
}

func (ts *TrustStore) RunAliasTUI(deviceUUID string) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n=== Clipboard Sync 设备别名管理 ===")
		alias := ts.GetDeviceAlias()
		if alias == "" {
			alias = "(未设置)"
		}
		fmt.Printf("当前设备别名: %s\n", alias)
		fmt.Printf("UUID: %s\n", deviceUUID)

		fmt.Println("\n操作:")
		fmt.Println("  [1] 修改别名")
		fmt.Println("  [2] 查看其他设备别名")
		fmt.Println("  [q] 退出")
		fmt.Print("请选择: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "1":
			ts.PromptSetDeviceAlias(deviceUUID)
		case "2":
			ts.showPeerAliases()
		case "q":
			return
		default:
			fmt.Println("无效选择，请重新输入")
		}
	}
}

func (ts *TrustStore) showPeerAliases() {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	fmt.Println("\n=== 其他设备别名 ===")
	if ts.data.DeviceAliases == nil || len(ts.data.DeviceAliases) == 0 {
		fmt.Println("暂无其他设备别名记录")
		return
	}

	for uuid, alias := range ts.data.DeviceAliases {
		hostname := ""
		if dev, ok := ts.data.Devices[uuid]; ok {
			hostname = dev.Hostname
		}
		displayName := FormatDisplayName(alias, hostname, uuid)
		fmt.Printf("  %s\n", displayName)
	}
}

func (ts *TrustStore) SetAliasCommand(alias string, deviceUUID string) {
	if err := ts.SetDeviceAlias(alias); err != nil {
		fmt.Printf("设置别名失败: %v\n", err)
		return
	}
	fmt.Printf("设备别名已设置为: %s\n", alias)
}

func (ts *TrustStore) ShowAliasCommand() {
	alias := ts.GetDeviceAlias()
	if alias == "" {
		fmt.Println("设备别名未设置")
		return
	}
	fmt.Printf("设备别名: %s\n", alias)
}
```

- [ ] **Step 2: 提交更改**

```bash
git add trust/alias.go
git commit -m "feat: add alias management TUI and commands"
```

---

## Task 3: 创建trust/alias_test.go单元测试

**Files:**
- Create: `trust/alias_test.go`

- [ ] **Step 1: 创建别名验证测试**

```go
package trust

import (
	"testing"
)

func TestValidateAlias(t *testing.T) {
	tests := []struct {
		name    string
		alias   string
		wantErr error
	}{
		{"valid simple", "my-laptop", nil},
		{"valid chinese", "我的电脑", nil},
		{"valid with spaces", "my laptop", nil},
		{"valid with dots", "my.laptop", nil},
		{"valid with underscores", "my_laptop", nil},
		{"empty", "", ErrAliasEmpty},
		{"too long", "this-is-a-very-long-alias-name-that-exceeds-twenty-chars", ErrAliasTooLong},
		{"invalid char @", "my@laptop", ErrAliasInvalidChar},
		{"invalid char #", "my#laptop", ErrAliasInvalidChar},
		{"invalid char newline", "my\nlaptop", ErrAliasInvalidChar},
		{"invalid char tab", "my\tlaptop", ErrAliasInvalidChar},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAlias(tt.alias)
			if err != tt.wantErr {
				t.Errorf("validateAlias(%q) = %v, want %v", tt.alias, err, tt.wantErr)
			}
		})
	}
}
```

- [ ] **Step 2: 创建别名存储测试**

```go
func TestDeviceAliasStorage(t *testing.T) {
	ts, err := New()
	if err != nil {
		t.Fatalf("Failed to create trust store: %v", err)
	}

	// Test initial state
	if ts.HasDeviceAlias() {
		t.Error("Expected no device alias initially")
	}

	// Test set alias
	if err := ts.SetDeviceAlias("test-device"); err != nil {
		t.Fatalf("Failed to set device alias: %v", err)
	}

	// Test get alias
	alias := ts.GetDeviceAlias()
	if alias != "test-device" {
		t.Errorf("GetDeviceAlias() = %q, want %q", alias, "test-device")
	}

	// Test has alias
	if !ts.HasDeviceAlias() {
		t.Error("Expected HasDeviceAlias() to return true")
	}

	// Test update alias
	if err := ts.SetDeviceAlias("updated-device"); err != nil {
		t.Fatalf("Failed to update device alias: %v", err)
	}

	alias = ts.GetDeviceAlias()
	if alias != "updated-device" {
		t.Errorf("GetDeviceAlias() after update = %q, want %q", alias, "updated-device")
	}
}

func TestPeerAliasStorage(t *testing.T) {
	ts, err := New()
	if err != nil {
		t.Fatalf("Failed to create trust store: %v", err)
	}

	// Test initial state
	alias := ts.GetPeerAlias("test-uuid")
	if alias != "" {
		t.Errorf("GetPeerAlias() initially = %q, want empty", alias)
	}

	// Test set peer alias
	ts.SetPeerAlias("test-uuid", "peer-device")

	alias = ts.GetPeerAlias("test-uuid")
	if alias != "peer-device" {
		t.Errorf("GetPeerAlias() = %q, want %q", alias, "peer-device")
	}

	// Test multiple peers
	ts.SetPeerAlias("uuid-2", "second-peer")

	alias1 := ts.GetPeerAlias("test-uuid")
	alias2 := ts.GetPeerAlias("uuid-2")
	if alias1 != "peer-device" || alias2 != "second-peer" {
		t.Errorf("Peer aliases mismatch: got %q and %q", alias1, alias2)
	}
}
```

- [ ] **Step 3: 创建显示格式测试**

```go
func TestFormatDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		alias    string
		hostname string
		uuid     string
		want     string
	}{
		{"all present", "我的电脑", "MacBook", "12345678-1234-1234-1234-123456789012", "我的电脑 (MacBook)"},
		{"no alias", "", "MacBook", "12345678-1234-1234-1234-123456789012", "MacBook (12345678)"},
		{"no hostname", "我的电脑", "", "12345678-1234-1234-1234-123456789012", "我的电脑 (12345678)"},
		{"only uuid", "", "", "12345678-1234-1234-1234-123456789012", "12345678-1234-1234-1234-123456789012"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDisplayName(tt.alias, tt.hostname, tt.uuid)
			if got != tt.want {
				t.Errorf("FormatDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 4: 运行测试**

```bash
cd /Users/Thirteen/trae/tools/clipboard-sync && go test ./trust/ -v -run "TestValidateAlias|TestDeviceAlias|TestPeerAlias|TestFormatDisplayName"
```

- [ ] **Step 5: 提交更改**

```bash
git add trust/alias_test.go
git commit -m "test: add unit tests for alias management"
```

---

## Task 4: 修改discovery/multicast.go集成别名

**Files:**
- Modify: `discovery/multicast.go:18-22`

- [ ] **Step 1: 修改heartbeatMsg结构体**

```go
type heartbeatMsg struct {
	UUID     string `json:"uuid"`
	Hostname string `json:"hostname"`
	Port     int    `json:"port"`
	Alias    string `json:"alias,omitempty"`
}
```

- [ ] **Step 2: 修改multicastRegister函数签名**

修改函数签名，添加trustStore参数：

```go
func multicastRegister(ctx context.Context, instance, uuid string, port int, trustStore interface{ GetDeviceAlias() string }) (*RegisterHandle, error) {
```

- [ ] **Step 3: 修改heartbeat消息构建**

在multicastRegister函数中，修改消息构建部分：

```go
msg := heartbeatMsg{
	UUID:     uuid,
	Hostname: instance,
	Port:     port,
	Alias:    trustStore.GetDeviceAlias(),
}
```

- [ ] **Step 4: 修改multicastDiscover函数处理接收消息**

在multicastDiscover函数中，添加别名处理：

```go
if _, exists := peers[msg.UUID]; !exists {
	log.Printf("multicast discovered: %s (%s)", hostname, msg.UUID)
	peers[msg.UUID] = peerRecord{info: info, lastSeen: time.Now()}
	handler.OnJoin(info)
} else {
	peers[msg.UUID] = peerRecord{info: info, lastSeen: time.Now()}
}

// 处理别名
if msg.Alias != "" && handler.OnAliasUpdate != nil {
	handler.OnAliasUpdate(msg.UUID, msg.Alias)
}
```

- [ ] **Step 5: 修改Handler结构体**

```go
type Handler struct {
	OnJoin       func(PeerInfo)
	OnLeave      func(PeerInfo)
	OnAliasUpdate func(uuid, alias string)
}
```

- [ ] **Step 6: 提交更改**

```bash
git add discovery/multicast.go
git commit -m "feat: integrate alias into heartbeat messages"
```

---

## Task 5: 修改discovery/discovery.go适配接口

**Files:**
- Modify: `discovery/discovery.go:31-37`

- [ ] **Step 1: 修改Register函数签名**

```go
func Register(ctx context.Context, instance, uuid, host string, port int, trustStore interface{ GetDeviceAlias() string }) (*RegisterHandle, error) {
	log.Printf("Registering service: %s (%s) on %s:%d", instance, uuid, host, port)
	if runtime.GOOS == "darwin" {
		registerDarwin(ctx, instance, uuid, port)
	}
	return multicastRegister(ctx, instance, uuid, port, trustStore)
}
```

- [ ] **Step 2: 提交更改**

```bash
git add discovery/discovery.go
git commit -m "feat: add trustStore parameter to Register function"
```

---

## Task 6: 修改main.go集成别名功能

**Files:**
- Modify: `main.go:24-67,127-148,168-177`

- [ ] **Step 1: 添加alias子命令处理**

在main函数开头添加alias子命令处理：

```go
if len(os.Args) > 1 && os.Args[1] == "alias" {
	store, err := trust.New()
	if err != nil {
		log.Fatalf("Failed to load trust store: %v", err)
	}
	deviceUUID := loadOrCreateUUID()

	if len(os.Args) > 2 {
		switch os.Args[2] {
		case "set":
			if len(os.Args) < 4 {
				log.Fatal("Usage: clipboardsync alias set <alias>")
			}
			store.SetAliasCommand(os.Args[3], deviceUUID)
		case "show":
			store.ShowAliasCommand()
		default:
			log.Fatalf("Unknown alias subcommand: %s", os.Args[2])
		}
	} else {
		store.RunAliasTUI(deviceUUID)
	}
	return
}
```

- [ ] **Step 2: 添加启动时别名检查**

在main函数中，加载trust store后添加别名检查：

```go
trustStore, err := trust.New()
if err != nil {
	log.Fatalf("Failed to load trust store: %v", err)
}
log.Printf("Trust store loaded")

// 检查设备别名
if !trustStore.HasDeviceAlias() {
	log.Println("设备别名未设置，提示用户设置...")
	trustStore.PromptSetDeviceAlias(deviceUUID)
}
```

- [ ] **Step 3: 修改discovery注册调用**

修改discovery.Register调用，传递trustStore：

```go
go func() {
	hostname, _ := os.Hostname()
	server, err := discovery.Register(ctx, hostname, deviceUUID, hostname, ServicePort, trustStore)
	if err != nil {
		log.Fatalf("Failed to register mDNS service: %v", err)
	}
	defer server.Shutdown()
	log.Printf("mDNS service registered as %s", hostname)
	<-ctx.Done()
}()
```

- [ ] **Step 4: 修改discovery handler添加别名处理**

```go
handler := discovery.Handler{
	OnJoin: func(info discovery.PeerInfo) {
		if info.UUID == "" || info.UUID == deviceUUID || info.Addr == "" {
			return
		}
		pm.Add(sync.PeerInfo{
			UUID:     info.UUID,
			Hostname: info.Hostname,
			Addr:     info.Addr,
			Port:     info.Port,
		})
	},
	OnLeave: func(info discovery.PeerInfo) {
		log.Printf("Peer left: %s (%s)", info.Hostname, info.UUID)
		pm.Remove(info.UUID)
	},
	OnAliasUpdate: func(uuid, alias string) {
		trustStore.SetPeerAlias(uuid, alias)
		log.Printf("设备 %s 别名更新: %s", uuid, alias)
	},
}
```

- [ ] **Step 5: 提交更改**

```bash
git add main.go
git commit -m "feat: integrate alias commands and startup check"
```

---

## Task 7: 修改trust/tui.go添加别名管理入口

**Files:**
- Modify: `trust/tui.go`

- [ ] **Step 1: 查看现有tui.go结构**

```bash
cat /Users/Thirteen/trae/tools/clipboard-sync/trust/tui.go
```

- [ ] **Step 2: 在TUI中添加别名管理选项**

在RunTUI函数中添加别名管理选项（如果存在主菜单）：

```go
case "alias":
	store.RunAliasTUI(deviceUUID)
```

- [ ] **Step 3: 提交更改**

```bash
git add trust/tui.go
git commit -m "feat: add alias management to trust TUI"
```

---

## Task 8: 集成测试和验证

**Files:**
- Test: 整体功能测试

- [ ] **Step 1: 构建项目**

```bash
cd /Users/Thirteen/trae/tools/clipboard-sync && go build -o dist/clipboardsync .
```

- [ ] **Step 2: 测试alias子命令**

```bash
./dist/clipboardsync alias show
./dist/clipboardsync alias set "测试设备"
./dist/clipboardsync alias show
```

- [ ] **Step 3: 运行单元测试**

```bash
go test ./trust/ -v
```

- [ ] **Step 4: 测试首次启动流程**

```bash
# 备份现有配置
mv ~/.config/clipboardsync/trusted.json ~/.config/clipboardsync/trusted.json.bak

# 运行程序，测试别名设置提示
./dist/clipboardsync

# 恢复配置
mv ~/.config/clipboardsync/trusted.json.bak ~/.config/clipboardsync/trusted.json
```

- [ ] **Step 5: 提交最终更改**

```bash
git add .
git commit -m "feat: complete device alias feature implementation"
```

---

## 完成检查清单

- [ ] 别名验证函数正确实现
- [ ] 别名存储和读取功能正常
- [ ] 别名在heartbeat中正确传输
- [ ] 别名在设备发现时正确显示
- [ ] TUI界面正常工作
- [ ] 命令行参数正常工作
- [ ] 首次启动提示正常
- [ ] 单元测试全部通过
- [ ] 不影响现有功能
- [ ] 代码已提交