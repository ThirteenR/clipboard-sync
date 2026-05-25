package trust

import (
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

func TestIsTrustedNonexistent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, _ := New()
	if ts.IsTrusted("nonexistent") {
		t.Error("expected false for nonexistent UUID")
	}
}

func TestConfigDirUsesXDG(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	got := configDir()
	want := filepath.Join(dir, "clipboardsync")
	if got != want {
		t.Errorf("configDir() = %s, want %s", got, want)
	}
}

func TestListAfterAdd(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, _ := New()
	ts.Add("uuid-1", "host-1")
	entries := ts.List()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !entries[0].Trusted {
		t.Error("expected entry to be trusted")
	}
	if entries[0].Hostname != "host-1" {
		t.Errorf("expected hostname host-1, got %s", entries[0].Hostname)
	}
}
