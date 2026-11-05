// Package main 是 Net Tool Box 的「命令行轻图形化版」（TUI）。
// 纯 Go 标准库实现：ANSI 彩色 + 框线菜单，跨 Windows / Linux / macOS，
// 编译为单个静态可执行文件，免安装、可放 U 盘即插即用。
package main

import (
	"fmt"
	"strings"
)

func main() {
	useColor = enableANSI()

	for {
		clearScreen()
		showBanner()
		showMenu()

		choice, ok := prompt(c("  请输入选项 [0-6]: ", cyan))
		if !ok {
			fmt.Println("\n已退出。")
			return
		}

		clearScreen()
		switch strings.ToLower(choice) {
		case "1":
			viewLocalIP()
		case "2":
			viewPublicIP()
		case "3":
			viewNAT()
		case "4":
			viewTrace()
		case "5":
			viewPort()
		case "6":
			viewAll()
		case "0", "q", "quit", "exit":
			fmt.Println(c("\n再见，感谢使用 Net Tool Box。", cyan))
			return
		default:
			fmt.Println(c("无效选项，请输入 0-6。", red))
		}

		fmt.Println()
		if !pause() {
			return
		}
	}
}

func showBanner() {
	fmt.Println(c(boxTop(), blue))
	fmt.Println(c(boxCenter("Net Tool Box · 网络工具箱"), blue+bold))
	fmt.Println(c(boxCenter("命令行轻图形化版 (TUI) · 免安装单文件 · 跨平台"), blue))
	fmt.Println(c(boxBottom(), blue))
	fmt.Println()
}

func showMenu() {
	fmt.Println(c(boxTop(), blue))
	fmt.Println(c(boxLine(""), blue))
	fmt.Println(c(boxLine("  1) 本地 IP 信息      网卡 / IPv4 / IPv6 / 网关 / DNS"), blue))
	fmt.Println(c(boxLine("  2) 公网 IP 查询      多源查询公网 IPv4 / IPv6"), blue))
	fmt.Println(c(boxLine("  3) NAT / CGNAT 检测  是否公网 / CGNAT / 多层 NAT"), blue))
	fmt.Println(c(boxLine("  4) 网络链路追踪      traceroute 逐跳可视化"), blue))
	fmt.Println(c(boxLine("  5) 端口查询          本地占用 / 监听列表 / 远端连通"), blue))
	fmt.Println(c(boxLine("  6) 一键全部检测      依次执行以上诊断"), blue))
	fmt.Println(c(boxLine("  0) 退出"), blue))
	fmt.Println(c(boxLine(""), blue))
	fmt.Println(c(boxBottom(), blue))
	fmt.Println()
}

func pause() bool {
	_, ok := prompt(c("\n  按回车返回主菜单…", gray))
	return ok
}
