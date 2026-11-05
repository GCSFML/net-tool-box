//go:build !windows

package main

// enableANSI 在 Linux / macOS 上终端原生支持 ANSI 转义序列，直接启用。
func enableANSI() bool {
	return true
}
