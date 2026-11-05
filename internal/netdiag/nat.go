package netdiag

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// NATCheckResult NAT/CGNAT 综合检测结果。
type NATCheckResult struct {
	LocalIP        string      `json:"localIP"`
	LocalIsPrivate bool        `json:"localIsPrivate"`
	Gateway        string      `json:"gateway"`
	PublicIPv4     string      `json:"publicIPv4"`
	IPv6           string      `json:"ipv6"`
	IPv6Available  bool        `json:"ipv6Available"`
	STUN           *STUNResult `json:"stun,omitempty"`
	UPnP           *UPnPResult `json:"upnp,omitempty"`
	RouterWANIP    string      `json:"routerWANIP"`
	RouterWANType  string      `json:"routerWANType"` // public / private / cgnat / unknown
	IsCGNAT        bool        `json:"isCGNAT"`
	MultiNAT       bool        `json:"multiNAT"`
	HasPublicIP    bool        `json:"hasPublicIP"`
	NATLayers      int         `json:"natLayers"`
	Summary        string      `json:"summary"`
	Details        []string    `json:"details"`
}

// CheckNAT 综合检测：本地 IP、公网 IP、STUN、UPnP，判断 CGNAT / 多层 NAT。
func CheckNAT() *NATCheckResult {
	res := &NATCheckResult{Details: []string{}}

	// 本地 IP + 网关（快速，同步）
	res.LocalIP = firstLocalIPv4()
	res.Gateway = detectGateway()
	if res.LocalIP != "" {
		res.LocalIsPrivate = isPrivateIPv4(net.ParseIP(res.LocalIP))
		if res.LocalIsPrivate {
			res.NATLayers = 1
			res.Details = append(res.Details, fmt.Sprintf("本机 IP %s 为私网地址，处于路由器 NAT 之后", res.LocalIP))
		} else {
			res.Details = append(res.Details, fmt.Sprintf("本机 IP %s 为公网地址，设备直连公网", res.LocalIP))
		}
	}

	// 并发执行公网 IP、STUN、UPnP 检测，缩短总耗时
	var pub *PublicIPResult
	var stun *STUNResult
	var upnp *UPnPResult
	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); pub = GetPublicIP(6 * time.Second) }()
	go func() { defer wg.Done(); s, _ := GetSTUN(1500 * time.Millisecond); stun = s }()
	go func() { defer wg.Done(); upnp = GetRouterWANIP(2 * time.Second) }()
	wg.Wait()

	res.PublicIPv4 = pub.IPv4
	res.IPv6 = pub.IPv6
	res.IPv6Available = pub.IPv6 != ""
	res.STUN = stun
	res.UPnP = upnp

	if res.PublicIPv4 != "" {
		res.Details = append(res.Details, "公网出口 IPv4："+res.PublicIPv4)
	}
	if res.IPv6Available {
		res.Details = append(res.Details, "检测到公网 IPv6："+res.IPv6+"（IPv6 通常无 NAT，可直连）")
	}
	if stun != nil {
		res.Details = append(res.Details, fmt.Sprintf("STUN 外部出口 %s:%d（%s）", stun.IP, stun.Port, stun.Server))
	}
	if upnp != nil && upnp.Available {
		res.RouterWANIP = upnp.WANIP
		res.RouterWANType = classifyIPv4(net.ParseIP(upnp.WANIP))
		res.Details = append(res.Details, "路由器 WAN 口 IP："+upnp.WANIP)
	} else {
		res.RouterWANType = "unknown"
		res.Details = append(res.Details, "UPnP 不可用（路由器可能关闭了 UPnP），无法自动读取 WAN 口 IP")
	}

	res.analyze()
	return res
}

func (r *NATCheckResult) analyze() {
	switch r.RouterWANType {
	case "cgnat":
		r.IsCGNAT = true
		r.NATLayers++
		r.Details = append(r.Details, "路由器 WAN 口 IP 属于 100.64.0.0/10（CGNAT 地址段）→ 确认处于运营商级 NAT 之后")
	case "private":
		r.MultiNAT = true
		r.NATLayers++
		r.Details = append(r.Details, "路由器 WAN 口 IP 仍是私网地址 → 存在多层 NAT（如光猫拨号 + 路由器二级路由）")
	case "public":
		r.HasPublicIP = true
		r.Details = append(r.Details, "路由器 WAN 口是公网 IP → 拥有真公网 IPv4（单层 NAT，可做端口映射）")
	case "unknown":
		// UPnP 不可用时的启发式判断
		if r.LocalIsPrivate && r.PublicIPv4 != "" && r.STUN != nil {
			if r.STUN.IP == r.PublicIPv4 {
				if r.NATLayers == 0 {
					r.NATLayers = 1
				}
				r.Details = append(r.Details, "STUN 出口 IP 与公网 IP 一致，推断为单层 NAT（是否 CGNAT 需登录路由器后台查看 WAN 口 IP 确认）")
			} else {
				r.MultiNAT = true
				r.NATLayers = 2
				r.Details = append(r.Details, "STUN 出口 IP 与公网 IP 不一致，可能存在多层 NAT 或代理")
			}
		}
	}

	switch {
	case r.IsCGNAT:
		r.Summary = "检测到 CGNAT（运营商级 NAT），无公网 IPv4"
	case r.MultiNAT:
		r.Summary = "检测到多层 NAT，无公网 IPv4"
	case r.HasPublicIP:
		r.Summary = "拥有公网 IPv4，可做端口映射 / DDNS"
	case r.IPv6Available:
		r.Summary = "无公网 IPv4，但拥有公网 IPv6，可走 IPv6 直连"
	default:
		r.Summary = "无法确定网络类型（建议登录路由器后台核对 WAN 口 IP）"
	}
}

// firstLocalIPv4 返回真正用于上网的本机 IPv4 地址：
// 优先选择子网包含默认网关的接口，避免误选 VMware/VirtualBox/WSL 等虚拟网卡。
func firstLocalIPv4() string {
	gwIP := net.ParseIP(detectGateway())
	ifaces, _ := net.Interfaces()

	var fallback string
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip4 := ipnet.IP.To4()
			if ip4 == nil || ip4.IsLoopback() || isLinkLocalIPv4(ip4) {
				continue
			}
			s := ip4.String()
			if fallback == "" {
				fallback = s
			}
			if gwIP != nil && ipnet.Contains(gwIP) {
				return s
			}
		}
	}
	return fallback
}
