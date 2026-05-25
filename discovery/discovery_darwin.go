package discovery

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"
)

func registerDarwin(ctx context.Context, instance, uuid string, port int) (*RegisterHandle, error) {
	rCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(rCtx, "dns-sd", "-R", instance,
		"_clipboardsync._tcp", "local.", fmt.Sprintf("%d", port), "uuid="+uuid)
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	log.Printf("dns-sd -R started for %s", instance)
	return &RegisterHandle{cancel: cancel}, nil
}

func discoverDarwin(ctx context.Context, handler Handler) error {
	instances := make(chan string, 10)

	go func() {
		cmd := exec.CommandContext(ctx, "script", "-q", "/dev/null",
			"dns-sd", "-B", "_clipboardsync._tcp", "local.")
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			log.Printf("dns-sd -B start error: %v", err)
			close(instances)
			return
		}
		log.Printf("dns-sd -B started for _clipboardsync._tcp")

		merged := bufio.NewScanner(io.MultiReader(stdout, stderr))
		for merged.Scan() {
			line := merged.Text()
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
		log.Printf("dns-sd -B scanner done: %v", merged.Err())
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "script", "-q", "/dev/null",
		"dns-sd", "-L", instance, "_clipboardsync._tcp", "local.")
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("dns-sd -L start error: %v", err)
		return
	}

	var hostname, uuidStr string
	port := 8920

	merged := bufio.NewScanner(io.MultiReader(stdout, stderr))
	for merged.Scan() {
		line := merged.Text()
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
		log.Printf("dns-sd -L resolve failed for %s: uuid=%q hostname=%q", instance, uuidStr, hostname)
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
