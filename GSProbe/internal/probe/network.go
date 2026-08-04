package probe

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"gsprobe/internal/model"
)

type Network struct{}

func (Network) ID() string   { return "network" }
func (Network) Name() string { return "Network" }
func (p Network) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	details := map[string]any{}
	log("Network: 枚举网卡与 IP 协议")
	ifaces, _ := net.Interfaces()
	v4, v6 := false, false
	names := []string{}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		names = append(names, iface.Name)
		addrs, _ := iface.Addrs()
		for _, a := range addrs {
			ip, _, _ := net.ParseCIDR(a.String())
			if ip == nil {
				continue
			}
			if ip.IsLoopback() {
				continue
			}
			if ip.To4() != nil {
				v4 = true
			} else {
				v6 = true
			}
		}
	}
	details["interfaces"] = names
	log("Network: 查询公网 IPv4、ASN 与 GeoIP")
	publicIP, country, city, asn, org, isp := "", "", "", 0, "", ""
	v4Client := ipv4HTTPClient(6 * time.Second)
	if who, err := lookupPublicIPv4Who(ctx, v4Client); err == nil {
		publicIP = who.IP
		country = who.Country
		city = who.City
		asn = who.Connection.ASN
		org = who.Connection.Org
		isp = who.Connection.ISP
	}
	if publicIP != "" {
		details["publicIP"] = publicIP
	}
	if asn > 0 {
		details["asn"] = fmt.Sprintf("AS%d", asn)
	}
	if org != "" {
		details["organization"] = org
	}
	if isp != "" {
		details["isp"] = isp
	}
	if country != "" || city != "" {
		details["location"] = strings.TrimSpace(country + " " + city)
		details["datacenter"] = map[string]string{
			"country": country,
			"city":    city,
			"label":   strings.TrimSpace(country + " " + city),
		}
	}

	log("Network: 本地 TCP 策略与 NAT 检测")
	tcpPolicy := collectTCPPolicy(publicIP, ifaces)
	details["tcpPolicy"] = tcpPolicy

	log("Network: UDP 出站、STUN 与 QUIC 检测")
	udpProbe := probeUDP(ctx, publicIP)
	details["udpProbe"] = udpProbe

	log("Network: DNS 与 TCP 延迟测试")
	dnsStart := time.Now()
	_, dnsErr := net.DefaultResolver.LookupHost(ctx, "cloudflare.com")
	dnsMs := float64(time.Since(dnsStart).Microseconds()) / 1000
	tcpMs := 0.0
	tcpStart := time.Now()
	if c, err := net.DialTimeout("tcp", "1.1.1.1:443", 4*time.Second); err == nil {
		tcpMs = float64(time.Since(tcpStart).Microseconds()) / 1000
		_ = c.Close()
	}

	log("Network: 国际节点 TCP 延迟")
	intl := probeIntlLatency(ctx)
	details["intlLatency"] = intl

	log("Network: Cloudflare 上下行测速")
	speed := probeSpeedTest(ctx)
	details["speedTest"] = speed

	status := model.StatusPassed
	if publicIP == "" || dnsErr != nil {
		status = model.StatusWarning
	}
	if udpProbe.Egress != "可用" {
		status = model.StatusWarning
	}
	s.Status = status
	s.Score = min(10000, 5000+int(speed.DownloadMbps*10))
	s.Summary = strings.TrimSpace(country + " " + city)

	tcpMetric := model.Metric{Name: "TCP 1.1.1.1", Value: tcpMs, Unit: "ms"}
	if tcpMs == 0 {
		tcpMetric = model.Metric{Name: "TCP 1.1.1.1", Text: "不可达"}
	}
	s.Metrics = []model.Metric{
		{Name: "IPv4", Text: fmt.Sprint(v4)},
		{Name: "IPv6", Text: fmt.Sprint(v6)},
		{Name: "NAT", Text: natLabel(tcpPolicy.NAT)},
		{Name: "TCP 拥塞", Text: tcpPolicy.Congestion},
		{Name: "队列调度", Text: tcpPolicy.Qdisc},
		{Name: "BBR", Text: bbrLabel(tcpPolicy.BBR)},
		{Name: "DNS", Value: dnsMs, Unit: "ms"},
		tcpMetric,
		{Name: "UDP 出站", Text: udpProbe.EgressText},
		{Name: "UDP NAT", Text: udpProbe.NATType},
		{Name: "QUIC", Text: udpProbe.QUICText},
	}
	if asn > 0 {
		s.Metrics = append([]model.Metric{{Name: "ASN", Text: fmt.Sprintf("AS%d", asn)}}, s.Metrics...)
	}
	if org != "" {
		s.Metrics = append(s.Metrics, model.Metric{Name: "组织", Text: org})
	}
	if speed.DownloadMbps > 0 {
		s.Metrics = append(s.Metrics, model.Metric{Name: "下载", Value: speed.DownloadMbps, Unit: "Mbps", Higher: true})
	} else if speed.DownloadText != "" {
		s.Metrics = append(s.Metrics, model.Metric{Name: "下载", Text: speed.DownloadText})
	}
	if speed.UploadMbps > 0 {
		s.Metrics = append(s.Metrics, model.Metric{Name: "上传", Value: speed.UploadMbps, Unit: "Mbps", Higher: true})
	} else if speed.UploadText != "" {
		s.Metrics = append(s.Metrics, model.Metric{Name: "上传", Text: speed.UploadText})
	}
	s.Details = details
	return Finish(s, started)
}
