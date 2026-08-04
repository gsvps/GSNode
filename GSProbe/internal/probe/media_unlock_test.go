package probe

import (
	"net/http"
	"testing"
)

func TestNetflixRegionFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.netflix.com/us-en/title/81280792": "US",
		"https://www.netflix.com/jp/title/81280792":    "JP",
		"https://www.netflix.com/title/81280792":       "US",
		"": "",
	}
	for in, want := range cases {
		if got := netflixRegionFromURL(in); got != want {
			t.Fatalf("netflixRegionFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractNetflixRegion(t *testing.T) {
	h := http.Header{}
	h.Set("Location", "https://www.netflix.com/sg-en/title/81280792")
	if got := extractNetflixRegion(h, nil); got != "SG" {
		t.Fatalf("extractNetflixRegion = %q, want SG", got)
	}
}

func TestIsPositiveMediaStatus(t *testing.T) {
	for _, status := range []string{"解锁", "自制剧", "仅网页", "仅APP"} {
		if !isPositiveMediaStatus(status) {
			t.Fatalf("%q should be positive", status)
		}
	}
	if isPositiveMediaStatus("封禁") {
		t.Fatal("封禁 should not be positive")
	}
}

func TestCheckGenericMediaBlockedByStatus(t *testing.T) {
	if serviceUnlocked(403, "welcome") {
		t.Fatal("403 should be blocked")
	}
	if serviceUnlocked(451, "welcome") {
		t.Fatal("451 should be blocked")
	}
}

func TestMediaMetricTextPartial(t *testing.T) {
	got := mediaMetricText("自制剧", "US", "原生")
	want := "自制剧 [US] 原生"
	if got != want {
		t.Fatalf("mediaMetricText = %q, want %q", got, want)
	}
}

func TestMediaMetricTextBlocked(t *testing.T) {
	got := mediaMetricText("封禁", "US", "DNS解锁")
	want := "封禁 [US] DNS解锁"
	if got != want {
		t.Fatalf("mediaMetricText = %q, want %q", got, want)
	}
}
