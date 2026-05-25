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
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
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
