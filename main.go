package main

import (
	"context"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
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
	if len(os.Args) <= 1 || os.Args[1] == "help" || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printUsage()
		return
	}

	if os.Args[1] == "uuid" {
		id := loadOrCreateUUID()
		os.Stdout.WriteString(id + "\n")
		return
	}

	if os.Args[1] == "start" {
		startBackground()
		return
	}

	if os.Args[1] == "stop" {
		stopBackground()
		return
	}

	if os.Args[1] == "run" {
		runForeground()
		return
	}

	if os.Args[1] == "trust" {
		store, err := trust.New()
		if err != nil {
			log.Fatalf("Failed to load trust store: %v", err)
		}
		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "list":
				log.SetOutput(io.Discard)
				trust.RunList(store)
			case "add":
				if len(os.Args) < 4 {
					log.Fatal("Usage: clipboardsync trust add <uuid>")
				}
				store.Add(os.Args[3], os.Args[3])
				log.Printf("Added %s to trusted devices", os.Args[3])
			case "remove":
				if len(os.Args) < 4 {
					log.Fatal("Usage: clipboardsync trust remove <uuid>")
				}
				store.Remove(os.Args[3])
				log.Printf("Removed %s from trusted devices", os.Args[3])
			default:
				log.Fatalf("Unknown trust subcommand: %s", os.Args[2])
			}
		} else {
			log.SetOutput(io.Discard)
			trust.RunTUI(store, loadOrCreateUUID())
		}
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "alias" {
		store, err := trust.New()
		if err != nil {
			log.Fatalf("Failed to load trust store: %v", err)
		}
		deviceUUID := loadOrCreateUUID()

		if len(os.Args) > 2 {
			switch os.Args[2] {
			case "set":
				if len(os.Args) < 4 {
					log.Fatal("Usage: clipboardsync alias set <alias>")
				}
				store.SetAliasCommand(os.Args[3], deviceUUID)
			case "show":
				store.ShowAliasCommand()
			default:
				log.Fatalf("Unknown alias subcommand: %s", os.Args[2])
			}
		} else {
			store.RunAliasTUI(deviceUUID)
		}
		return
	}

	printUsage()
}

// isTerminal 检查 stdin 是否是终端
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func loadOrCreateUUID() string {
	uuidFile := filepath.Join(os.TempDir(), "clipboard-sync-uuid")
	data, err := os.ReadFile(uuidFile)
	if err == nil && len(data) > 0 {
		return strings.TrimSpace(string(data))
	}
	id := uuid.New().String()
	os.WriteFile(uuidFile, []byte(id), 0644)
	return id
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.ReplaceAll(s[:n], "\n", " ") + "..."
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func sendToPeer(peer sync.PeerInfo, data []byte) {
	addr := net.JoinHostPort(peer.Addr, itoa(peer.Port))
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		log.Printf("Failed to connect to %s (%s): %v", peer.Hostname, peer.UUID, err)
		return
	}
	defer conn.Close()

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		log.Printf("SetWriteDeadline to %s failed: %v", peer.Hostname, err)
		return
	}
	if _, err := conn.Write(data); err != nil {
		log.Printf("Write to %s failed: %v", peer.Hostname, err)
	}
}

func handleInbound(conn net.Conn, deviceUUID string, trustStore *trust.TrustStore, deduper *dedup.Dedup) {
	defer conn.Close()

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	msg, err := sync.Decode(conn)
	if err != nil {
		log.Printf("Failed to decode message: %v", err)
		return
	}

	if msg.Sender == deviceUUID {
		return
	}

	if !trustStore.IsTrusted(msg.Sender) {
		log.Printf("Skipped message from untrusted device: %s", truncate(msg.Sender, 16))
		return
	}

	if msg.Type == "hello" {
		log.Printf("Received hello from %s", msg.Sender)
		return
	}

	if msg.Type == "clipboard" {
		if deduper.Seen(msg.Hash) {
			return
		}
		deduper.Mark(msg.Hash)
		log.Printf("Received clipboard from %s: %s", msg.Sender, truncate(msg.Content, 50))
		if err := clipboard.Write(msg.Content); err != nil {
			log.Printf("Failed to write clipboard: %v", err)
		}
	}
}

