package probe

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type udpTargetResult struct {
	Name string  `json:"name"`
	Host string  `json:"host"`
	Port string  `json:"port"`
	Ms   float64 `json:"ms,omitempty"`
	Text string  `json:"text"`
}

type stunMapping struct {
	Server string  `json:"server"`
	IP     string  `json:"ip"`
	Port   int     `json:"port"`
	Ms     float64 `json:"ms,omitempty"`
}

type udpProbeResult struct {
	Egress     string            `json:"egress"`
	EgressHost string            `json:"egressHost,omitempty"`
	EgressMs   float64           `json:"egressMs,omitempty"`
	EgressText string            `json:"egressText"`
	NATType    string            `json:"natType"`
	STUNText   string            `json:"stunText"`
	QUIC       string            `json:"quic"`
	QUICText   string            `json:"quicText"`
	MappedIP   string            `json:"mappedIp,omitempty"`
	MappedPort int               `json:"mappedPort,omitempty"`
	Targets    []udpTargetResult `json:"targets,omitempty"`
	STUN       []stunMapping     `json:"stun,omitempty"`
}

var udpDNSTargets = []struct {
	Name string
	Host string
	Port string
}{
	{Name: "Cloudflare", Host: "1.1.1.1", Port: "53"},
	{Name: "Google", Host: "8.8.8.8", Port: "53"},
	{Name: "Quad9", Host: "9.9.9.9", Port: "53"},
}

var stunServers = []string{
	"stun.l.google.com:19302",
	"stun.cloudflare.com:3478",
}

const stunMagicCookie = 0x2112A442

func probeUDP(ctx context.Context, publicIP string) udpProbeResult {
	out := udpProbeResult{
		Egress:     "不可用",
		EgressText: "不可用",
		NATType:    "未知",
		STUNText:   "失败",
		QUIC:       "不可用",
		QUICText:   "不可用",
	}

	targets := probeUDPDNSTargets(ctx)
	out.Targets = targets
	for _, t := range targets {
		if t.Ms <= 0 {
			continue
		}
		out.Egress = "可用"
		out.EgressHost = t.Host
		out.EgressMs = t.Ms
		out.EgressText = fmt.Sprintf("%.1f ms (%s)", t.Ms, t.Name)
		break
	}

	mappings := probeSTUNMappings(ctx)
	out.STUN = mappings
	if len(mappings) > 0 {
		out.MappedIP = mappings[0].IP
		out.MappedPort = mappings[0].Port
		out.STUNText = formatSTUNText(mappings[0])
		out.NATType = classifyUDPNAT(mappings, publicIP)
	} else if out.Egress == "可用" {
		out.NATType = "未知"
		out.STUNText = "无响应"
	}

	if probeQUICEgress(ctx) {
		out.QUIC = "可用"
		out.QUICText = "UDP 443 出站正常"
	}

	return out
}

func formatSTUNText(mapping stunMapping) string {
	masked := maskIP(mapping.IP)
	if strings.Contains(mapping.IP, ":") {
		masked = "[" + masked + "]"
	}
	return fmt.Sprintf("%s:***** · %.1f ms", masked, mapping.Ms)
}

func probeUDPDNSTargets(ctx context.Context) []udpTargetResult {
	out := make([]udpTargetResult, len(udpDNSTargets))
	var wg sync.WaitGroup
	for i, target := range udpDNSTargets {
		i, target := i, target
		wg.Add(1)
		go func() {
			defer wg.Done()
			ms, err := probeDNSUDP(ctx, target.Host, target.Port)
			text := "超时"
			if err == nil && ms > 0 {
				text = fmt.Sprintf("%.1f ms", ms)
			}
			out[i] = udpTargetResult{
				Name: target.Name,
				Host: target.Host,
				Port: target.Port,
				Ms:   ms,
				Text: text,
			}
		}()
	}
	wg.Wait()
	return out
}

