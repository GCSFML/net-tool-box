package netdiag

import (
	"bufio"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// AddrInfo 描述网卡上的一个 IP 地址。
type AddrInfo struct {
	IP          string `json:"ip"`
	Version     string `json:"version"`
	IsLoopback  bool   `json:"isLoopback"`
	IsPrivate   bool   `json:"isPrivate"`
	IsLinkLocal bool   `json:"isLinkLocal"`
	CIDR        string `json:"cidr"`
}

// InterfaceInfo 描述一块网卡。
type InterfaceInfo struct {
	Name     string     `json:"name"`
	MAC      string     `json:"mac"`
	MTU      int        `json:"mtu"`
	Up       bool       `json:"up"`
	Loopback bool       `json:"loopback"`
	Addrs    []AddrInfo `json:"addrs"`
}

// LocalIPResult 本地网络信息汇总。
type LocalIPResult struct {
	Interfaces []InterfaceInfo `json:"interfaces"`
	Gateway    string          `json:"gateway"`
	DNS        []string        `json:"dns"`
}

// GetLocalIP 枚举本机网卡、IP 地址、默认网关和 DNS。
func GetLocalIP() (*LocalIPResult, error) {
	res := &LocalIPResult{DNS: []string{}}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		info := InterfaceInfo{
			Name:     iface.Name,
			MAC:      iface.HardwareAddr.String(),
			MTU:      iface.MTU,
			Up:       iface.Flags&net.FlagUp != 0,
			Loopback: iface.Flags&net.FlagLoopback != 0,
			Addrs:    []AddrInfo{},
		}
		addrs, err := iface.Addrs()
		if err != nil {
			res.Interfaces = append(res.Interfaces, info)
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipnet.IP
			ver := "IPv6"
			if ip.To4() != nil {
				ver = "IPv4"
			}
			info.Addrs = append(info.Addrs, AddrInfo{
				IP:          ip.String(),
				Version:     ver,
				IsLoopback:  ip.IsLoopback(),
				IsPrivate:   ip.IsPrivate(),
				IsLinkLocal: ip.IsLinkLocalUnicast(),
				CIDR:        ipnet.String(),
			})
		}
		res.Interfaces = append(res.Interfaces, info)
	}

	res.Gateway = detectGateway()
	res.DNS = detectDNS()
	return res, nil
}

func detectGateway() string {
	switch runtime.GOOS {
	case "windows":
		return detectGatewayWindows()
	case "darwin":
		return detectGatewayDarwin()
	default:
		return detectGatewayLinux()
	}
}

// detectGatewayWindows 解析 `route print -4` 中的默认路由（数据行为纯数字，与系统语言无关）。
func detectGatewayWindows() string {
	out := run("route", "print", "-4")
	bestMetric := 1 << 30
	bestGW := ""
	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		if fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			metric := 0
			if len(fields) >= 5 {
				metric, _ = strconv.Atoi(fields[4])
			}
			if metric < bestMetric {
				bestMetric = metric
				bestGW = fields[2]
			}
		}
	}
	return bestGW
}

func detectGatewayDarwin() string {
	out := run("route", "-n", "get", "default")
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "gateway:" {
			return fields[1]
		}
	}
	return ""
}

func detectGatewayLinux() string {
	out := run("ip", "route", "show", "default")
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "via" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	// 回退：读取 /proc/net/route
	if b, err := os.ReadFile("/proc/net/route"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(b)))
		scanner.Scan() // 跳过表头
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 3 && fields[1] == "00000000" {
				if gw := decodeRouteHex(fields[2]); gw != "" {
					return gw
				}
			}
		}
	}
	return ""
}

// decodeRouteHex 解析 /proc/net/route 中小端序十六进制网关。
func decodeRouteHex(h string) string {
	if len(h) != 8 {
		return ""
	}
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseUint(h[6-2*i:8-2*i], 16, 8)
		if err != nil {
			return ""
		}
		b[i] = byte(v)
	}
	return net.IP(b).String()
}

func detectDNS() []string {
	if runtime.GOOS == "windows" {
		return detectDNSWindows()
	}
	return detectDNSResolvConf()
}

func detectDNSResolvConf() []string {
	b, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil
	}
	var dns []string
	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "nameserver" {
			dns = append(dns, fields[1])
		}
	}
	return dns
}

func detectDNSWindows() []string {
	if dns := detectDNSWindowsPS(); len(dns) > 0 {
		return dns
	}
	// PowerShell/WMI 不可用时回退到 ipconfig
	return detectDNSWindowsIpconfig()
}

func detectDNSWindowsPS() []string {
	out := run("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-DnsClientServerAddress -AddressFamily IPv4 | Where-Object {$_.ServerAddresses} | ForEach-Object {$_.ServerAddresses} | Sort-Object -Unique")
	var dns []string
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if ip := net.ParseIP(line); ip != nil {
			dns = append(dns, line)
		}
	}
	return dns
}

// detectDNSWindowsIpconfig 解析 `ipconfig /all` 的 DNS 服务器段（与系统语言无关）。
func detectDNSWindowsIpconfig() []string {
	out := run("ipconfig", "/all")
	lines := strings.Split(out, "\n")
	var dns []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !strings.Contains(strings.ToUpper(line), "DNS") || !strings.Contains(line, ":") {
			continue
		}
		idx := strings.Index(line, ":")
		rest := strings.TrimSpace(line[idx+1:])

		var ips []string
		if ip := net.ParseIP(rest); ip != nil {
			ips = append(ips, rest)
		} else if rest != "" {
			continue // 如 "Connection-specific DNS Suffix"，跳过
		}
		// 续行收集纯 IP（可能有多条 DNS 服务器，缩进排列）
		for j := i + 1; j < len(lines); j++ {
			t := strings.TrimSpace(lines[j])
			if ip := net.ParseIP(t); ip != nil {
				ips = append(ips, t)
				continue
			}
			break
		}
		dns = append(dns, ips...)
	}
	return uniqueStrings(dns)
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
