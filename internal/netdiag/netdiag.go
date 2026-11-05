// Package netdiag 提供网络诊断的核心能力：
// 本地 IP 枚举、公网 IP 查询、STUN、UPnP、CGNAT/多层 NAT 判定、端口查询。
// 全部基于 Go 标准库实现，无第三方依赖。
package netdiag

import (
	"context"
	"net"
	"os/exec"
	"time"
)

// run 执行外部命令并返回 stdout（失败返回空串），用于读取系统网络信息。
func run(name string, args ...string) string {
	return runTimeout(6*time.Second, name, args...)
}

// runTimeout 执行外部命令，超时后终止进程并返回 stdout（失败返回空串）。
func runTimeout(timeout time.Duration, name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	out, _ := cmd.Output()
	return string(out)
}

// --- IPv4 地址分类辅助 ---

// isPrivateIPv4 判断是否为 RFC1918 私网地址（10/8、172.16/12、192.168/16）。
func isPrivateIPv4(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 10 {
			return true
		}
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
	}
	return false
}

// isCGNATIPv4 判断是否为运营商级 NAT（CGNAT）地址段 100.64.0.0/10。
func isCGNATIPv4(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
	}
	return false
}

// isLinkLocalIPv4 判断是否为链路本地地址 169.254.0.0/16。
func isLinkLocalIPv4(ip net.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 169 && ip4[1] == 254
	}
	return false
}

// classifyIPv4 返回 IPv4 地址的类别：public/private/cgnat/loopback/link-local/invalid。
func classifyIPv4(ip net.IP) string {
	switch {
	case ip == nil || ip.To4() == nil:
		return "invalid"
	case ip.IsLoopback():
		return "loopback"
	case isLinkLocalIPv4(ip):
		return "link-local"
	case isCGNATIPv4(ip):
		return "cgnat"
	case isPrivateIPv4(ip):
		return "private"
	default:
		return "public"
	}
}
