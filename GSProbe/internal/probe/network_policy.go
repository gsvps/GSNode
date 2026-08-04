package probe

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"time"
)

type tcpPolicyInfo struct {
	NAT        string `json:"nat"`
	Congestion string `json:"congestion"`
	Qdisc      string `json:"qdisc"`
	BBR        string `json:"bbr"`
	RmemMax    string `json:"rmemMax"`
	WmemMax    string `json:"wmemMax"`
	TcpRmem    string `json:"tcpRmem"`
	TcpWmem    string `json:"tcpWmem"`
}

type intlLatencyResult struct {
	City string  `json:"city"`
	Host string  `json:"host"`
	Ms   float64 `json:"ms,omitempty"`
	Text string  `json:"text"`
}

// 国际互连使用 NetQuality iperf.json 同款 speedtest 节点（开放 TCP 测速端口），
// 避免对固定 IP:443 探测导致大量「不可达」（如 Quad9 不监听 443、云厂商 IP 变更等）。
var intlTCPTargets = []struct {
	City    string
	Host    string
	Port    string
	AltHost string
	AltPort string
}{
	{"洛杉矶", "speedtest.lax12.us.leaseweb.net", "5201", "", ""},
	{"纽约", "nyc.speedtest.clouvider.net", "5200", "speedtest.nyc1.us.leaseweb.net", "5201"},
	{"东京", "speedtest.tyo11.jp.leaseweb.net", "5201", "", ""},
	{"韩国", "sel.speedtest.sggs.network", "5201", "", ""},
	{"新加坡", "speedtest.sin1.sg.leaseweb.net", "5201", "", ""},
	{"法兰克福", "fra.speedtest.clouvider.net", "5200", "speedtest.fra1.de.leaseweb.net", "5201"},
	{"伦敦", "speedtest.lon1.uk.leaseweb.net", "5201", "", ""},
	{"阿姆斯特丹", "iperf-ams-nl.eranium.net", "5201", "speedtest.ams1.nl.leaseweb.net", "5201"},
	{"圣保罗", "speedtest.sao1.edgoo.net", "9205", "speedtest.sao1.edgoo.net", "5201"},
}

func probeIntlNode(ctx context.Context, city, host, port, altHost, altPort string) intlLatencyResult {
	perCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if ms, text := tcpDialMs(perCtx, host, port); ms > 0 {
		return intlLatencyResult{City: city, Host: host, Ms: ms, Text: text}
	}
	if ms, text := pingTarget(perCtx, host); ms > 0 {
		return intlLatencyResult{City: city, Host: host, Ms: ms, Text: text}
	}
	if altHost != "" {
		p := altPort
		if p == "" {
			p = port
		}
		if ms, text := tcpDialMs(perCtx, altHost, p); ms > 0 {
			return intlLatencyResult{City: city, Host: altHost, Ms: ms, Text: text}
		}
		if ms, text := pingTarget(perCtx, altHost); ms > 0 {
			return intlLatencyResult{City: city, Host: altHost, Ms: ms, Text: text}
		}
	}
	return intlLatencyResult{City: city, Host: host, Text: "不可达"}
}

func probeIntlLatency(ctx context.Context) []intlLatencyResult {
	targets := intlTCPTargets
	out := make([]intlLatencyResult, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		i, target := i, target
		wg.Add(1)
		go func() {
			defer wg.Done()
			out[i] = probeIntlNode(ctx, target.City, target.Host, target.Port, target.AltHost, target.AltPort)
		}()
	}
	wg.Wait()
	return out
}

func isPrivateIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 10 ||
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
		(ip4[0] == 192 && ip4[1] == 168) ||
		ip4[0] == 127 ||
		(ip4[0] == 169 && ip4[1] == 254)
}

func detectNATFromIPs(publicIP string, localIPs []net.IP) string {
	if publicIP == "" {
		return "未知"
	}
	pub := net.ParseIP(publicIP)
	if pub == nil {
		return "未知"
	}
	hasPublicLocal := false
	hasPrivateLocal := false
	for _, ip := range localIPs {
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil {
			if isPrivateIPv4(ip4) {
				hasPrivateLocal = true
			} else if pub.To4() != nil && ip4.Equal(pub.To4()) {
				hasPublicLocal = true
			}
		}
	}
	if hasPublicLocal {
		return "开放网络无NAT"
	}
	if hasPrivateLocal {
		return "NAT 后出网"
	}
	return "未知"
}

func detectNATType(publicIP string, ifaces []net.Interface) string {
	local := make([]net.IP, 0, 8)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ip, _, _ := net.ParseCIDR(a.String())
			if ip != nil {
				local = append(local, ip)
			}
		}
	}
	return detectNATFromIPs(publicIP, local)
}

func collectTCPPolicy(publicIP string, ifaces []net.Interface) tcpPolicyInfo {
	info := tcpPolicyInfo{NAT: detectNATType(publicIP, ifaces)}
	if runtime.GOOS != "linux" {
		info.Congestion = "unknown"
		info.Qdisc = "unknown"
		info.BBR = "unknown"
		return info
	}
	configured := sysctlConfigured()
	congestion := sysctlValue("net.ipv4.tcp_congestion_control", configured)
	qdisc := sysctlValue("net.core.default_qdisc", configured)
	info.Congestion = sysctlPlain(congestion)
	info.Qdisc = sysctlPlain(qdisc)
	info.RmemMax = sysctlPlain(sysctlValue("net.core.rmem_max", configured))
	info.WmemMax = sysctlPlain(sysctlValue("net.core.wmem_max", configured))
	info.TcpRmem = sysctlPlain(sysctlValue("net.ipv4.tcp_rmem", configured))
	info.TcpWmem = sysctlPlain(sysctlValue("net.ipv4.tcp_wmem", configured))
	if strings.Contains(strings.ToLower(congestion), "bbr") {
		info.BBR = "enabled"
	} else if congestion != "unknown" {
		info.BBR = "disabled"
	} else {
		info.BBR = "unknown"
	}
	return info
}

func tcpDialMs(ctx context.Context, host, port string) (float64, string) {
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return 0, "不可达"
	}
	_ = conn.Close()
	ms := float64(time.Since(start).Microseconds()) / 1000
	return ms, fmt.Sprintf("%.1f ms", ms)
}

func bbrLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "enabled":
		return "已启用"
	case "disabled":
		return "未启用"
	default:
		return raw
	}
}

func natLabel(raw string) string {
	switch raw {
	case "开放网络无NAT":
		return raw
	case "NAT 后出网":
		return raw
	default:
		return raw
	}
}
