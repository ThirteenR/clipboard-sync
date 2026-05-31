package trust

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"clipboard-sync/discovery"

	tea "github.com/charmbracelet/bubbletea"
)

type entry struct {
	uuid     string
	hostname string
	trusted  bool
	online   bool
}

type model struct {
	entries      []entry
	cursor       int
	discovering  bool
	discoveryErr error
	store        *TrustStore
	saved        bool
	quitting     bool
	aliasMode    bool
	aliasInput   string
	deviceUUID   string
}

type discoveryDoneMsg struct {
	entries []entry
	err     error
}

func initialModel(store *TrustStore, deviceUUID string) model {
	entries := storedEntries(store)
	return model{
		entries:     entries,
		discovering: true,
		store:       store,
		deviceUUID:  deviceUUID,
	}
}

func storedEntries(store *TrustStore) []entry {
	var entries []entry
	for _, de := range store.List() {
		entries = append(entries, entry{
			uuid:     de.UUID,
			hostname: de.Hostname,
			trusted:  de.Trusted,
			online:   false,
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
				online:   true,
			})
		},
		OnAliasUpdate: func(uuid, alias string) {
			m.store.SetPeerAlias(uuid, alias)
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
				online:   false,
			})
		}
	}

	return discoveryDoneMsg{entries: entries}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.aliasMode {
			switch msg.String() {
			case "enter":
				alias := strings.TrimSpace(m.aliasInput)
				if alias == "" {
					alias, _ = os.Hostname()
				}
				if err := m.store.SetDeviceAlias(alias); err != nil {
					m.aliasMode = false
					return m, nil
				}
				m.aliasMode = false
				return m, nil
			case "esc":
				m.aliasMode = false
				return m, nil
			case "backspace":
				if len(m.aliasInput) > 0 {
					m.aliasInput = m.aliasInput[:len(m.aliasInput)-1]
				}
			default:
				if len(msg.String()) == 1 && len(m.aliasInput) < 20 {
					m.aliasInput += msg.String()
				}
			}
			return m, nil
		}
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
		case "a":
			m.aliasMode = true
			m.aliasInput = m.store.GetDeviceAlias()
			return m, nil
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

	if m.aliasMode {
		var b strings.Builder
		b.WriteString("Clipboard Sync - 设备别名管理\n\n")
		currentAlias := m.store.GetDeviceAlias()
		if currentAlias == "" {
			currentAlias = "(未设置)"
		}
		b.WriteString(fmt.Sprintf("当前别名: %s\n", currentAlias))
		b.WriteString(fmt.Sprintf("UUID: %s\n\n", m.deviceUUID))
		b.WriteString(fmt.Sprintf("输入新别名 (最大20字符): %s\n", m.aliasInput))
		b.WriteString("\n\x1b[90menter 确认\x1b[0m  \x1b[90mesc 取消\x1b[0m")
		return b.String()
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
				online := "  "
				if e.online {
					online = " ●"
				}
				alias := m.store.GetPeerAlias(e.uuid)
				displayName := FormatDisplayName(alias, e.hostname, e.uuid)
				b.WriteString(fmt.Sprintf("%s[%s]%s %s\n", cursor, check, online, displayName))
			}
		}

	if m.discovering {
		b.WriteString("\n Searching for devices on LAN... (8s)")
	}
	if m.discoveryErr != nil {
		b.WriteString(fmt.Sprintf("\n Discovery error: %v", m.discoveryErr))
	}

	b.WriteString("\n")
	b.WriteString("\n \x1b[90mup/down\x1b[0m  \x1b[90mspace toggle\x1b[0m  \x1b[90ma alias\x1b[0m  \x1b[90ms save\x1b[0m  \x1b[90mr rediscover\x1b[0m  \x1b[90mq quit\x1b[0m")
	return b.String()
}

func RunTUI(store *TrustStore, deviceUUID string) {
	p := tea.NewProgram(initialModel(store, deviceUUID))
	if _, err := p.Run(); err != nil {
		log.Fatalf("TUI failed: %v", err)
	}
}

func RunList(store *TrustStore) {
	type device struct {
		hostname string
		uuid     string
		trusted  bool
		online   bool
		seen     string
	}
	deviceMap := make(map[string]*device)

	// 先加载已存储的设备
	for _, de := range store.List() {
		deviceMap[de.UUID] = &device{
			uuid:     de.UUID,
			hostname: de.Hostname,
			trusted:  de.Trusted,
			online:   false,
			seen:     de.LastSeen,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	fmt.Fprint(os.Stderr, "Searching for devices...")
	discovery.Discover(ctx, discovery.Handler{
		OnJoin: func(info discovery.PeerInfo) {
			if info.UUID == "" {
				return
			}
			if existing, ok := deviceMap[info.UUID]; ok {
				// 更新已存在设备的在线状态和主机名
				existing.online = true
				existing.hostname = info.Hostname
				existing.seen = time.Now().Format(time.RFC3339)
			} else {
				// 新发现的设备
				trusted := store.IsTrusted(info.UUID)
				deviceMap[info.UUID] = &device{
					uuid:     info.UUID,
					hostname: info.Hostname,
					trusted:  trusted,
					online:   true,
					seen:     time.Now().Format(time.RFC3339),
				}
			}
			fmt.Fprint(os.Stderr, ".")
		},
		OnAliasUpdate: func(uuid, alias string) {
			store.SetPeerAlias(uuid, alias)
		},
	})
	fmt.Fprintln(os.Stderr, " done")

	if len(deviceMap) == 0 {
		fmt.Println("No devices found on LAN.")
		return
	}

	var devices []device
	for _, d := range deviceMap {
		devices = append(devices, *d)
	}

	// 排序：在线设备优先，然后按主机名排序
	sort.Slice(devices, func(i, j int) bool {
		if devices[i].online != devices[j].online {
			return devices[i].online
		}
		return devices[i].hostname < devices[j].hostname
	})

	fmt.Println()
	for _, d := range devices {
		check := " "
		if d.trusted {
			check = "*"
		}
		online := "  "
		if d.online {
			online = " ●"
		}
		alias := store.GetPeerAlias(d.uuid)
		displayName := FormatDisplayName(alias, d.hostname, d.uuid)
		fmt.Printf("  [%s]%s %s", check, online, displayName)
		if !d.trusted {
			fmt.Print("  (untrusted)")
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("Run 'clipboardsync trust add <uuid>' to trust a device.")
}
