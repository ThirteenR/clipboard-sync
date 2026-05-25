# 受信设备白名单系统 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 添加受信设备白名单机制，仅受信设备可同步剪贴板内容，通过 TUI 管理白名单

**Architecture:** 新增 `trust/` 包（TrustStore + TUI shell），修改 `main.go` 添加信任检查及 CLI 子命令，安装脚本自动创建配置

**Tech Stack:** Go, bubbletea (TUI), zeroconf (mDNS, 已有依赖)

---

### Task 1: 添加 bubbletea 依赖

**Files:**
- Modify: `go.mod`

- [ ] **添加依赖并整理**

Run: `go get github.com/charmbracelet/bubbletea@v1.3.4 && go mod tidy`

Expected: `go.mod` 和 `go.sum` 更新

- [ ] **验证编译**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **提交**

```bash
git add go.mod go.sum
git commit -m "chore: add bubbletea TUI dependency"
```

---

### Task 2: 创建 trust/store.go — TrustStore 核心

**Files:**
- Create: `trust/store.go`
- Create: `trust/store_test.go`

- [ ] **实现 TrustStore**

```go
package trust

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DeviceInfo struct {
	Hostname string `json:"hostname"`
	LastSeen string `json:"last_seen"`
}

type storeData struct {
	TrustedUUIDs []string            `json:"trusted_uuids"`
	Devices      map[string]DeviceInfo `json:"devices"`
}

type TrustStore struct {
	mu       sync.RWMutex
	data     storeData
	filePath string
	modTime  time.Time
}

func configDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "clipboardsync")
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		return filepath.Join(home, ".config", "clipboardsync")
	}
	return filepath.Join(os.Getenv("APPDATA"), "clipboardsync")
}

func New() (*TrustStore, error) {
	cfgDir := configDir()
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		return nil, err
	}
	filePath := filepath.Join(cfgDir, "trusted.json")
	ts := &TrustStore{
		filePath: filePath,
		data: storeData{
			TrustedUUIDs: []string{},
			Devices:      make(map[string]DeviceInfo),
		},
	}
	if err := ts.load(); err != nil {
		if os.IsNotExist(err) {
			if e := ts.save(); e != nil {
				return nil, e
			}
			return ts, nil
		}
		return nil, err
	}
	return ts, nil
}

func (ts *TrustStore) load() error {
	data, err := os.ReadFile(ts.filePath)
	if err != nil {
		return err
	}
	info, err := os.Stat(ts.filePath)
	if err == nil {
		ts.modTime = info.ModTime()
	}
	return json.Unmarshal(data, &ts.data)
}

func (ts *TrustStore) save() error {
	data, err := json.MarshalIndent(ts.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ts.filePath, data, 0644)
}

func (ts *TrustStore) IsTrusted(uuid string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	for _, id := range ts.data.TrustedUUIDs {
		if id == uuid {
			return true
		}
	}
	return false
}

func (ts *TrustStore) Add(uuid, hostname string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, id := range ts.data.TrustedUUIDs {
		if id == uuid {
			return
		}
	}
	ts.data.TrustedUUIDs = append(ts.data.TrustedUUIDs, uuid)
	ts.data.Devices[uuid] = DeviceInfo{
		Hostname: hostname,
		LastSeen: time.Now().UTC().Format(time.RFC3339),
	}
	if err := ts.save(); err != nil {
		log.Printf("Failed to save trust store: %v", err)
	}
}

func (ts *TrustStore) Remove(uuid string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	filtered := make([]string, 0, len(ts.data.TrustedUUIDs))
	for _, id := range ts.data.TrustedUUIDs {
		if id != uuid {
			filtered = append(filtered, id)
		}
	}
	ts.data.TrustedUUIDs = filtered
	if err := ts.save(); err != nil {
		log.Printf("Failed to save trust store: %v", err)
	}
}

type DeviceInfo struct {
	Hostname string `json:"hostname"`
	LastSeen string `json:"last_seen"`
	Trusted  bool   `json:"-"`
}

func (ts *TrustStore) List() []DeviceEntry {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	entries := make([]DeviceEntry, 0, len(ts.data.Devices))
	for uuid, dev := range ts.data.Devices {
		trusted := false
		for _, id := range ts.data.TrustedUUIDs {
			if id == uuid {
				trusted = true
				break
			}
		}
		entries = append(entries, DeviceEntry{
			UUID:     uuid,
			Hostname: dev.Hostname,
			LastSeen: dev.LastSeen,
			Trusted:  trusted,
		})
	}
	return entries
}

func (ts *TrustStore) ReloadIfChanged() error {
	info, err := os.Stat(ts.filePath)
	if err != nil {
		return err
	}
	if info.ModTime().After(ts.modTime) {
		return ts.load()
	}
	return nil
}
```

- [ ] **编写测试 `trust/store_test.go`**

