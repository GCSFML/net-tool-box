// Package server 提供内嵌网页 UI 的 HTTP 服务与 API 路由。
package server

import (
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/GCSFML/net-tool-box/internal/netdiag"
)

// NewMux 构建 HTTP 路由。
func NewMux(assets fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/localip", handleLocalIP)
	mux.HandleFunc("/api/publicip", handlePublicIP)
	mux.HandleFunc("/api/nat", handleNAT)
	mux.HandleFunc("/api/trace", handleTrace)
	mux.HandleFunc("/api/port/local", handleLocalPort)
	mux.HandleFunc("/api/port/list", handleListPorts)
	mux.HandleFunc("/api/port/remote", handleRemotePort)

	mux.HandleFunc("/", serveStatic(assets))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleLocalIP(w http.ResponseWriter, _ *http.Request) {
	res, err := netdiag.GetLocalIP()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}

func handlePublicIP(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, netdiag.GetPublicIP(6*time.Second))
}

func handleNAT(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, netdiag.CheckNAT())
}

func handleTrace(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, netdiag.TraceNetwork())
}

func handleLocalPort(w http.ResponseWriter, r *http.Request) {
	port, err := strconv.Atoi(r.URL.Query().Get("port"))
	if err != nil || port < 1 || port > 65535 {
		writeJSON(w, 400, map[string]any{"error": "端口参数无效，应为 1-65535"})
		return
	}
	writeJSON(w, 200, netdiag.CheckLocalPort(port))
}

func handleListPorts(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"ports": netdiag.ListListeningPorts()})
}

func handleRemotePort(w http.ResponseWriter, r *http.Request) {
	host := strings.TrimSpace(r.URL.Query().Get("host"))
	port, err := strconv.Atoi(r.URL.Query().Get("port"))
	if err != nil || port < 1 || port > 65535 {
		writeJSON(w, 400, map[string]any{"error": "端口参数无效，应为 1-65535"})
		return
	}
	if host == "" {
		writeJSON(w, 400, map[string]any{"error": "缺少 host 参数"})
		return
	}
	writeJSON(w, 200, netdiag.CheckRemotePort(host, port, 6*time.Second))
}

// serveStatic 从内嵌 FS 提供静态资源，并做路径清理防止穿越。
func serveStatic(assets fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		p = path.Clean(p)
		if p == "." || strings.HasPrefix(p, "..") {
			http.NotFound(w, r)
			return
		}
		b, err := fs.ReadFile(assets, p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ct := mime.TypeByExtension(path.Ext(p))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		_, _ = w.Write(b)
	}
}
