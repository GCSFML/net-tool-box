package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GCSFML/net-tool-box/internal/netdiag"
)

// viewLocalIP 展示本机网卡、IP、网关、DNS。
func viewLocalIP() {
	printHeader("本地 IP 信息")
	res, err := netdiag.GetLocalIP()
	if err != nil {
		printError("获取失败：" + err.Error())
		return
	}

	shown := 0
	for _, iface := range res.Interfaces {
		if !iface.Up || iface.Loopback {
			continue
		}
		var addrs []netdiag.AddrInfo
		for _, a := range iface.Addrs {
			if a.IsLoopback || a.IsLinkLocal {
				continue
			}
			addrs = append(addrs, a)
		}
		if len(addrs) == 0 {
			continue
		}
		shown++

		title := iface.Name
		if iface.MAC != "" {
			title += "  (" + iface.MAC + ")"
		}
		printSubHeader(title)
		for _, a := range addrs {
			tag, tagCol := "[公网]", yellow
			if a.IsPrivate {
				tag, tagCol = "[私网]", cyan
			}
			fmt.Printf("    %s  %s  %s %s\n",
				c(fmt.Sprintf("%-4s", a.Version), gray),
				c(a.IP, green+bold),
				c(tag, tagCol),
				c(a.CIDR, gray))
		}
	}
	if shown == 0 {
		printWarn("未检测到有效的网络接口")
	}

	fmt.Println()
	printSubHeader("网络配置")
	printKV("默认网关", orNone(res.Gateway))
	printKV("DNS", joinOrNone(res.DNS))
}

// viewPublicIP 多源查询公网 IPv4 / IPv6。
func viewPublicIP() {
	printHeader("公网 IP 查询")
	fmt.Println(c("  正在并发查询多个来源…", gray))

	res := netdiag.GetPublicIP(6 * time.Second)

	fmt.Println()
	if res.IPv4 != "" {
		printKV("公网 IPv4", c(res.IPv4, green+bold))
	} else {
		printKV("公网 IPv4", c("未检测到", red))
	}
	if res.IPv6 != "" {
		printKV("公网 IPv6", c(res.IPv6, green+bold))
	} else {
		printKV("公网 IPv6", c("未检测到（可能未启用 IPv6）", gray))
	}

	fmt.Println()
	printSubHeader("各来源明细")
	for _, e := range res.Entries {
		if e.OK {
			fmt.Printf("    %s  %s  %s\n",
				c(padRight(e.Source, 14), white),
				c(padRight(e.IP, 40), green),
				c(fmt.Sprintf("%dms", e.LatencyMs), gray))
		} else {
			fmt.Printf("    %s  %s  %s\n",
				c(padRight(e.Source, 14), white),
				c(padRight("失败", 40), red),
				c(e.Error, gray))
		}
	}
}

// viewNAT 检测公网 IP / CGNAT / 多层 NAT。
func viewNAT() {
	printHeader("NAT / CGNAT 检测")
	fmt.Println(c("  正在检测（STUN / UPnP / 公网 IP），约需数秒…", gray))

	res := netdiag.CheckNAT()

	fmt.Println()
	col := cyan
	switch {
	case res.HasPublicIP:
		col = green
	case res.IsCGNAT || res.MultiNAT:
		col = red
	case res.IPv6Available:
		col = yellow
	}
	fmt.Println(c("  ★ "+res.Summary, col+bold))

	fmt.Println()
	printSubHeader("检测结果")
	printKV("本机 IP", orNone(res.LocalIP))
	printKV("默认网关", orNone(res.Gateway))
	printKV("公网 IPv4", orNone(res.PublicIPv4))
	if res.IPv6Available {
		printKV("公网 IPv6", c(res.IPv6, green))
	}
	if res.STUN != nil {
		printKV("STUN 出口", fmt.Sprintf("%s:%d", res.STUN.IP, res.STUN.Port))
	}
	if res.UPnP != nil && res.UPnP.Available {
		printKV("路由器 WAN", orNone(res.RouterWANIP))
	} else {
		printKV("路由器 WAN", c("UPnP 不可用（可登录路由器后台核对）", gray))
	}
	printKV("NAT 层数", strconv.Itoa(res.NATLayers))

	fmt.Println()
	printSubHeader("判定明细")
	for _, d := range res.Details {
		fmt.Println("    " + c("·", cyan) + " " + d)
	}
}

