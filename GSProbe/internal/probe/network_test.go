package probe

import (
	"net"
	"testing"
)

func TestDetectNATFromIPs(t *testing.T) {
	if got := detectNATFromIPs("203.0.113.5", []net.IP{net.ParseIP("192.168.1.10")}); got != "NAT 后出网" {
		t.Fatalf("private local = %q, want NAT 后出网", got)
	}
	if got := detectNATFromIPs("203.0.113.5", []net.IP{net.ParseIP("203.0.113.5")}); got != "开放网络无NAT" {
		t.Fatalf("public local = %q, want 开放网络无NAT", got)
	}
	if got := detectNATFromIPs("", []net.IP{net.ParseIP("10.0.0.1")}); got != "未知" {
		t.Fatalf("empty public = %q, want 未知", got)
	}
}

func TestIsPrivateIPv4(t *testing.T) {
	tests := map[string]bool{
		"10.0.0.1":     true,
		"172.16.0.1":   true,
		"192.168.1.1":  true,
		"8.8.8.8":      false,
		"203.0.113.5":  false,
	}
	for ip, want := range tests {
		if got := isPrivateIPv4(net.ParseIP(ip)); got != want {
			t.Fatalf("isPrivateIPv4(%s) = %v, want %v", ip, got, want)
		}
	}
}
