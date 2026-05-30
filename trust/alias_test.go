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

func TestDeviceAliasStorage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, err := New()
	if err != nil {
		t.Fatalf("Failed to create trust store: %v", err)
	}

	if ts.HasDeviceAlias() {
		t.Error("Expected no device alias initially")
	}

	if err := ts.SetDeviceAlias("test-device"); err != nil {
		t.Fatalf("Failed to set device alias: %v", err)
	}

	alias := ts.GetDeviceAlias()
	if alias != "test-device" {
		t.Errorf("GetDeviceAlias() = %q, want %q", alias, "test-device")
	}

	if !ts.HasDeviceAlias() {
		t.Error("Expected HasDeviceAlias() to return true")
	}

	if err := ts.SetDeviceAlias("updated-device"); err != nil {
		t.Fatalf("Failed to update device alias: %v", err)
	}

	alias = ts.GetDeviceAlias()
	if alias != "updated-device" {
		t.Errorf("GetDeviceAlias() after update = %q, want %q", alias, "updated-device")
	}
}

func TestPeerAliasStorage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	ts, err := New()
	if err != nil {
		t.Fatalf("Failed to create trust store: %v", err)
	}

	alias := ts.GetPeerAlias("test-uuid")
	if alias != "" {
		t.Errorf("GetPeerAlias() initially = %q, want empty", alias)
	}

	ts.SetPeerAlias("test-uuid", "peer-device")

	alias = ts.GetPeerAlias("test-uuid")
	if alias != "peer-device" {
		t.Errorf("GetPeerAlias() = %q, want %q", alias, "peer-device")
	}

	ts.SetPeerAlias("uuid-2", "second-peer")

	alias1 := ts.GetPeerAlias("test-uuid")
	alias2 := ts.GetPeerAlias("uuid-2")
	if alias1 != "peer-device" || alias2 != "second-peer" {
		t.Errorf("Peer aliases mismatch: got %q and %q", alias1, alias2)
	}
}

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