// viewTrace 追踪本机到公网的完整链路。
func viewTrace() {
	printHeader("本地网络链路追踪")
	fmt.Println(c("  正在追踪路由（最长约 30 秒，请稍候）…", gray))

	res := netdiag.TraceNetwork()

	fmt.Println()
	col := cyan
	switch {
	case res.CGNATDetected:
		col = red
	case res.NATLayers >= 2:
		col = yellow
	case res.NATLayers == 1:
		col = green
	}
	fmt.Println(c("  ★ "+res.Summary, col+bold))
	fmt.Printf("  目标 %s · 本机 %s · 网关 %s\n\n",
		c(res.Target, cyan), orNone(res.SourceIP), orNone(res.Gateway))

	for i, h := range res.Hops {
		label, labelCol := "无响应", gray
		switch h.Type {
		case netdiag.HopLocal:
			label, labelCol = "本机", cyan
		case netdiag.HopPrivate:
			label, labelCol = "内网跳", yellow
		case netdiag.HopCGNAT:
			label, labelCol = "CGNAT", red
		case netdiag.HopPublic:
			label, labelCol = "公网出口", green
		}

		line := fmt.Sprintf("  #%-2d  %s", h.Hop, c(padRight(label, 14), labelCol))
		if h.IP != "" {
			line += c(h.IP, green+bold)
		}
		if h.IsGateway {
			line += " " + c("[网关]", cyan)
		}
		fmt.Println(line)

		if i < len(res.Hops)-1 {
			fmt.Println(c("        |", gray))
		}
	}
}

// viewPort 端口查询子菜单。
func viewPort() {
	printHeader("端口查询")
	fmt.Println("  1) 检查本地端口占用")
	fmt.Println("  2) 列出本机监听端口")
	fmt.Println("  3) 检测远端端口连通性")
	fmt.Println()

	choice, ok := prompt(c("  请选择 [1-3]（回车返回）: ", cyan))
	if !ok || choice == "" {
		return
	}
	fmt.Println()

	switch choice {
	case "1":
		port := promptInt(c("  请输入本地端口: ", cyan))
		if port < 1 || port > 65535 {
			printError("端口号无效（应为 1-65535）")
			return
		}
		res := netdiag.CheckLocalPort(port)
		if res.InUse {
			printWarn(fmt.Sprintf("端口 %d：%s", res.Port, res.Message))
		} else {
			printOK(fmt.Sprintf("端口 %d：%s", res.Port, res.Message))
		}

	case "2":
		listAndPrintPorts()

	case "3":
		line, ok := prompt(c("  请输入 主机:端口（如 example.com:443）: ", cyan))
		if !ok {
			return
		}
		host, port, valid := splitHostPort(line)
		if !valid {
			printError("格式无效，请使用 主机:端口 的形式")
			return
		}
		fmt.Println(c("  正在连接…", gray))
		res := netdiag.CheckRemotePort(host, port, 6*time.Second)
		if res.Open {
			printOK(fmt.Sprintf("%s:%d 已开放（%d ms）", res.Host, res.Port, res.LatencyMs))
		} else {
			printError(fmt.Sprintf("%s:%d 不可达：%s", res.Host, res.Port, res.Error))
		}

	default:
		printError("无效选择")
	}
}

func listAndPrintPorts() {
	fmt.Println(c("  正在读取本机监听端口…", gray))
	ports := netdiag.ListListeningPorts()
	if len(ports) == 0 {
		printWarn("未获取到监听端口（可能受系统权限限制）")
		return
	}
	printSubHeader(fmt.Sprintf("共 %d 条监听端口", len(ports)))
	fmt.Println("    " + padRight("协议", 6) + padRight("端口", 8) + padRight("地址", 24) + "PID")
	for _, p := range ports {
		fmt.Printf("    %-6s%-8d%-24s%s\n", p.Proto, p.Port, p.Addr, p.PID)
	}
}

// viewAll 依次执行全部诊断。
func viewAll() {
	printHeader("一键全部检测")
	fmt.Println(c("  依次执行：本地 IP → 公网 IP → NAT/CGNAT → 链路追踪", gray))
	fmt.Println(c("  链路追踪最长约 30 秒，请耐心等待…", gray))
	fmt.Println()

	viewLocalIP()
	sep()
	viewPublicIP()
	sep()
	viewNAT()
	sep()
	viewTrace()
}

// splitHostPort 解析 "host:port" 形式。
func splitHostPort(s string) (string, int, bool) {
	s = strings.TrimSpace(s)
	idx := strings.LastIndex(s, ":")
	if idx <= 0 || idx == len(s)-1 {
		return "", 0, false
	}
	host := strings.TrimSpace(s[:idx])
	port, err := strconv.Atoi(strings.TrimSpace(s[idx+1:]))
	if err != nil || port < 1 || port > 65535 {
		return "", 0, false
	}
	return host, port, true
}
