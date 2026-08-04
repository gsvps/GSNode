package probe

import (
	"reflect"
	"testing"
)

func TestDetectRouteLabels(t *testing.T) {
	input := "AS4134 CHINANET-BB -> AS4809 CN2-BackBone -> AS58807 CMIN2-NET"
	want := []string{"CN2 GIA", "电信 163", "CMIN2"}
	if got := DetectRouteLabels(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("DetectRouteLabels() = %#v, want %#v", got, want)
	}
}

func TestRouteLineUnknown(t *testing.T) {
	if got := routeLine(nil); got != "Unknown" {
		t.Fatalf("routeLine(nil) = %q", got)
	}
}

func TestDetectRouteHops(t *testing.T) {
	input := "1  10.0.0.1  1 ms\n2  59.43.1.1  120 ms\n3  59.43.1.1  121 ms"
	want := []string{"10.0.0.1", "59.43.1.1"}
	if got := detectRouteHops(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("detectRouteHops() = %#v, want %#v", got, want)
	}
}

func TestParseRoutePath(t *testing.T) {
	input := `1   172.18.0.1      *                         RFC1918
4   223.119.66.245  AS58453  [CMI-INT]        中国 香港   cmi.chinamobile.com  移动
5   223.120.6.17    AS58453  [CMI-INT]        美国 加利福尼亚 洛杉矶  cmi.chinamobile.com  移动
7   219.158.96.25   AS4837   [CU169-BACKBONE] 中国 北京   chinaunicom.cn  联通
11  202.106.50.1    AS4808   [UNICOM-BJ]      中国 北京  丰台区 www.bbn.com.cn  联通`
	got := parseRoutePath(input)
	if len(got) < 3 {
		t.Fatalf("parseRoutePath() = %#v, want at least 3 nodes", got)
	}
	if got[0].Label != "CMI" || got[0].Region != "香港" {
		t.Fatalf("first node = %#v, want 香港 CMI", got[0])
	}
	if got[len(got)-1].Region != "北京" {
		t.Fatalf("last node region = %q, want 北京", got[len(got)-1].Region)
	}
}

func TestServiceUnlocked(t *testing.T) {
	tests := []struct {
		status int
		body   string
		want   bool
	}{
		{200, "welcome", true},
		{302, "redirect", true},
		{403, "access denied", false},
		{200, "This title is not available in your region", false},
	}
	for _, test := range tests {
		if got := serviceUnlocked(test.status, test.body); got != test.want {
			t.Fatalf("serviceUnlocked(%d, %q) = %v, want %v", test.status, test.body, got, test.want)
		}
	}
}
