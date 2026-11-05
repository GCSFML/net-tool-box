package netdiag

import (
	"bufio"
	"fmt"
	"net"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// LocalPortResult 本机端口占用检测结果。
type LocalPortResult struct {
	Port    int    `json:"port"`
	InUse   bool   `json:"inUse"`
	Message string `json:"message"`
}

// CheckLocalPort 检测本机指定端口是否被占用（以 TCP 能否绑定为准）。
func CheckLocalPort(port int) *LocalPortResult {
	res := &LocalPortResult{Port: port}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		res.InUse = true
		res.Message = "端口已被占用"
		return res
	}
	_ = ln.Close()
	res.InUse = false
	res.Message = "端口空闲（可绑定）"
	return res
}

// ListeningPort 一条监听端口记录。
type ListeningPort struct {
	Proto string `json:"proto"`
	Port  int    `json:"port"`
	Addr  string `json:"addr"`
	PID   string `json:"pid,omitempty"`
}

// ListListeningPorts 列出本机正在监听的端口（尽力而为，依赖系统命令）。
func ListListeningPorts() []ListeningPort {
	if runtime.GOOS == "windows" {
		return listPortsWindows()
	}
	return listPortsUnix()
}

func listPortsWindows() []ListeningPort {
	out := run("netstat", "-ano", "-p", "tcp")
	out += "\n" + run("netstat", "-ano", "-p", "udp")

	var res []ListeningPort
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		proto := fields[0]
		if proto != "TCP" && proto != "UDP" {
			continue
		}
		local := fields[1]
		idx := strings.LastIndex(local, ":")
		if idx < 0 {
			continue
		}
		port, err := strconv.Atoi(local[idx+1:])
		if err != nil || port <= 0 {
			continue
		}
		lp := ListeningPort{Proto: proto, Port: port, Addr: local}
		if proto == "TCP" && len(fields) >= 5 && fields[3] == "LISTENING" {
			lp.PID = fields[4]
		} else if proto == "UDP" && len(fields) >= 4 {
			lp.PID = fields[3]
		} else if proto == "TCP" {
			continue // 非 LISTENING 的 TCP 连接不显示
		}
		res = append(res, lp)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Port < res[j].Port })
	return dedupePorts(res)
}

func listPortsUnix() []ListeningPort {
	out := run("ss", "-tuln")
	if strings.TrimSpace(out) == "" {
		out = run("netstat", "-tuln")
	}
	var res []ListeningPort
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, "LISTEN") && !strings.Contains(line, "UNCONN") {
			continue
		}
		proto := "TCP"
		if strings.Contains(line, "udp") || strings.Contains(line, "UNCONN") {
			proto = "UDP"
		}
		for _, f := range strings.Fields(line) {
			idx := strings.LastIndex(f, ":")
			if idx < 0 {
				continue
			}
			port, err := strconv.Atoi(f[idx+1:])
			if err == nil && port > 0 {
				res = append(res, ListeningPort{Proto: proto, Port: port, Addr: f})
				break
			}
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Port < res[j].Port })
	return dedupePorts(res)
}

func dedupePorts(in []ListeningPort) []ListeningPort {
	seen := map[string]bool{}
	var out []ListeningPort
	for _, p := range in {
		key := p.Proto + ":" + strconv.Itoa(p.Port)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, p)
	}
	return out
}

// RemotePortResult 远端端口连通性检测结果。
type RemotePortResult struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Open      bool   `json:"open"`
	LatencyMs int64  `json:"latencyMs"`
	Error     string `json:"error,omitempty"`
}

// CheckRemotePort 检测远端主机端口是否可连通（TCP 连接）。
func CheckRemotePort(host string, port int, timeout time.Duration) *RemotePortResult {
	res := &RemotePortResult{Host: host, Port: port}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	res.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		res.Open = false
		res.Error = err.Error()
		return res
	}
	_ = conn.Close()
	res.Open = true
	return res
}
