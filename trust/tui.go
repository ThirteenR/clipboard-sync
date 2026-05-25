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
	entries    []entry
	cursor     int
	discovering bool
	discoveryErr error
	store      *TrustStore
	saved      bool
	quitting   bool
}

type discoveryDoneMsg struct {
	entries []entry
	err     error
}

func initialModel(store *TrustStore) model {
	entries := storedEntries(store)
	return model{
		entries:     entries,
		discovering: true,
		store:       store,
	}
}

func storedEntries(store *TrustStore) []entry {
	var entries []entry
	for _, de := range store.List() {
		entries = append(entries, entry{
			uuid:     de.UUID,
			hostname: de.Hostname,
			trusted:  de.Trusted,
		})
	}
	return entries
}

func (m model) Init() tea.Cmd {
	return m.discover
}

func (m model) discover() tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
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
	}

	if err := discovery.Discover(ctx, handler); err != nil {
		return discoveryDoneMsg{err: err}
	}

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
			m.saved = true
			return m, tea.Quit
		case "r":
			m.discovering = true
			m.discoveryErr = nil
			return m, m.discover
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
		m.discovering = false
		if msg.err != nil {
			m.discoveryErr = msg.err
		} else {
			m.entries = msg.entries
		}
	case tea.WindowSizeMsg:
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
	if m.saved {
		return "Saved.\n"
	}
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString("Clipboard Sync - Trusted Devices\n\n")

	if len(m.entries) == 0 && !m.discovering {
		b.WriteString("No devices found on LAN.\n")
	} else {
		for i, e := range m.entries {
			cursor := "  "
			if i == m.cursor {
				cursor = "> "
			}
			check := " "
			if e.trusted {
				check = "x"
			}
			b.WriteString(fmt.Sprintf("%s[%s] %s  (%s)\n", cursor, check, e.hostname, e.uuid))
		}
	}

	if m.discovering {
		b.WriteString("\n Searching for devices on LAN... (8s)")
	}
	if m.discoveryErr != nil {
		b.WriteString(fmt.Sprintf("\n Discovery error: %v", m.discoveryErr))
	}

	b.WriteString("\n")
	b.WriteString("\n \x1b[90mup/down\x1b[0m  \x1b[90mspace toggle\x1b[0m  \x1b[90ms save\x1b[0m  \x1b[90mr rediscover\x1b[0m  \x1b[90mq quit\x1b[0m")
	return b.String()
}

func RunTUI(store *TrustStore) {
	p := tea.NewProgram(initialModel(store))
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI failed: %v", err)
	}
}

func RunList(store *TrustStore) {
	type device struct {
		hostname string
		uuid     string
		trusted  bool
		seen     string
	}
	seen := make(map[string]bool)
	var devices []device

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	fmt.Fprint(os.Stderr, "Searching for devices...")
	discovery.Discover(ctx, discovery.Handler{
		OnJoin: func(info discovery.PeerInfo) {
			if info.UUID == "" || seen[info.UUID] {
				return
			}
			seen[info.UUID] = true
			trusted := store.IsTrusted(info.UUID)
			devices = append(devices, device{
				uuid:     info.UUID,
				hostname: info.Hostname,
				trusted:  trusted,
				seen:     time.Now().Format(time.RFC3339),
			})
			fmt.Fprint(os.Stderr, ".")
		},
	})
	fmt.Fprintln(os.Stderr, " done")

	for _, de := range store.List() {
		if !seen[de.UUID] {
			devices = append(devices, device{
				uuid:     de.UUID,
				hostname: de.Hostname,
				trusted:  de.Trusted,
				seen:     de.LastSeen,
			})
		}
	}

	if len(devices) == 0 {
		fmt.Println("No devices found on LAN.")
		return
	}

	fmt.Println()
	for _, d := range devices {
		check := " "
		if d.trusted {
			check = "*"
		}
		fmt.Printf("  [%s] %s  (%s)", check, d.hostname, d.uuid)
		if !d.trusted {
			fmt.Print("  (untrusted)")
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("Run 'clipboardsync trust add <uuid>' to trust a device.")
}
