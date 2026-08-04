package cli

import (
	"testing"

	"gsprobe/internal/model"
)

func TestFormatRouteHopsShowsFullIP(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := routeRow{
		Hops: []string{"8.218.1.2", "202.97.33.1", "61.135.1.1"},
	}
	got := formatRouteHops(r)
	want := "8.218.1.2 → 202.97.33.1 → 61.135.1.1"
	if got != want {
		t.Fatalf("formatRouteHops = %q, want %q", got, want)
	}
}

func TestTerminalPublicIPPrefersRaw(t *testing.T) {
	base := map[string]any{
		"ip":    "8.218.*.*",
		"rawIP": "8.218.1.2",
	}
	if got := terminalPublicIP(base); got != "8.218.1.2" {
		t.Fatalf("terminalPublicIP = %q, want 8.218.1.2", got)
	}
}

func TestTerminalIPSummaryUnmasks(t *testing.T) {
	base := map[string]any{
		"ip":    "8.218.*.*",
		"rawIP": "8.218.1.2",
	}
	section := model.Section{Summary: "8.218.*.* · Example Org · 黑名单 0/12 · 25端口可用"}
	got := terminalIPSummary(section, base)
	want := "8.218.1.2 · Example Org · 黑名单 0/12 · 25端口可用"
	if got != want {
		t.Fatalf("terminalIPSummary = %q, want %q", got, want)
	}
}
