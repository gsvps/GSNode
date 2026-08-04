package cli

import (
	"strings"
	"testing"
)

func TestRouteMapTextWithPath(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := routeRow{
		Path: []routeMapNode{
			{Region: "香港", Label: "CMI"},
			{Region: "上海", Label: "电信 163"},
		},
	}
	got := stripANSI(routeMapText(r))
	if !strings.Contains(got, "本机") {
		t.Fatalf("missing 本机: %q", got)
	}
	if !strings.Contains(got, "[香港]CMI") {
		t.Fatalf("missing path node: %q", got)
	}
	if !strings.Contains(got, "[上海]电信 163") {
		t.Fatalf("missing path node: %q", got)
	}
}

func TestRouteMapTextFallbackLabels(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := routeRow{Labels: []string{"电信", "163"}}
	got := stripANSI(routeMapText(r))
	if !strings.Contains(got, "电信") || !strings.Contains(got, "163") {
		t.Fatalf("unexpected fallback: %q", got)
	}
}
