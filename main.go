package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os/exec"
	"runtime"

	"github.com/GCSFML/net-tool-box/internal/server"
)

//go:embed all:web
var webFS embed.FS

func main() {
	portFlag := flag.Int("port", 0, "HTTP 服务端口，0 表示自动选择空闲端口")
	noBrowser := flag.Bool("no-browser", false, "启动后不自动打开浏览器")
	flag.Parse()

	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("加载内嵌网页资源失败: %v", err)
	}

	addr := "127.0.0.1:0"
	if *portFlag > 0 {
		addr = fmt.Sprintf("127.0.0.1:%d", *portFlag)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("无法监听端口 %s: %v", addr, err)
	}

	url := "http://" + ln.Addr().String() + "/"
	go func() {
		_ = http.Serve(ln, server.NewMux(webSub))
	}()

	fmt.Printf("Net Tool Box 已启动：%s\n", url)
	fmt.Println("按 Ctrl+C 退出。")

	if !*noBrowser {
		openBrowser(url)
	}

	select {}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("自动打开浏览器失败，请手动访问：%s", url)
	}
}
