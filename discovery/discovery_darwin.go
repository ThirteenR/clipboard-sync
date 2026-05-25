package discovery

import (
	"bufio"
	"context"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"
)

func discoverDarwin(ctx context.Context, handler Handler) error {
	instances := make(chan string, 10)

	go func() {
		cmd := exec.CommandContext(ctx, "dns-sd", "-B", "_clipboardsync._tcp", "local.")
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			log.Printf("dns-sd -B start error: %v", err)
			close(instances)
			return
		}
		log.Printf("dns-sd -B started for _clipboardsync._tcp")

		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("dns-sd -B line: %s", line)
			if !strings.Contains(line, "_clipboardsync._tcp") {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 4 || fields[1] != "Add" {
				continue
			}
			inst := fields[len(fields)-1]
			if inst != "" {
				log.Printf("dns-sd -B found instance: %s", inst)
				instances <- inst
			}
		}
		log.Printf("dns-sd -B scanner done: %v", scanner.Err())
		cmd.Wait()
		close(instances)
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case inst, ok := <-instances:
			if !ok {
				return nil
			}
			resolveInstance(inst, handler)
		}
	}
}

func resolveInstance(instance string, handler Handler) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "dns-sd", "-L", instance, "_clipboardsync._tcp", "local.")
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return
	}

	var hostname, uuidStr string
	port := 8920

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "uuid=") {
			idx := strings.Index(line, "uuid=")
			uuidStr = strings.TrimSpace(line[idx+5:])
		}
		if strings.Contains(line, "can be reached at") {
			parts := strings.Split(line, "can be reached at ")
			if len(parts) == 2 {
				hostPart := strings.TrimSpace(parts[1])
				if colonIdx := strings.Index(hostPart, ":"); colonIdx > 0 {
					hostname = strings.TrimSuffix(hostPart[:colonIdx], ".local.")
				}
			}
		}
	}
	cmd.Wait()

	if uuidStr == "" || hostname == "" {
		return
	}

	addrs, err := net.LookupHost(hostname + ".local")
	if err != nil {
		addrs, err = net.LookupHost(hostname)
		if err != nil {
			log.Printf("Failed to resolve %s: %v", hostname, err)
			return
		}
	}

	addr := ""
	if len(addrs) > 0 {
		addr = addrs[0]
	}

	handler.OnJoin(PeerInfo{
		UUID:     uuidStr,
		Hostname: hostname,
		Addr:     addr,
		Port:     port,
	})
}
