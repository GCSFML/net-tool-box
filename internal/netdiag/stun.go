package netdiag

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

const stunMagicCookie uint32 = 0x2112A442

// 依次尝试，覆盖国内与国际 STUN 服务器。
var stunServers = []string{
	"stun.miwifi.com:3478",
	"stun.qq.com:3478",
	"stun.l.google.com:19302",
	"stun.cloudflare.com:3478",
}

// STUNResult 记录 STUN 检测到的 NAT 外部出口地址。
type STUNResult struct {
	Server    string `json:"server"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	LatencyMs int64  `json:"latencyMs"`
}

// GetSTUN 依次尝试多个 STUN 服务器，返回第一个成功的 NAT 外部出口地址。
// 使用 RFC 5389 的 Binding Request / XOR-MAPPED-ADDRESS。
func GetSTUN(timeout time.Duration) (*STUNResult, error) {
	var lastErr error
	for _, server := range stunServers {
		res, err := querySTUN(server, timeout)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("所有 STUN 服务器均不可用")
}

func querySTUN(server string, timeout time.Duration) (*STUNResult, error) {
	start := time.Now()
	conn, err := net.DialTimeout("udp", server, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	txID := make([]byte, 12)
	if _, err := rand.Read(txID); err != nil {
		return nil, err
	}

	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], 0x0001) // Binding Request
	binary.BigEndian.PutUint16(msg[2:4], 0x0000)
	binary.BigEndian.PutUint32(msg[4:8], stunMagicCookie)
	copy(msg[8:20], txID)

	if _, err := conn.Write(msg); err != nil {
		return nil, err
	}
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}
	buf = buf[:n]

	if n < 20 {
		return nil, fmt.Errorf("STUN 响应过短")
	}
	if msgType := binary.BigEndian.Uint16(buf[0:2]); msgType != 0x0101 { // Binding Success Response
		return nil, fmt.Errorf("STUN 响应类型异常 0x%04x", msgType)
	}
	if !bytes.Equal(buf[8:20], txID) {
		return nil, fmt.Errorf("STUN 事务 ID 不匹配")
	}

	ip, port, err := parseXORMappedAddress(buf[20:], txID)
	if err != nil {
		ip, port, err = parseMappedAddress(buf[20:])
		if err != nil {
			return nil, err
		}
	}

	return &STUNResult{
		Server:    server,
		IP:        ip.String(),
		Port:      port,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

func parseXORMappedAddress(attrs, txID []byte) (net.IP, int, error) {
	for len(attrs) >= 4 {
		attrType := binary.BigEndian.Uint16(attrs[0:2])
		attrLen := int(binary.BigEndian.Uint16(attrs[2:4]))
		if 4+attrLen > len(attrs) {
			break
		}
		val := attrs[4 : 4+attrLen]

		if attrType == 0x0020 && len(val) >= 4 { // XOR-MAPPED-ADDRESS
			family := val[1]
			port := int(binary.BigEndian.Uint16(val[2:4]) ^ uint16(stunMagicCookie>>16))
			if family == 0x01 && len(val) >= 8 {
				x := binary.BigEndian.Uint32(val[4:8]) ^ stunMagicCookie
				return net.IPv4(byte(x>>24), byte(x>>16), byte(x>>8), byte(x)), port, nil
			}
			if family == 0x02 && len(val) >= 20 {
				// IPv6：XOR 密钥 = magicCookie(4) + txID(12)
				key := make([]byte, 16)
				binary.BigEndian.PutUint32(key[0:4], stunMagicCookie)
				copy(key[4:16], txID[:12])
				ip := make(net.IP, 16)
				for i := 0; i < 16; i++ {
					ip[i] = val[4+i] ^ key[i]
				}
				return ip, port, nil
			}
		}

		adv := 4 + attrLen
		if adv%4 != 0 {
			adv += 4 - adv%4
		}
		attrs = attrs[adv:]
	}
	return nil, 0, fmt.Errorf("未找到 XOR-MAPPED-ADDRESS")
}

func parseMappedAddress(attrs []byte) (net.IP, int, error) {
	for len(attrs) >= 4 {
		attrType := binary.BigEndian.Uint16(attrs[0:2])
		attrLen := int(binary.BigEndian.Uint16(attrs[2:4]))
		if 4+attrLen > len(attrs) {
			break
		}
		val := attrs[4 : 4+attrLen]
		if attrType == 0x0001 && len(val) >= 8 { // MAPPED-ADDRESS
			if val[1] == 0x01 {
				port := int(binary.BigEndian.Uint16(val[2:4]))
				return net.IPv4(val[4], val[5], val[6], val[7]), port, nil
			}
		}
		adv := 4 + attrLen
		if adv%4 != 0 {
			adv += 4 - adv%4
		}
		attrs = attrs[adv:]
	}
	return nil, 0, fmt.Errorf("未找到 MAPPED-ADDRESS")
}
