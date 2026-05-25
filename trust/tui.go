package trust

import (
	"context"
	"fmt"
	"log"
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
	saved    bool
	quitting bool
}

type discoveryDoneMsg struct {
	entries []entry
	err     error
}

func initialModel(store *TrustStore) model {
	return model{
		loading: true,
		store:   store,
	}
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
	case tea.WindowSizeMsg:
		// no special handling needed
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
	if m.loading {
		return " Searching for devices on LAN...\n\n Press q to quit."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\n Press q to quit.", m.err)
	}
	if len(m.entries) == 0 {
		return "No devices found on LAN.\n\n Press q to quit."
	}

	var b strings.Builder
	b.WriteString("Clipboard Sync - Trusted Devices\n\n")
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
	b.WriteString("\n \x1b[90mup/down navigate\x1b[0m  \x1b[90mspace toggle\x1b[0m  \x1b[90ms save\x1b[0m  \x1b[90mq quit\x1b[0m")
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
