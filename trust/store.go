package trust

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
	"unicode"
)

var (
	ErrAliasTooLong     = errors.New("别名长度不能超过20字符")
	ErrAliasInvalidChar = errors.New("别名包含无效字符")
	ErrAliasEmpty       = errors.New("别名不能为空")
)

type DeviceInfo struct {
	Hostname string `json:"hostname"`
	LastSeen string `json:"last_seen"`
}

type storeData struct {
	TrustedUUIDs  []string            `json:"trusted_uuids"`
	Devices       map[string]DeviceInfo `json:"devices"`
	DeviceAlias   string              `json:"device_alias,omitempty"`
	DeviceAliases map[string]string   `json:"device_aliases,omitempty"`
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
	// Try HOME environment variable first (more reliable in LaunchAgent context)
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "clipboardsync")
	}
	// Fallback to os.UserHomeDir()
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return filepath.Join(home, ".config", "clipboardsync")
	}
	// Last resort: use APPDATA on Windows
	if appdata := os.Getenv("APPDATA"); appdata != "" {
		return filepath.Join(appdata, "clipboardsync")
	}
	// Should never happen, but return absolute path as last resort
	if runtime.GOOS == "windows" {
		return filepath.Join(os.TempDir(), "clipboardsync")
	}
	return "/tmp/clipboardsync"
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
	if fi, err := os.Stat(ts.filePath); err == nil {
		ts.modTime = fi.ModTime()
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

type DeviceEntry struct {
	UUID     string
	Hostname string
	LastSeen string
	Trusted  bool
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
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if info.ModTime().After(ts.modTime) {
		return ts.load()
	}
	return nil
}

func validateAlias(alias string) error {
	if alias == "" {
		return ErrAliasEmpty
	}
	if len([]rune(alias)) > 20 {
		return ErrAliasTooLong
	}
	for _, r := range alias {
		if r == '\n' || r == '\t' || r == '\x00' {
			return ErrAliasInvalidChar
		}
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != ' ' && r != '-' && r != '_' && r != '.' {
			return ErrAliasInvalidChar
		}
	}
	return nil
}

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

// SetPeerAlias 更新内存中的对端别名缓存（不持久化，通过 heartbeat 接收）。
func (ts *TrustStore) SetPeerAlias(uuid, alias string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.data.DeviceAliases == nil {
		ts.data.DeviceAliases = make(map[string]string)
	}
	ts.data.DeviceAliases[uuid] = alias
}

func FormatDisplayName(alias, hostname, uuid string) string {
	uuidPrefix := uuid
	if len(uuid) >= 8 {
		uuidPrefix = uuid[:8]
	}
	if alias != "" && hostname != "" {
		return alias + " (" + hostname + ")"
	}
	if alias != "" {
		return alias + " (" + uuidPrefix + ")"
	}
	if hostname != "" {
		return hostname + " (" + uuidPrefix + ")"
	}
	return uuid
}