func probeDNSUDP(ctx context.Context, host, port string) (float64, error) {
	perCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(perCtx, "udp", net.JoinHostPort(host, port))
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	txID := make([]byte, 2)
	if _, err := rand.Read(txID); err != nil {
		return 0, err
	}
	query := buildDNSQuery(binary.BigEndian.Uint16(txID), "cloudflare.com")

	deadline, ok := perCtx.Deadline()
	if !ok {
		deadline = time.Now().Add(4 * time.Second)
	}
	_ = conn.SetDeadline(deadline)

	start := time.Now()
	if _, err = conn.Write(query); err != nil {
		return 0, err
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil || n < 12 {
		return 0, fmt.Errorf("dns udp: no response")
	}
	if buf[0] != txID[0] || buf[1] != txID[1] {
		return 0, fmt.Errorf("dns udp: bad transaction id")
	}
	return float64(time.Since(start).Microseconds()) / 1000, nil
}

func buildDNSQuery(id uint16, domain string) []byte {
	name := encodeDNSName(domain)
	pkt := make([]byte, 12+len(name)+4)
	binary.BigEndian.PutUint16(pkt[0:2], id)
	binary.BigEndian.PutUint16(pkt[2:4], 0x0100)
	binary.BigEndian.PutUint16(pkt[4:6], 1)
	copy(pkt[12:], name)
	off := 12 + len(name)
	binary.BigEndian.PutUint16(pkt[off:off+2], 1)
	binary.BigEndian.PutUint16(pkt[off+2:off+4], 1)
	return pkt
}

func encodeDNSName(domain string) []byte {
	parts := strings.Split(strings.Trim(domain, "."), ".")
	var out []byte
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, byte(len(part)))
		out = append(out, part...)
	}
	out = append(out, 0)
	return out
}

func probeSTUNMappings(ctx context.Context) []stunMapping {
	out := make([]stunMapping, 0, len(stunServers))
	for _, server := range stunServers {
		mapping, ok := stunBinding(ctx, server)
		if ok {
			out = append(out, mapping)
		}
	}
	return out
}

