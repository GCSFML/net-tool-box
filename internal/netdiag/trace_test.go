package netdiag

import "testing"

func TestParseTraceWindowsChinese(t *testing.T) {
	out := "通过最多 15 个跃点跟踪到 223.5.5.5 的路由\n\n" +
		"  1    <1 毫秒    <1 毫秒    <1 毫秒  192.168.10.1\n" +
		"  2     2 ms       2 ms       2 ms  100.64.0.1\n" +
		"  3     *          *          *      请求超时。\n" +
		"  4     5 ms       4 ms       5 ms  110.80.1.1\n"

	r := &TraceResult{Target: "223.5.5.5", SourceIP: "192.168.10.5", Gateway: "192.168.10.1", Hops: []HopInfo{}}
	r.Hops = append(r.Hops, HopInfo{Hop: 0, IP: "192.168.10.5", IPs: []string{"192.168.10.5"}, Type: HopLocal})
	r.parseTrace(out)
	r.finalize()

	if len(r.Hops) != 5 {
		t.Fatalf("期望 5 跳（本机+4），实际 %d", len(r.Hops))
	}
	if r.Hops[1].IP != "192.168.10.1" || r.Hops[1].Type != HopPrivate || !r.Hops[1].IsGateway {
		t.Errorf("第 1 跳解析错误: %+v", r.Hops[1])
	}
	if r.Hops[2].IP != "100.64.0.1" || r.Hops[2].Type != HopCGNAT {
		t.Errorf("第 2 跳应为 CGNAT: %+v", r.Hops[2])
	}
	if r.Hops[3].Type != HopTimeout {
		t.Errorf("第 3 跳应为 timeout: %+v", r.Hops[3])
	}
	if r.Hops[4].Type != HopPublic {
		t.Errorf("第 4 跳应为 public: %+v", r.Hops[4])
	}
	if !r.CGNATDetected {
		t.Errorf("应检测到 CGNAT")
	}
	if r.NATLayers != 2 {
		t.Errorf("NAT 层数应为 2，实际 %d", r.NATLayers)
	}
}

func TestParseTraceWindowsEnglish(t *testing.T) {
	out := "Tracing route to 223.5.5.5 over a maximum of 15 hops\n\n" +
		"  1    <1 ms    <1 ms    <1 ms  192.168.1.1\n" +
		"  2     1 ms     1 ms     1 ms  192.168.0.1\n" +
		"  3     3 ms     3 ms     3 ms  110.80.1.1\n"

	r := &TraceResult{Target: "223.5.5.5", SourceIP: "192.168.1.5", Gateway: "192.168.1.1", Hops: []HopInfo{}}
	r.Hops = append(r.Hops, HopInfo{Hop: 0, IP: "192.168.1.5", IPs: []string{"192.168.1.5"}, Type: HopLocal})
	r.parseTrace(out)
	r.finalize()

	if len(r.Hops) != 4 {
		t.Fatalf("期望 4 跳，实际 %d", len(r.Hops))
	}
	if r.Hops[1].Type != HopPrivate || r.Hops[2].Type != HopPrivate || r.Hops[3].Type != HopPublic {
		t.Errorf("跳类型错误: %+v", r.Hops)
	}
	if r.NATLayers != 2 {
		t.Errorf("NAT 层数应为 2（双层 NAT），实际 %d", r.NATLayers)
	}
	if r.CGNATDetected {
		t.Errorf("不应检测到 CGNAT")
	}
}

func TestParseTraceUnix(t *testing.T) {
	out := "traceroute to 223.5.5.5 (223.5.5.5), 15 hops max, 60 byte packets\n" +
		" 1  192.168.1.1  1.234 ms  1.123 ms  0.987 ms\n" +
		" 2  * * *\n" +
		" 3  110.80.1.1  4.567 ms  4.444 ms  4.333 ms\n"

	r := &TraceResult{Target: "223.5.5.5", SourceIP: "192.168.1.5", Gateway: "192.168.1.1", Hops: []HopInfo{}}
	r.Hops = append(r.Hops, HopInfo{Hop: 0, IP: "192.168.1.5", IPs: []string{"192.168.1.5"}, Type: HopLocal})
	r.parseTrace(out)
	r.finalize()

	if len(r.Hops) != 4 {
		t.Fatalf("期望 4 跳，实际 %d", len(r.Hops))
	}
	if r.Hops[1].IP != "192.168.1.1" || r.Hops[1].Type != HopPrivate {
		t.Errorf("第 1 跳解析错误: %+v", r.Hops[1])
	}
	if r.Hops[2].Type != HopTimeout {
		t.Errorf("第 2 跳应为 timeout: %+v", r.Hops[2])
	}
	if r.Hops[3].Type != HopPublic {
		t.Errorf("第 3 跳应为 public: %+v", r.Hops[3])
	}
}
