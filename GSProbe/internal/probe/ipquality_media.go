package probe

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"regexp"
	"strings"
)

func getPublicIPv4(ctx context.Context, client *http.Client) string {
	who, err := lookupPublicIPv4Who(ctx, client)
	if err != nil {
		return ""
	}
	return who.IP
}

func isPublicIPv4(ip net.IP) bool {
	if ip = ip.To4(); ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() {
		return false
	}
	return true
}

func samePublicV4(a, b string) bool {
	ia, ib := net.ParseIP(a), net.ParseIP(b)
	if ia == nil || ib == nil {
		return false
	}
	ia, ib = ia.To4(), ib.To4()
	if ia == nil || ib == nil {
		return false
	}
	return ia[0] == ib[0] && ia[1] == ib[1] && ia[2] == ib[2]
}

func dnsUnlockType(ctx context.Context, domain, serverIP string) string {
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", domain)
	if err == nil {
		for _, ip := range ips {
			if isPublicIPv4(ip) && samePublicV4(ip.String(), serverIP) {
				return "原生"
			}
		}
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(999999))
	sub := fmt.Sprintf("gsprobe%d.%s", n.Int64(), domain)
	if ips, err := net.DefaultResolver.LookupIP(ctx, "ip4", sub); err == nil && len(ips) > 0 {
		return "DNS解锁"
	}
	return "原生"
}

var regionRE = regexp.MustCompile(`"region"\s*:\s*"([A-Z]{2})"`)

func extractRegion(body string) string {
	if m := regionRE.FindStringSubmatch(body); len(m) == 2 {
		return m[1]
	}
	return ""
}

func mediaItem(name, status, region, unlockType string) map[string]any {
	return map[string]any{
		"name": name, "status": status, "region": region, "unlockType": unlockType,
	}
}

func mediaMetricText(status, region, unlockType string) string {
	parts := []string{status}
	if region != "" {
		parts = append(parts, fmt.Sprintf("[%s]", region))
	}
	if unlockType != "" {
		parts = append(parts, unlockType)
	}
	return strings.Join(parts, " ")
}