func runForeground() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	log.Println("Clipboard Sync starting...")

	deviceUUID := loadOrCreateUUID()
	log.Printf("Device UUID: %s", deviceUUID)

	trustStore, err := trust.New()
	if err != nil {
		log.Fatalf("Failed to load trust store: %v", err)
	}
	log.Printf("Trust store loaded")

	if !trustStore.HasDeviceAlias() {
		if isTerminal() {
			log.Println("设备别名未设置，提示用户设置...")
			trustStore.PromptSetDeviceAlias(deviceUUID)
		} else {
			hostname, _ := os.Hostname()
			if err := trustStore.SetDeviceAlias(hostname); err != nil {
				log.Printf("设置默认别名失败: %v", err)
			} else {
				log.Printf("设备别名自动设置为: %s", hostname)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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

	pm := sync.NewPeerManager()

	go func() {
		w := clipboard.New(func(text string) {
			hash := deduper.Hash(text)
			if deduper.Seen(hash) {
				return
			}
			deduper.Mark(hash)
			log.Printf("Local clipboard changed, broadcasting to %d peers...", pm.Len())

			msg := sync.Message{
				ID:        uuid.New().String(),
				Hash:      hash,
				Type:      "clipboard",
				Content:   text,
				Timestamp: time.Now().UnixMilli(),
				Sender:    deviceUUID,
			}
			data, _ := sync.Encode(msg)

			for _, peer := range pm.All() {
				go sendToPeer(peer, data)
			}
		})
		if err := w.Start(ctx); err != nil {
			log.Fatalf("Clipboard watcher failed: %v", err)
		}
	}()

	go func() {
		handler := discovery.Handler{
			OnJoin: func(info discovery.PeerInfo) {
				if info.UUID == "" || info.UUID == deviceUUID || info.Addr == "" {
					return
				}
				pm.Add(sync.PeerInfo{
					UUID:     info.UUID,
					Hostname: info.Hostname,
					Addr:     info.Addr,
					Port:     info.Port,
				})
				alias := trustStore.GetPeerAlias(info.UUID)
				displayName := trust.FormatDisplayName(alias, info.Hostname, info.UUID)
				log.Printf("设备发现: %s (%s:%d)", displayName, info.Addr, info.Port)
			},
			OnLeave: func(info discovery.PeerInfo) {
				log.Printf("Peer left: %s (%s)", info.Hostname, info.UUID)
				pm.Remove(info.UUID)
			},
			OnAliasUpdate: func(uuid, alias string) {
				trustStore.SetPeerAlias(uuid, alias)
				log.Printf("设备 %s 别名更新: %s", uuid, alias)
			},
		}

		if err := discovery.Discover(ctx, handler); err != nil {
			log.Printf("Discovery error: %v", err)
		}
	}()

	go func() {
		listener, err := net.Listen("tcp", net.JoinHostPort("0.0.0.0", itoa(ServicePort)))
		if err != nil {
			log.Fatalf("Failed to listen: %v", err)
		}
		defer listener.Close()
		log.Printf("TCP server listening on :%d", ServicePort)

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Accept error: %v", err)
				continue
			}
			go handleInbound(conn, deviceUUID, trustStore, deduper)
		}
	}()

	go func() {
		hostname, _ := os.Hostname()
		server, err := discovery.Register(ctx, hostname, deviceUUID, hostname, ServicePort, trustStore)
		if err != nil {
			log.Fatalf("Failed to register mDNS service: %v", err)
		}
		defer server.Shutdown()
		log.Printf("mDNS service registered as %s", hostname)
		<-ctx.Done()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Println("Shutting down...")
	cancel()
	time.Sleep(500 * time.Millisecond)
}

func printUsage() {
	help := `Clipboard Sync - 局域网剪贴板同步工具

用法:
  clipboardsync              显示此帮助
  clipboardsync run          前台运行（Ctrl+C 停止）
  clipboardsync start        后台运行
  clipboardsync stop         停止后台运行
  clipboardsync trust        管理受信任设备
  clipboardsync alias        管理设备别名
  clipboardsync uuid         显示本机 UUID
  clipboardsync help         显示此帮助

更多信息: https://github.com/ThirteenR/clipboard-sync
`
	os.Stdout.WriteString(help)
}

func startBackground() {
	// 检查是否已在运行
	pidFile := getPidFile()
	if data, err := os.ReadFile(pidFile); err == nil {
		pid := strings.TrimSpace(string(data))
		if isProcessRunning(pid) {
			log.Printf("Clipboard Sync is already running (PID: %s)", pid)
			return
		}
	}

	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}

	// 检查并设置设备别名
	trustStore, err := trust.New()
	if err != nil {
		log.Fatalf("Failed to load trust store: %v", err)
	}
	deviceUUID := loadOrCreateUUID()
	if !trustStore.HasDeviceAlias() {
		if isTerminal() {
			log.Println("设备别名未设置，提示用户设置...")
			trustStore.PromptSetDeviceAlias(deviceUUID)
		} else {
			hostname, _ := os.Hostname()
			if err := trustStore.SetDeviceAlias(hostname); err != nil {
				log.Printf("设置默认别名失败: %v", err)
			} else {
				log.Printf("设备别名自动设置为: %s", hostname)
			}
		}
	}

	// 启动后台进程
	logFile := getLogFile()
	log.Printf("Starting Clipboard Sync in background...")
	log.Printf("Log file: %s", logFile)

	// 创建日志文件
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close()

	cmd := exec.Command(execPath, "run")
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil

	// 设置进程属性，使其与终端分离
	setDetachAttr(cmd)

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start background process: %v", err)
	}

	// 保存 PID
	if err := os.WriteFile(pidFile, []byte(itoa(cmd.Process.Pid)), 0644); err != nil {
		log.Printf("Failed to save PID file: %v", err)
	}

	log.Printf("Clipboard Sync started in background (PID: %d)", cmd.Process.Pid)
	log.Printf("Use 'clipboardsync stop' to stop")
}

