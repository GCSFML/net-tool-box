package netdiag

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// PublicIPEntry 单个公网 IP 查询源的结果。
type PublicIPEntry struct {
	Source    string `json:"source"`
	IP        string `json:"ip"`
	Version   string `json:"version"`
	LatencyMs int64  `json:"latencyMs"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

// PublicIPResult 公网 IP 查询结果汇总。
type PublicIPResult struct {
	Entries []PublicIPEntry `json:"entries"`
	IPv4    string          `json:"ipv4"`
	IPv6    string          `json:"ipv6"`
}

type ipEndpoint struct {
	name string
	url  string
}

// 多源查询，同时覆盖国内与国际线路，避免单一来源失败。
var ipv4Endpoints = []ipEndpoint{
	{"ipw.cn", "https://4.ipw.cn"},
	{"ipify", "https://api.ipify.org"},
	{"ipip.net", "https://myip.ipip.net"},
	{"3322.net", "https://ip.3322.net"},
	{"ifconfig.me", "https://ifconfig.me/ip"},
	{"icanhazip", "https://ipv4.icanhazip.com"},
}

var ipv6Endpoints = []ipEndpoint{
	{"ipw.cn", "https://6.ipw.cn"},
	{"ipify", "https://api6.ipify.org"},
}

// GetPublicIP 多源并发查询公网 IPv4 / IPv6，并做多数一致（consensus）汇总。
func GetPublicIP(timeout time.Duration) *PublicIPResult {
	res := &PublicIPResult{Entries: []PublicIPEntry{}}
	var mu sync.Mutex
	var wg sync.WaitGroup

	query := func(ep ipEndpoint) {
		defer wg.Done()
		e := PublicIPEntry{Source: ep.name}
		start := time.Now()
		ip, err := queryEndpoint(ep.url, timeout)
		e.LatencyMs = time.Since(start).Milliseconds()
		if err != nil {
			e.OK = false
			e.Error = err.Error()
		} else if parsed := net.ParseIP(ip); parsed == nil {
			e.OK = false
			e.Error = "无法解析返回的 IP"
		} else {
			e.OK = true
			e.IP = parsed.String()
			if parsed.To4() != nil {
				e.Version = "IPv4"
			} else {
				e.Version = "IPv6"
			}
		}
		mu.Lock()
		res.Entries = append(res.Entries, e)
		mu.Unlock()
	}

	for _, ep := range ipv4Endpoints {
		wg.Add(1)
		go query(ep)
	}
	for _, ep := range ipv6Endpoints {
		wg.Add(1)
		go query(ep)
	}
	wg.Wait()

	sort.Slice(res.Entries, func(i, j int) bool { return res.Entries[i].Source < res.Entries[j].Source })
	res.IPv4 = consensus(res.Entries, "IPv4")
	res.IPv6 = consensus(res.Entries, "IPv6")
	return res
}

var ipv4Pattern = regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)

func queryEndpoint(url string, timeout time.Duration) (string, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	text := strings.TrimSpace(string(body))
	if ip := net.ParseIP(text); ip != nil {
		return text, nil
	}
	// 有些来源返回带文字的响应（如 ipip.net），用正则提取 IP。
	if m := ipv4Pattern.FindString(text); m != "" {
		return m, nil
	}
	return "", fmt.Errorf("响应中未找到 IP")
}

// consensus 返回某版本中出现次数最多的 IP。
func consensus(entries []PublicIPEntry, version string) string {
	counts := map[string]int{}
	var best string
	for _, e := range entries {
		if e.OK && e.Version == version {
			counts[e.IP]++
			if counts[e.IP] > counts[best] {
				best = e.IP
			}
		}
	}
	return best
}
