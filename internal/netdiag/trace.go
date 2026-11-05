package netdiag

import (
	"bufio"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// HopType 表示路由追踪中一跳的类型。
type HopType string

const (
	HopLocal   HopType = "local"   // 本机
	HopPrivate HopType = "private" // 内网/私网跳（路由器、光猫等）
	HopCGNAT   HopType = "cgnat"   // 运营商级 NAT（100.64.0.0/10）
	HopPublic  HopType = "public"  // 公网出口
	HopTimeout HopType = "timeout" // 无响应（被防火墙隐藏）
)

// HopInfo 路由追踪中的一跳。
type HopInfo struct {
	Hop       int      `json:"hop"`
	IP        string   `json:"ip"`
	IPs       []string `json:"ips"`
	Type      HopType  `json:"type"`
	IsGateway bool     `json:"isGateway"`
}

// TraceResult 本地网络链路追踪结果。
type TraceResult struct {
	Target         string    `json:"target"`
	SourceIP       string    `json:"sourceIP"`
	Gateway        string    `json:"gateway"`
	Hops           []HopInfo `json:"hops"`
	NATLayers      int       `json:"natLayers"`
	FirstPublicHop int       `json:"firstPublicHop"`
	CGNATDetected  bool      `json:"cgnatDetected"`
	Summary        string    `json:"summary"`
}

// defaultTraceTarget 国内可达的公共 DNS，作为路由追踪终点。
const defaultTraceTarget = "223.5.5.5"

// TraceNetwork 一键追踪「本机 → 公网」的完整链路，并逐跳标注类型。
func TraceNetwork() *TraceResult {
	res := &TraceResult{
		Target:   defaultTraceTarget,
		SourceIP: firstLocalIPv4(),
		Gateway:  detectGateway(),
		Hops:     []HopInfo{},
	}

	// 链路起点：本机
	res.Hops = append(res.Hops, HopInfo{
		Hop: 0, IP: res.SourceIP, IPs: []string{res.SourceIP}, Type: HopLocal,
	})

	res.parseTrace(runTrace(res.Target))
	res.finalize()
	return res
}

func runTrace(target string) string {
	switch runtime.GOOS {
	case "windows":
		return runTimeout(60*time.Second, "tracert", "-d", "-h", "15", "-w", "600", target)
	default:
		out := runTimeout(60*time.Second, "traceroute", "-n", "-m", "15", "-w", "1", target)
		if strings.TrimSpace(out) == "" {
			out = runTimeout(60*time.Second, "tracepath", "-n", "-m", "15", target)
		}
		return out
	}
}

func (r *TraceResult) parseTrace(out string) {
	gatewayIP := net.ParseIP(r.Gateway)
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hop, err := strconv.Atoi(fields[0])
		if err != nil {
			continue // 表头等非跳行
		}

		var ips []string
		for _, f := range fields[1:] {
			if ip := net.ParseIP(f); ip != nil {
				ips = append(ips, f)
			}
		}

		info := HopInfo{Hop: hop, IPs: ips}
		if len(ips) > 0 {
			info.IP = ips[0]
			info.Type = hopType(net.ParseIP(ips[0]))
			if gatewayIP != nil && net.ParseIP(ips[0]).Equal(gatewayIP) {
				info.IsGateway = true
			}
		} else {
			info.Type = HopTimeout
		}
		r.Hops = append(r.Hops, info)
	}
}

func hopType(ip net.IP) HopType {
	switch classifyIPv4(ip) {
	case "private":
		return HopPrivate
	case "cgnat":
		return HopCGNAT
	case "public":
		return HopPublic
	default:
		return HopTimeout
	}
}

func (r *TraceResult) finalize() {
	layers := 0
	cgnat := false
	firstPublic := -1
	for _, h := range r.Hops {
		switch h.Type {
		case HopPrivate:
			layers++
		case HopCGNAT:
			layers++
			cgnat = true
		case HopPublic:
			if firstPublic == -1 {
				firstPublic = h.Hop
			}
		}
	}
	r.NATLayers = layers
	r.FirstPublicHop = firstPublic
	r.CGNATDetected = cgnat

	switch {
	case cgnat:
		r.Summary = fmt.Sprintf("链路中检测到 CGNAT 地址段（100.64.0.0/10），确认处于运营商级 NAT，约 %d 层", layers)
	case layers >= 2:
		r.Summary = fmt.Sprintf("检测到多层 NAT，本地网络约 %d 层", layers)
	case layers == 1:
		r.Summary = "单层 NAT（仅自家路由器），本地链路共 1 层"
	case layers == 0:
		r.Summary = "未发现内网跳（设备可能直连公网，或中间设备不响应 ICMP 被隐藏）"
	}
}
