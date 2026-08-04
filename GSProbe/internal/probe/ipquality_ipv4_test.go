package probe

import "testing"

func TestClassifyNativeOrBroadcastIPGeoConsistent(t *testing.T) {
	cases := []struct {
		use, reg, want string
	}{
		{"JP", "JP", "原生IP"},
		{"jp", "JP", "原生IP"},
		{"US", "JP", "广播IP"},
		{"HK", "US", "广播IP"},
		{"", "JP", "—"},
		{"JP", "", "—"},
		{"JP", "null", "—"},
		{"", "", "—"},
	}
	for _, tc := range cases {
		if got := classifyNativeOrBroadcastIP(tc.use, tc.reg); got != tc.want {
			t.Errorf("classifyNativeOrBroadcastIP(%q, %q) = %q, want %q", tc.use, tc.reg, got, tc.want)
		}
	}
}

func TestMaskIPKeepsIPv4Form(t *testing.T) {
	if got := maskIP("203.0.113.45"); got != "203.0.*.*" {
		t.Fatalf("maskIP = %q", got)
	}
}
