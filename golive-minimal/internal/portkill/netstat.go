package portkill

import (
	"bufio"
	"net"
	"strings"
)

// ListeningPIDsFromNetstat returns PIDs in LISTENING state on the given TCP port.
// port is the numeric port only, e.g. "9060".
func ListeningPIDsFromNetstat(output, port string) []string {
	seen := make(map[string]struct{})
	var pids []string
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 5 {
			continue
		}
		if !strings.EqualFold(fields[0], "TCP") {
			continue
		}
		if !strings.EqualFold(fields[3], "LISTENING") {
			continue
		}
		if tcpPort(fields[1]) != port {
			continue
		}
		pid := fields[len(fields)-1]
		if pid == "" || pid == "0" {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		pids = append(pids, pid)
	}
	return pids
}

func tcpPort(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	_, p, err := net.SplitHostPort(addr)
	if err == nil {
		return p
	}
	i := strings.LastIndex(addr, ":")
	if i < 0 || i == len(addr)-1 {
		return ""
	}
	return addr[i+1:]
}
