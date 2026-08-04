package probe

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ipv4HTTPClient forces tcp4 so dual-stack hosts do not pick an IPv6 egress by mistake.
func ipv4HTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, _, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp4", addr)
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// lookupPublicIPv4Who returns GeoIP for the machine's IPv4 egress (never IPv6).
// Web/report fields should still call maskIP on who.IP; rawIP may keep the full address for terminal.
func lookupPublicIPv4Who(ctx context.Context, client *http.Client) (ipWhoResponse, error) {
	v4Client := client
	if v4Client == nil {
		v4Client = ipv4HTTPClient(10 * time.Second)
	}

	var who ipWhoResponse
	if err := getJSON(ctx, v4Client, "https://ipwho.is/?lang=zh-CN", &who); err == nil {
		if ip4 := net.ParseIP(who.IP).To4(); ip4 != nil {
			who.IP = ip4.String()
			who.Type = "IPv4"
			return who, nil
		}
	}

	ip, err := fetchPlainPublicIPv4(ctx, v4Client)
	if err != nil {
		return ipWhoResponse{}, fmt.Errorf("无法取得公网 IPv4: %w", err)
	}
	if err := getJSON(ctx, v4Client, "https://ipwho.is/"+ip+"?lang=zh-CN", &who); err != nil {
		who = ipWhoResponse{IP: ip, Type: "IPv4"}
		return who, nil
	}
	if net.ParseIP(who.IP).To4() == nil {
		who.IP = ip
	}
	who.Type = "IPv4"
	return who, nil
}

func fetchPlainPublicIPv4(ctx context.Context, client *http.Client) (string, error) {
	endpoints := []string{
		"https://api4.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://4.ipw.cn",
	}
	var lastErr error
	for _, url := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "GSProbe/0.1")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
			continue
		}
		ip := strings.TrimSpace(string(body))
		if v4 := net.ParseIP(ip).To4(); v4 != nil {
			return v4.String(), nil
		}
		lastErr = fmt.Errorf("%s: not IPv4 (%q)", url, ip)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no IPv4 endpoint succeeded")
	}
	return "", lastErr
}

// classifyNativeOrBroadcastIP 与 xykt/IPQuality 对齐：
// 使用地国家码 == 注册地国家码 → 原生IP（Geo-consistent）；否则 → 广播IP（Geo-discrepant）。
func classifyNativeOrBroadcastIP(useCountry, regCountry string) string {
	use := normalizeCountryCode(useCountry)
	reg := normalizeCountryCode(regCountry)
	if !isISOCountryCode(use) || !isISOCountryCode(reg) {
		return "—"
	}
	if use == reg {
		return "原生IP"
	}
	return "广播IP"
}

func isISOCountryCode(code string) bool {
	if code == "" {
		return false
	}
	switch code {
	case "NULL", "N/A", "UNKNOWN", "-", "—":
		return false
	}
	if len(code) != 2 && len(code) != 3 {
		return false
	}
	for _, r := range code {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}
