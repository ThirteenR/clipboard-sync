package dedup

import (
	"testing"
	"time"
)

func TestHash(t *testing.T) {
	d := New(5 * time.Minute)
	h1 := d.Hash("hello")
	h2 := d.Hash("hello")
	h3 := d.Hash("world")

	if h1 != h2 {
		t.Errorf("same content should produce same hash: got %s and %s", h1, h2)
	}
	if h1 == h3 {
		t.Errorf("different content should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("sha256 hex should be 64 chars, got %d", len(h1))
	}
}

func TestSeen(t *testing.T) {
	d := New(5 * time.Minute)
	h := d.Hash("test")
	if d.Seen(h) {
		t.Error("hash should not be seen before Mark")
	}
	d.Mark(h)
	if !d.Seen(h) {
		t.Error("hash should be seen after Mark")
	}
}

func TestMultipleItems(t *testing.T) {
	d := New(5 * time.Minute)
	a := d.Hash("a")
	b := d.Hash("b")

	d.Mark(a)
	if !d.Seen(a) {
		t.Error("a should be seen")
	}
	if d.Seen(b) {
		t.Error("b should not be seen")
	}

	d.Mark(b)
	if !d.Seen(b) {
		t.Error("b should be seen after Mark")
	}
}

func TestTTLExpiry(t *testing.T) {
	d := New(1 * time.Millisecond)
	defer d.Stop()
	h := d.Hash("expire-me")
	d.Mark(h)
	if !d.Seen(h) {
		t.Fatal("hash should be seen immediately after Mark")
	}
	time.Sleep(10 * time.Millisecond)
	if d.Seen(h) {
		t.Error("hash should not be seen after TTL expiry")
	}
}
