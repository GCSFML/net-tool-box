package netdiag

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type upnpService struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

type upnpDevice struct {
	Services []upnpService `xml:"serviceList>service"`
	Devices  []upnpDevice  `xml:"deviceList>device"`
}

type upnpRoot struct {
	Device upnpDevice `xml:"device"`
}

// UPnPResult UPnP 读取路由器 WAN 口 IP 的结果。
type UPnPResult struct {
	Available bool   `json:"available"`
	WANIP     string `json:"wanIP"`
	Service   string `json:"service"`
}

// GetRouterWANIP 通过 UPnP 读取路由器 WAN 口 IP，用于 CGNAT / 多层 NAT 判定。
// 失败（路由器关闭 UPnP 等）时 Available 为 false，不报错。
func GetRouterWANIP(timeout time.Duration) *UPnPResult {
	res := &UPnPResult{}
	controlURL, serviceURN, err := findIGDControlURL(timeout)
	if err != nil {
		return res
	}
	ip, err := getExternalIPViaUPnP(controlURL, serviceURN, timeout)
	if err != nil {
		return res
	}
	res.Available = true
	res.WANIP = ip
	res.Service = serviceURN
	return res
}

func findIGDControlURL(timeout time.Duration) (string, string, error) {
	locations := ssdpDiscover("urn:schemas-upnp-org:device:InternetGatewayDevice:1", timeout)
	for _, loc := range locations {
		cu, urn, err := parseIGDControlURL(loc, timeout)
		if err == nil {
			return cu, urn, nil
		}
	}
	return "", "", fmt.Errorf("未发现 UPnP 网关设备")
}

// ssdpDiscover 发送 SSDP M-SEARCH 多播请求并收集 LOCATION 响应。
func ssdpDiscover(st string, timeout time.Duration) []string {
	var locations []string
	pc, err := net.ListenPacket("udp4", "0.0.0.0:0")
	if err != nil {
		return locations
	}
	defer pc.Close()

	msg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: " + st + "\r\n\r\n"

	dst, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return locations
	}
	_, _ = pc.WriteTo([]byte(msg), dst)

	_ = pc.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 8192)
	seen := map[string]bool{}
	for {
		n, _, err := pc.ReadFrom(buf)
		if err != nil {
			break
		}
		if loc := parseHeader(string(buf[:n]), "LOCATION"); loc != "" && !seen[loc] {
			seen[loc] = true
			locations = append(locations, loc)
		}
	}
	return locations
}

func parseHeader(resp, key string) string {
	key = strings.ToLower(key)
	for _, line := range strings.Split(resp, "\r\n") {
		if strings.HasPrefix(strings.ToLower(line), key+":") {
			return strings.TrimSpace(line[len(key)+1:])
		}
	}
	return ""
}

// parseIGDControlURL 获取设备描述 XML 并定位 WANIPConnection/WANPPPConnection 的 controlURL。
func parseIGDControlURL(loc string, timeout time.Duration) (string, string, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(loc)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", err
	}
	var root upnpRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		return "", "", err
	}

	var find func(d upnpDevice) (string, string, bool)
	find = func(d upnpDevice) (string, string, bool) {
		for _, s := range d.Services {
			if strings.Contains(s.ServiceType, "WANIPConnection") || strings.Contains(s.ServiceType, "WANPPPConnection") {
				return resolveURL(loc, s.ControlURL), s.ServiceType, true
			}
		}
		for _, sub := range d.Devices {
			if cu, urn, ok := find(sub); ok {
				return cu, urn, true
			}
		}
		return "", "", false
	}
	if cu, urn, ok := find(root.Device); ok {
		return cu, urn, nil
	}
	return "", "", fmt.Errorf("设备描述中未找到 WAN 连接服务")
}

func resolveURL(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	u, err := url.Parse(base)
	if err != nil {
		return ref
	}
	u.Path = ref
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

var wanIPPattern = regexp.MustCompile(`<NewExternalIPAddress[^>]*>([^<]*)</NewExternalIPAddress>`)

func getExternalIPViaUPnP(controlURL, serviceURN string, timeout time.Duration) (string, error) {
	ns := "urn:schemas-upnp-org:service:WANIPConnection:1"
	if strings.Contains(serviceURN, "WANPPPConnection") {
		ns = "urn:schemas-upnp-org:service:WANPPPConnection:1"
	}
	body := `<?xml version="1.0"?>` +
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">` +
		`<s:Body><u:GetExternalIPAddress xmlns:u="` + ns + `"></u:GetExternalIPAddress></s:Body>` +
		`</s:Envelope>`

	req, err := http.NewRequest("POST", controlURL, bytes.NewReader([]byte(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", `"`+ns+`#GetExternalIPAddress"`)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if m := wanIPPattern.FindSubmatch(respBody); len(m) == 2 {
		return strings.TrimSpace(string(m[1])), nil
	}
	return "", fmt.Errorf("未解析到外部 IP")
}
