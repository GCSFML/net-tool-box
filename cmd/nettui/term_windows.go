//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	enableVirtualTerminalProcessing = 0x0004
	cpUTF8                          = 65001
	stdOutputHandle                 = 0xFFFFFFF5 // -11 (DWORD)
)

var (
	kernel32               = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle       = kernel32.NewProc("GetStdHandle")
	procGetConsoleMode     = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode     = kernel32.NewProc("SetConsoleMode")
	procSetConsoleOutputCP = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleCP       = kernel32.NewProc("SetConsoleCP")
)

// enableANSI 在 Windows 控制台上启用 UTF-8 输出与 ANSI 转义序列。
// 返回 true 表示成功（可放心输出颜色/框线）；失败时降级为纯文本输出。
func enableANSI() bool {
	// 将控制台输入/输出代码页切到 UTF-8（避免中文乱码）
	procSetConsoleOutputCP.Call(uintptr(cpUTF8))
	procSetConsoleCP.Call(uintptr(cpUTF8))

	h, _, _ := procGetStdHandle.Call(uintptr(stdOutputHandle))
	if h == 0 || h == ^uintptr(0) {
		return false // 无控制台（输出被重定向）
	}
	var mode uint32
	r, _, _ := procGetConsoleMode.Call(h, uintptr(unsafe.Pointer(&mode)))
	if r == 0 {
		return false
	}
	mode |= enableVirtualTerminalProcessing
	r, _, _ = procSetConsoleMode.Call(h, uintptr(mode))
	return r != 0
}