```go
package trust

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewCreatesDefault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if ts == nil {
		t.Fatal("New() returned nil")
	}
}

func TestAddAndIsTrusted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, _ := New()
	ts.Add("uuid-1", "host-1")
	if !ts.IsTrusted("uuid-1") {
		t.Error("expected uuid-1 to be trusted after Add")
	}
	if ts.IsTrusted("uuid-2") {
		t.Error("expected uuid-2 not to be trusted")
	}
}

func TestAddDuplicate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, _ := New()
	ts.Add("uuid-1", "host-1")
	ts.Add("uuid-1", "host-2")
	entries := ts.List()
	count := 0
	for _, e := range entries {
		if e.UUID == "uuid-1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 entry for uuid-1, got %d", count)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, _ := New()
	ts.Add("uuid-1", "host-1")
	ts.Remove("uuid-1")
	if ts.IsTrusted("uuid-1") {
		t.Error("expected uuid-1 not to be trusted after Remove")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts1, _ := New()
	ts1.Add("uuid-1", "host-1")
	ts1.Add("uuid-2", "host-2")

	ts2, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if !ts2.IsTrusted("uuid-1") {
		t.Error("expected uuid-1 to persist")
	}
	if !ts2.IsTrusted("uuid-2") {
		t.Error("expected uuid-2 to persist")
	}
}

func TestIsTrustedNotPersisted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, _ := New()
	if ts.IsTrusted("nonexistent") {
		t.Error("expected false for nonexistent UUID")
	}
}

func TestConfigDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got := configDir()
	want := filepath.Join(dir, "clipboardsync")
	if got != want {
		t.Errorf("configDir() = %s, want %s", got, want)
	}
}
```

- [ ] **运行测试**

Run: `go test ./trust/ -v`
Expected: 全部 PASS

- [ ] **提交**

```bash
git add trust/
git commit -m "feat: add TrustStore with persistence"
```

---

### Task 3: 创建 trust/tui.go — TUI 管理界面

**Files:**
- Create: `trust/tui.go`

- [ ] **实现 TUI**

```go
package trust

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"clipboard-sync/discovery"

	tea "github.com/charmbracelet/bubbletea"
)

type entry struct {
	uuid     string
	hostname string
	trusted  bool
}

type model struct {
	entries  []entry
	cursor   int
	loading  bool
	err      error
	store    *TrustStore
	quitting bool
}

func initialModel(store *TrustStore) model {
	return model{
		entries:  nil,
		cursor:   0,
		loading:  true,
		store:    store,
		quitting: false,
	}
}

type discoveryDoneMsg struct {
	entries []entry
	err     error
}

func (m model) Init() tea.Cmd {
	return m.discover
}

func (m model) discover() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	seen := make(map[string]bool)
	var entries []entry

	handler := discovery.Handler{
		OnJoin: func(info discovery.PeerInfo) {
			if info.UUID == "" || seen[info.UUID] {
				return
			}
			seen[info.UUID] = true
			trusted := m.store.IsTrusted(info.UUID)
			entries = append(entries, entry{
				uuid:     info.UUID,
				hostname: info.Hostname,
				trusted:  trusted,
			})
		},
		OnLeave: nil,
	}

	if err := discovery.Discover(ctx, handler); err != nil {
		return discoveryDoneMsg{err: err}
	}

	// Also include stored devices not discovered
	for _, de := range m.store.List() {
		if !seen[de.UUID] {
			entries = append(entries, entry{
				uuid:     de.UUID,
				hostname: de.Hostname,
				trusted:  de.Trusted,
			})
		}
	}

	return discoveryDoneMsg{entries: entries}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "s":
			m.save()
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case " ":
			if m.cursor >= 0 && m.cursor < len(m.entries) {
				m.entries[m.cursor].trusted = !m.entries[m.cursor].trusted
			}
		}
	case discoveryDoneMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.entries = msg.entries
		}
	}
	return m, nil
}

func (m model) save() {
	for _, e := range m.entries {
		if e.trusted {
			m.store.Add(e.uuid, e.hostname)
		} else {
			m.store.Remove(e.uuid)
		}
	}
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.loading {
		return "🔍  Searching for devices on LAN... (3s)\n\nPress q to quit."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}
	if len(m.entries) == 0 {
		return "No devices found on LAN.\n\nPress q to quit."
	}

	var b strings.Builder
	b.WriteString("Clipboard Sync - Trusted Devices\n\n")
	for i, e := range m.entries {
		cursor := " "
		if i == m.cursor {
			cursor = ">"
		}
		check := " "
		if e.trusted {
			check = "x"
		}
		b.WriteString(fmt.Sprintf("%s [%s] %s  (%s)\n", cursor, check, e.hostname, e.uuid))
	}
	b.WriteString("\n↑/↓ navigate • space toggle • s save • q quit")
	return b.String()
}

func RunTUI(store *TrustStore) {
	p := tea.NewProgram(initialModel(store))
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI failed: %v", err)
	}
}

func RunList(store *TrustStore) {
	for _, e := range store.List() {
		trusted := " "
		if e.Trusted {
			trusted = "*"
		}
		fmt.Printf("[%s] %s  (%s)  last seen: %s\n", trusted, e.Hostname, e.UUID, e.LastSeen)
	}
}
```

- [ ] **验证编译**

