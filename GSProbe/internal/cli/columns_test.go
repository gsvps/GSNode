package cli

import (
	"bytes"
	"strings"
	"testing"

	"gsprobe/internal/model"
)

func TestPadRightTruncate(t *testing.T) {
	got := padRight("你好世界测试", 6)
	if runeWidth(got) != 6 {
		t.Fatalf("width = %d, got %q", runeWidth(got), got)
	}
}

func TestTerminalWidthDefault(t *testing.T) {
	t.Setenv("COLUMNS", "")
	if w := terminalWidth(); w < 80 {
		t.Fatalf("default width too small: %d", w)
	}
}

func TestVisibleWidthIgnoresANSI(t *testing.T) {
	s := colorGood + "12345" + colorReset
	if visibleWidth(s) != 5 {
		t.Fatalf("visible width = %d", visibleWidth(s))
	}
}

func TestLatencyClass(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if latencyClass(50, "50.0 ms") != colorExcellent {
		t.Fatal("expected excellent class")
	}
	if latencyClass(80, "80.0 ms") != colorGood {
		t.Fatal("expected good class")
	}
	if latencyClass(150, "150.0 ms") != colorFair {
		t.Fatal("expected fair class")
	}
	if latencyClass(220, "220.0 ms") != colorWarn {
		t.Fatal("expected medium class")
	}
	if latencyClass(300, "300.0 ms") != colorHigh {
		t.Fatal("expected high class")
	}
}

func TestTableRenderHasBorders(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	tbl := newKVTable(6)
	tbl.addKV("CPU", "2224")
	lines := tbl.render(80)
	if len(lines) < 3 {
		t.Fatalf("expected table lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "┌") || !strings.HasPrefix(lines[len(lines)-1], "└") {
		t.Fatalf("missing box borders: %#v", lines)
	}
}

func TestPrintReportTables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	sections := []model.Section{
		{ID: "cpu", Score: 2224, Metrics: []model.Metric{{Name: "型号", Text: "Intel Xeon"}}},
		{ID: "memory", Score: 4821, Metrics: []model.Metric{{Name: "内存总量", Text: "1.7 GB"}}},
		{ID: "disk", Score: 2147},
		{ID: "system", Summary: "Rocky Linux", Metrics: []model.Metric{{Name: "操作系统", Text: "Rocky Linux 9"}}},
		{ID: "ip-quality", Score: 9520, Summary: "8.218.*.* · Example · 黑名单 0/12 · 25端口可用", Details: map[string]any{
			"base": map[string]any{"ip": "8.218.*.*", "rawIP": "8.218.1.2", "asn": "AS123", "organization": "Example"},
		}, Metrics: []model.Metric{{Name: "公网 IP", Text: "8.218.*.*"}}},
		{ID: "services", Score: 7222, Metrics: []model.Metric{{Name: "Netflix", Text: "解锁 原生"}, {Name: "Max", Text: "封禁"}}},
		{ID: "network", Score: 10000, Summary: "香港", Metrics: []model.Metric{{Name: "IPv4", Text: "true"}, {Name: "DNS", Text: "0.5 ms"}}},
		{ID: "route", Score: 10000, Summary: "9/9", Details: map[string]any{
			"returnRoutes": []map[string]any{
				{"city": "北京", "carrier": "电信", "line": "电信 163", "labels": []string{"电信", "163"}, "pingText": "39.0 ms", "pingMs": 39.0},
			},
		}},
		{ID: "china-latency", Score: 10000, Summary: "93/93", Details: map[string]any{
			"chinaLatency": []map[string]any{
				{"code": "BJ", "short": "京", "name": "北京", "carrier": "电信", "text": "39.0 ms", "ms": 39.0, "lossText": "丢0%"},
			},
		}},
	}
	var buf bytes.Buffer
	printReportTables(&buf, sections)
	out := buf.String()
	for _, want := range []string{"系统报告", "IP 质量", "网络质量", "回程路由", "▓▓", "Netflix", "延迟图例", "8.218.1.2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}
