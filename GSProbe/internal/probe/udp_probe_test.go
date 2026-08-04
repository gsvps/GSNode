package probe

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestBuildDNSQuery(t *testing.T) {
	pkt := buildDNSQuery(0x1234, "cloudflare.com")
	if len(pkt) < 20 {
		t.Fatalf("dns query too short: %d", len(pkt))
	}
	if binary.BigEndian.Uint16(pkt[0:2]) != 0x1234 {
		t.Fatalf("bad query id")
	}
}

func TestEncodeDNSName(t *testing.T) {
	got := encodeDNSName("cloudflare.com")
	want := []byte{10, 'c', 'l', 'o', 'u', 'd', 'f', 'l', 'a', 'r', 'e', 3, 'c', 'o', 'm', 0}
	if !bytesEqual(got, want) {
		t.Fatalf("encodeDNSName() = %v, want %v", got, want)
	}
}

func TestDecodeXORMappedAddress(t *testing.T) {
	val := []byte{
		0x00, 0x01,
		0x00, 0x00,
		0x20, 0x13, 0xA5, 0x43,
	}
	ip, port, ok := decodeXORMappedAddress(val)
	if !ok {
		t.Fatal("decodeXORMappedAddress failed")
	}
	if port != 0x2112 {
		t.Fatalf("port = %d, want %d", port, 0x2112)
	}
	if !ip.Equal(net.IPv4(1, 1, 1, 1)) {
		t.Fatalf("ip = %s, want 1.1.1.1", ip)
	}
}

func TestFormatSTUNTextMasksAddressAndPort(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mapping stunMapping
		want    string
	}{
		{name: "IPv4", mapping: stunMapping{IP: "192.255.216.166", Port: 36014, Ms: 32.3}, want: "192.255.*.*:***** · 32.3 ms"},
		{name: "IPv6", mapping: stunMapping{IP: "2001:db8:1234:5678::1", Port: 36014, Ms: 18.5}, want: "[2001:db8:*:*:*:*:*:*]:***** · 18.5 ms"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatSTUNText(tc.mapping); got != tc.want {
				t.Fatalf("formatSTUNText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifyUDPNAT(t *testing.T) {
	cases := []struct {
		name     string
		mappings []stunMapping
		publicIP string
		want     string
	}{
		{
			name: "symmetric",
			mappings: []stunMapping{
				{IP: "203.0.113.1", Port: 40001},
				{IP: "203.0.113.1", Port: 40002},
			},
			want: "对称型 NAT",
		},
		{
			name: "public",
			mappings: []stunMapping{
				{IP: "203.0.113.5", Port: 40001},
			},
			publicIP: "203.0.113.5",
			want:     "公网直连",
		},
		{
			name: "restricted",
			mappings: []stunMapping{
				{IP: "203.0.113.5", Port: 40001},
			},
			publicIP: "198.51.100.2",
			want:     "端口限制型 NAT",
		},
	}
	for _, tc := range cases {
		if got := classifyUDPNAT(tc.mappings, tc.publicIP); got != tc.want {
			t.Fatalf("%s: classifyUDPNAT() = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestParseSTUNMappedAddr(t *testing.T) {
	txID := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	attr := []byte{
		0x00, 0x20,
		0x00, 0x08,
		0x00, 0x01,
		0x00, 0x00,
		0x20, 0x13, 0xA5, 0x43,
	}
	pkt := make([]byte, 20+len(attr))
	binary.BigEndian.PutUint16(pkt[0:2], 0x0101)
	binary.BigEndian.PutUint16(pkt[2:4], uint16(len(attr)))
	binary.BigEndian.PutUint32(pkt[4:8], stunMagicCookie)
	copy(pkt[8:20], txID)
	copy(pkt[20:], attr)

	ip, port, ok := parseSTUNMappedAddr(pkt, txID)
	if !ok || !ip.Equal(net.IPv4(1, 1, 1, 1)) || port != 0x2112 {
		t.Fatalf("parseSTUNMappedAddr() = %v:%d ok=%v", ip, port, ok)
	}
}