func stopBackground() {
	pidFile := getPidFile()
	data, err := os.ReadFile(pidFile)
	if err != nil {
		log.Printf("Clipboard Sync is not running (no PID file)")
		return
	}

	pid := strings.TrimSpace(string(data))
	if !isProcessRunning(pid) {
		log.Printf("Clipboard Sync is not running (stale PID file)")
		os.Remove(pidFile)
		return
	}

	// 使用 kill 命令精确终止指定 PID
	killCmd := exec.Command("kill", pid)
	output, err := killCmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to stop process: %v, output: %s", err, string(output))
	}

	// 等待进程退出
	for i := 0; i < 10; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isProcessRunning(pid) {
			break
		}
	}

	// 清理 PID 文件
	os.Remove(pidFile)
	log.Printf("Clipboard Sync stopped (PID: %s)", pid)
}

func getPidFile() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "clipboardsync.pid")
	}
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "clipboardsync", "clipboardsync.pid")
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "clipboardsync", "clipboardsync.pid")
		}
		return filepath.Join(os.TempDir(), "clipboardsync.pid")
	}
	return "/tmp/clipboardsync.pid"
}

func getLogFile() string {
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".config", "clipboardsync", "clipboardsync.log")
	}
	if runtime.GOOS == "windows" {
		if appdata := os.Getenv("APPDATA"); appdata != "" {
			return filepath.Join(appdata, "clipboardsync", "clipboardsync.log")
		}
		return filepath.Join(os.TempDir(), "clipboardsync.log")
	}
	return "/tmp/clipboardsync.log"
}

func isProcessRunning(pid string) bool {
	if pid == "" {
		return false
	}
	// macOS 上 FindProcess 总是成功，Signal(0) 对非子进程可能失败
	// 直接检查 /proc 或使用 ps 命令
	cmd := exec.Command("ps", "-p", pid)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}
