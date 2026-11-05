# Net Tool Box · 网络工具箱

一个**图形化、免安装、纯 Go 实现**的网络诊断工具箱，面向普通家庭/办公宽带用户，一键搞清楚：

> 我是谁、我在哪、我有没有公网 IP、我卡在几层 NAT、我的端口通不通。

提供两种形态，均为**单个免安装可执行文件**：

- **网页图形版** — 启动本地服务并自动打开浏览器，界面美观
- **命令行 TUI 版** — 终端里的轻图形化界面（彩框菜单），无需浏览器，可放 U 盘即插即用

## ✨ 功能

- 📍 **本地 IP 查询** — 枚举所有网卡、IPv4/IPv6 地址、MAC、默认网关、DNS
- 🌐 **公网 IP 查询** — 多源并发查询公网 IPv4 与 IPv6（国内 + 国际线路）
- 🔀 **一键 CGNAT 检测** — 判断是否处于运营商级 NAT（`100.64.0.0/10`）
- 🔀 **一键多层 NAT 检测** — 判断是否光猫拨号 + 路由器二级路由等多层转发
- 🔗 **本地网络链路追踪** — traceroute 一键列出「本机 → 公网」整条链路，逐跳标注内网/私网/CGNAT/公网
- 🔌 **端口查询** — 本地端口占用检测、本机监听端口列表、远端端口连通性检测

## 🚀 使用（免安装）

1. 到 [Releases](releases) 下载对应平台的可执行程序
2. **网页图形版**：Windows 双击 `nettoolbox.exe`，Linux/macOS 运行 `./nettoolbox`，程序会自动打开浏览器
3. **命令行 TUI 版**：在终端运行 `nettoolbox-cli`（Windows 为 `nettoolbox-cli.exe`），按菜单数字选择功能

> 无需安装任何运行时，解压即用（绿色版 / 单文件）。TUI 版不依赖浏览器，可放在 U 盘中跨机器使用。

## 🛠 构建

需要本机安装 [Go](https://go.dev/dl/) 1.22 或更高版本。项目**零第三方依赖**，只需标准库。

```bash
# Windows（产出 dist\nettoolbox.exe 与 dist\nettoolbox-cli.exe）
build.bat

# Linux / macOS（产出 dist/nettoolbox 与 dist/nettoolbox-cli）
./build.sh

# 或分别构建
go build -trimpath -ldflags "-s -w" -o nettoolbox .              # 网页版
go build -trimpath -ldflags "-s -w" -o nettoolbox-cli ./cmd/nettui # 命令行版
```

## 🧭 工作原理

| 功能 | 实现方式 |
|------|----------|
| 本地 IP | Go 标准库 `net.Interfaces()` 枚举网卡 |
| 默认网关 | 解析系统路由表（`route print` / `ip route` / `/proc/net/route`） |
| 公网 IP | 并发请求多个公网 IP 服务，取多数一致结果 |
| CGNAT / 多层 NAT | STUN 获取外部出口 + UPnP 读取路由器 WAN 口 IP，综合判定 |
| 链路追踪 | 调用系统 `tracert`/`traceroute`，解析逐跳 IP 并分类 |
| 端口查询 | 本地绑定检测 + `netstat` 解析 + TCP 连接探测 |

## 🏗 技术栈

- **语言**：Go（标准库，无第三方依赖）
- **网页图形版**：内嵌网页 UI（HTML/CSS/JS），程序启动本地 HTTP 服务并自动打开浏览器
- **命令行 TUI 版**：纯标准库 ANSI 彩色 + 框线菜单；Windows 自动开启 VT 转义与 UTF-8 代码页，Linux/macOS 原生支持
- **跨平台**：Windows / Linux / macOS

## 📂 目录结构

```
.
├── main.go                  # 网页版入口：启动本地服务 + 打开浏览器
├── cmd/
│   └── nettui/              # 命令行 TUI 版入口（菜单界面）
├── internal/
│   ├── netdiag/             # 诊断核心（本地IP/公网IP/STUN/UPnP/NAT/链路/端口）—— 两个版本共用
│   └── server/              # HTTP 服务与 API 路由
├── web/                     # 网页 UI（内嵌）
├── build.bat / build.sh     # 构建脚本（一次构建两个版本）
└── .github/workflows/       # GitHub Actions 自动构建与发布
```

## 📄 License

[MIT](LICENSE)
