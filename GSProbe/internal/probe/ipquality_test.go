package probe

import "testing"

func TestMaskIP(t *testing.T) {
	if got := maskIP("80.251.10.20"); got != "80.251.*.*" {
		t.Fatalf("maskIP() = %q", got)
	}
	for input, want := range map[string]string{
		"2001:db8:1234:5678:90ab:cdef:1234:5678": "2001:db8:*:*:*:*:*:*",
		"2001:db8::1":                            "2001:db8:*:*:*:*:*:*",
		"::1":                                    "0:0:*:*:*:*:*:*",
	} {
		if got := maskIP(input); got != want {
			t.Errorf("maskIP(%q) = %q, want %q", input, got, want)
		}
	}
}
