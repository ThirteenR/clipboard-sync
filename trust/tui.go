package trust

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"clipboard-sync/discovery"
)

type uiEntry struct {
	uuid     string
	hostname string
	trusted  bool
}

func RunTUI(store *TrustStore) {
	fmt.Println("Clipboard Sync - Trusted Devices")
	fmt.Println()

	entries := discoverEntries(store)
	if len(entries) == 0 {
		fmt.Println("No devices found on LAN.")
		fmt.Println()
		waitForExit()
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		printEntries(entries)
		fmt.Println()
		fmt.Print("Enter number to toggle, 's' save, 'q' quit: ")

		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "q", "quit":
			return
		case "s", "save":
			saveEntries(store, entries)
			fmt.Println("Saved.")
			return
		default:
			var n int
			if _, err := fmt.Sscanf(line, "%d", &n); err == nil && n >= 1 && n <= len(entries) {
				entries[n-1].trusted = !entries[n-1].trusted
			}
		}
	}
}

func printEntries(entries []uiEntry) {
	for i, e := range entries {
		check := " "
		if e.trusted {
			check = "*"
		}
		fmt.Printf("  %2d. [%s] %s  (%s)\n", i+1, check, e.hostname, e.uuid)
	}
}

func saveEntries(store *TrustStore, entries []uiEntry) {
	for _, e := range entries {
		if e.trusted {
			store.Add(e.uuid, e.hostname)
		} else {
			store.Remove(e.uuid)
		}
	}
}

func discoverEntries(store *TrustStore) []uiEntry {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	seen := make(map[string]bool)
	var entries []uiEntry

	fmt.Print("Searching for devices on LAN...")
	handler := discovery.Handler{
		OnJoin: func(info discovery.PeerInfo) {
			if info.UUID == "" || seen[info.UUID] {
				return
			}
			seen[info.UUID] = true
			trusted := store.IsTrusted(info.UUID)
			entries = append(entries, uiEntry{
				uuid:     info.UUID,
				hostname: info.Hostname,
				trusted:  trusted,
			})
			fmt.Print(".")
		},
	}

	if err := discovery.Discover(ctx, handler); err != nil {
		log.Printf("Discovery error: %v", err)
	}
	fmt.Println(" done")

	for _, de := range store.List() {
		if !seen[de.UUID] {
			entries = append(entries, uiEntry{
				uuid:     de.UUID,
				hostname: de.Hostname,
				trusted:  de.Trusted,
			})
		}
	}

	return entries
}

func waitForExit() {
	fmt.Print("Press Enter to exit.")
	bufio.NewScanner(os.Stdin).Scan()
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