Run: `go build ./trust/`
Expected: 编译成功

- [ ] **提交**

```bash
git add trust/tui.go
git commit -m "feat: add TUI management interface for trusted devices"
```

---

### Task 4: 修改 main.go — 信任检查 + CLI 子命令

**Files:**
- Modify: `main.go`

- [ ] **添加 trust CLI 子命令和 onMessage 信任检查**

修改 `main.go`：

```go
package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"clipboard-sync/clipboard"
	"clipboard-sync/dedup"
	"clipboard-sync/discovery"
	"clipboard-sync/sync"
	"clipboard-sync/trust"

	"github.com/google/uuid"
)

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// CLI subcommand: trust management
	if len(os.Args) > 1 && os.Args[1] == "trust" {
		store, err := trust.New()
		if err != nil {
			log.Fatalf("Failed to load trust store: %v", err)
		}
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "list":
				trust.RunList(store)
			case "add":
				if len(os.Args) > 3 {
					store.Add(os.Args[3], os.Args[3])
					log.Printf("Added %s to trusted devices", os.Args[3])
				} else {
					log.Fatal("Usage: clipboardsync trust add <uuid>")
				}
			case "remove":
				if len(os.Args) > 3 {
					store.Remove(os.Args[3])
					log.Printf("Removed %s from trusted devices", os.Args[3])
				} else {
					log.Fatal("Usage: clipboardsync trust remove <uuid>")
				}
			default:
				log.Fatal("Usage: clipboardsync trust [list|add|remove]")
			}
		} else {
			trust.RunTUI(store)
		}
		return
	}

	log.Println("Clipboard Sync starting...")

	deviceUUID := loadOrCreateUUID()
	log.Printf("Device UUID: %s", deviceUUID)

	trustStore, err := trust.New()
	if err != nil {
		log.Fatalf("Failed to load trust store: %v", err)
	}
	log.Printf("Trust store loaded, %d trusted device(s)", countTrusted(trustStore))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Periodically reload trust config
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := trustStore.ReloadIfChanged(); err != nil && !os.IsNotExist(err) {
					log.Printf("Trust store reload error: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	deduper := dedup.New(DedupTTL)
	defer deduper.Stop()

	pm := sync.NewPeerManager(deviceUUID, func(msg sync.Message) {
		if !trustStore.IsTrusted(msg.Sender) {
			log.Printf("Skipped clipboard from untrusted device %s", truncate(msg.Sender, 16))
			return
		}
		if deduper.Seen(msg.Hash) {
			return
		}
		deduper.Mark(msg.Hash)
		log.Printf("Received clipboard from %s: %s", msg.Sender, truncate(msg.Content, 50))
		if err := clipboard.Write(msg.Content); err != nil {
			log.Printf("Failed to write clipboard: %v", err)
		}
	}, 0)

	// ... rest of main.go remains unchanged ...
}

func countTrusted(ts *trust.TrustStore) int {
	count := 0
	for _, e := range ts.List() {
		if e.Trusted {
			count++
		}
	}
	return count
}
```

- [ ] **验证编译**

Run: `go build .`
Expected: 编译成功

- [ ] **提交**

```bash
git add main.go
git commit -m "feat: add trust check to clipboard sync and CLI subcommand"
```

---

### Task 5: 更新安装脚本 — 安装时自动创建配置

**Files:**
- Modify: `install.sh`
- Modify: `install.ps1`

- [ ] **修改 install.sh：安装时创建 `trusted.json`**

在 `cmd_install()` 函数中找到一个合适位置，在复制二进制后、创建 plist 前添加：

```bash
echo "  Creating config..."
CONFIG_DIR="$HOME/.config/clipboardsync"
mkdir -p "$CONFIG_DIR"
if [ ! -f "$CONFIG_DIR/trusted.json" ]; then
  echo '{"trusted_uuids":[],"devices":{}}' > "$CONFIG_DIR/trusted.json"
fi
echo "  Run 'clipboardsync trust' to configure trusted devices."
```

- [ ] **修改 install.ps1：安装时创建 `trusted.json`**

在 `Install-Service()` 的复制二进制步骤后添加：

```powershell
Write-Host "  Creating config..."
$configDir = "$env:APPDATA\clipboardsync"
$null = New-Item -ItemType Directory -Force -Path $configDir
$configFile = "$configDir\trusted.json"
if (-not (Test-Path $configFile)) {
    Set-Content $configFile '{"trusted_uuids":[],"devices":{}}'
}
Write-Host "  Run 'clipboardsync trust' to configure trusted devices."
```

- [ ] **验证脚本语法**

Run: `bash -n install.sh`
Expected: 无语法错误

- [ ] **提交**

```bash
git add install.sh install.ps1
git commit -m "feat: auto-create trust config on install"
```

---

### Task 6: 最终验证

- [ ] **完整编译验证**

Run: `go build ./...`
Expected: 编译成功，无错误

- [ ] **运行所有测试**

Run: `go test ./...`
Expected: 全部 PASS

- [ ] **最终提交**

```bash
git add -A
git commit -m "feat: add trusted device whitelist with TUI management"
```