func stunBinding(ctx context.Context, server string) (stunMapping, bool) {
	perCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	txID := make([]byte, 12)
	if _, err := rand.Read(txID); err != nil {
		return stunMapping{}, false
	}
	req := buildSTUNBinding(txID)

	dialer := net.Dialer{Timeout: 3 * time.Second}
	conn, err := dialer.DialContext(perCtx, "udp", server)
	if err != nil {
		return stunMapping{}, false
	}
	defer conn.Close()

	deadline, ok := perCtx.Deadline()
	if !ok {
		deadline = time.Now().Add(4 * time.Second)
	}
	_ = conn.SetDeadline(deadline)

	start := time.Now()
	if _, err = conn.Write(req); err != nil {
		return stunMapping{}, false
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil || n < 20 {
		return stunMapping{}, false
	}
	ip, port, ok := parseSTUNMappedAddr(buf[:n], txID)
	if !ok || ip == nil || port <= 0 {
		return stunMapping{}, false
	}
	return stunMapping{
		Server: server,
		IP:     ip.String(),
		Port:   port,
		Ms:     float64(time.Since(start).Microseconds()) / 1000,
	}, true
}

func buildSTUNBinding(txID []byte) []byte {
	pkt := make([]byte, 20)
	binary.BigEndian.PutUint16(pkt[0:2], 0x0001)
	binary.BigEndian.PutUint16(pkt[2:4], 0)
	binary.BigEndian.PutUint32(pkt[4:8], stunMagicCookie)
	copy(pkt[8:20], txID)
	return pkt
}

func parseSTUNMappedAddr(data, txID []byte) (net.IP, int, bool) {
	if len(data) < 20 || len(txID) != 12 {
		return nil, 0, false
	}
	if binary.BigEndian.Uint16(data[0:2]) != 0x0101 {
		return nil, 0, false
	}
	if !bytesEqual(data[8:20], txID) {
		return nil, 0, false
	}
	attrLen := int(binary.BigEndian.Uint16(data[2:4]))
	attrs := data[20:]
	if attrLen > len(attrs) {
		attrLen = len(attrs)
	}
	attrs = attrs[:attrLen]
	for len(attrs) >= 4 {
		typ := binary.BigEndian.Uint16(attrs[0:2])
		length := int(binary.BigEndian.Uint16(attrs[2:4]))
		if len(attrs) < 4+length {
			break
		}
		val := attrs[4 : 4+length]
		switch typ {
		case 0x0020:
			if ip, port, ok := decodeXORMappedAddress(val); ok {
				return ip, port, true
			}
		case 0x0001:
			if ip, port, ok := decodeMappedAddress(val); ok {
				return ip, port, true
			}
		}
		pad := (4 - length%4) % 4
		attrs = attrs[4+length+pad:]
	}
	return nil, 0, false
}

func decodeXORMappedAddress(val []byte) (net.IP, int, bool) {
	if len(val) < 8 || val[0] != 0 {
		return nil, 0, false
	}
	magic := []byte{0x21, 0x12, 0xA4, 0x42}
	switch val[1] {
	case 0x01:
		port := int(binary.BigEndian.Uint16(val[2:4]) ^ uint16(stunMagicCookie>>16))
		ip := net.IPv4(
			val[4]^magic[0],
			val[5]^magic[1],
			val[6]^magic[2],
			val[7]^magic[3],
		)
		return ip, port, true
	case 0x02:
		if len(val) < 20 {
			return nil, 0, false
		}
		port := int(binary.BigEndian.Uint16(val[2:4]) ^ uint16(stunMagicCookie>>16))
		ip := make(net.IP, 16)
		for i := 0; i < 16; i++ {
			ip[i] = val[4+i] ^ magic[i%4]
		}
		return ip, port, true
	default:
		return nil, 0, false
	}
}

func decodeMappedAddress(val []byte) (net.IP, int, bool) {
	if len(val) < 8 || val[0] != 0 {
		return nil, 0, false
	}
	port := int(binary.BigEndian.Uint16(val[2:4]))
	switch val[1] {
	case 0x01:
		if len(val) < 8 {
			return nil, 0, false
		}
		return net.IPv4(val[4], val[5], val[6], val[7]).To4(), port, true
	case 0x02:
		if len(val) < 20 {
			return nil, 0, false
		}
		ip := make(net.IP, 16)
		copy(ip, val[4:20])
		return ip, port, true
	default:
		return nil, 0, false
	}
}

func classifyUDPNAT(mappings []stunMapping, publicIP string) string {
	if len(mappings) == 0 {
		return "未知"
	}
	if len(mappings) >= 2 &&
		mappings[0].IP == mappings[1].IP &&
		mappings[0].Port != mappings[1].Port {
		return "对称型 NAT"
	}
	mappedIP := strings.TrimSpace(mappings[0].IP)
	publicIP = strings.TrimSpace(publicIP)
	if publicIP != "" && mappedIP == publicIP {
		return "公网直连"
	}
	if publicIP != "" && mappedIP != "" {
		return "端口限制型 NAT"
	}
	return "NAT 映射"
}

func probeQUICEgress(ctx context.Context) bool {
	targets := []string{
		"1.1.1.1:443",
		"8.8.8.8:443",
	}
	for _, target := range targets {
		if udpPortWritable(ctx, target) {
			return true
		}
	}
	return false
}

func udpPortWritable(ctx context.Context, address string) bool {
	perCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(perCtx, "udp", address)
	if err != nil {
		return false
	}
	defer conn.Close()
	deadline, ok := perCtx.Deadline()
	if !ok {
		deadline = time.Now().Add(3 * time.Second)
	}
	_ = conn.SetDeadline(deadline)
	_, err = conn.Write([]byte{0x40})
	return err == nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
