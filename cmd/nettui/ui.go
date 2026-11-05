package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ---- ANSI 颜色 ----
const (
	reset   = "\x1b[0m"
	bold    = "\x1b[1m"
	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
	white   = "\x1b[97m"
	gray    = "\x1b[90m"
)

var (
	useColor = true
	stdin    = bufio.NewReader(os.Stdin)
)

// 固定内框宽度（显示列数）
const boxW = 64

func c(s, code string) string {
	if !useColor {
		return s
	}
	return code + s + reset
}

// ---- 中英文混排的显示宽度计算（中文/全角按 2 列） ----

func runeWidth(r rune) int {
	if r < 32 {
		return 0
	}
	switch {
	case r >= 0x1100 && r <= 0x115F,
		r == 0x2329, r == 0x232A,
		r >= 0x2E80 && r <= 0xA4CF && r != 0x303F,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE6F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6,
		r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		w += runeWidth(r)
	}
	return w
}

func truncateTo(s string, w int) string {
	if displayWidth(s) <= w {
		return s
	}
	var b strings.Builder
	cw := 0
	for _, r := range s {
		rw := runeWidth(r)
		if cw+rw > w {
			break
		}
		b.WriteRune(r)
		cw += rw
	}
	return b.String()
}

func padRight(s string, w int) string {
	s = truncateTo(s, w)
	return s + strings.Repeat(" ", w-displayWidth(s))
}

// ---- 框线绘制 ----

func boxTop() string    { return "╔" + strings.Repeat("═", boxW) + "╗" }
func boxBottom() string { return "╚" + strings.Repeat("═", boxW) + "╝" }
func boxLine(s string) string {
	return "║ " + padRight(s, boxW-1) + "║"
}
func boxCenter(s string) string {
	s = truncateTo(s, boxW)
	dw := displayWidth(s)
	left := (boxW - dw) / 2
	right := boxW - dw - left
	return "║" + strings.Repeat(" ", left) + s + strings.Repeat(" ", right) + "║"
}

func clearScreen() {
	if useColor {
		fmt.Print("\x1b[2J\x1b[H")
	}
}

// ---- 文本输出辅助 ----

func printHeader(title string) {
	fmt.Println()
	fmt.Println(c("═══ "+title+" ═══", cyan+bold))
	fmt.Println()
}

func printSubHeader(s string) {
	fmt.Println(c("── "+s, bold))
}

func printKV(k, v string) {
	fmt.Printf("  %s %s\n", c(padRight(k, 12), gray), v)
}

func printError(s string) { fmt.Println(c("  ✗ "+s, red)) }
func printOK(s string)    { fmt.Println(c("  ✓ "+s, green)) }
func printWarn(s string)  { fmt.Println(c("  ! "+s, yellow)) }

func sep() {
	fmt.Println()
	fmt.Println(c("  "+strings.Repeat("─", 58), gray))
	fmt.Println()
}

// ---- 输入辅助 ----

// prompt 输出提示并读取一行输入。返回 (输入, 是否未遇到 EOF)。
func prompt(label string) (string, bool) {
	fmt.Print(label)
	line, err := stdin.ReadString('\n')
	line = strings.TrimSpace(line)
	if err != nil && line == "" {
		return "", false // stdin 结束
	}
	return line, true
}

func promptInt(label string) int {
	s, ok := prompt(label)
	if !ok {
		return 0
	}
	n := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0
		}
		n = n*10 + int(ch-'0')
	}
	return n
}

func orNone(s string) string {
	if s == "" {
		return c("未检测到", gray)
	}
	return c(s, green)
}

func joinOrNone(list []string) string {
	if len(list) == 0 {
		return c("未检测到", gray)
	}
	return c(strings.Join(list, "，"), green)
}
