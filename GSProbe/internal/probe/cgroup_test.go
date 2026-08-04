package probe

import "testing"

func TestParseCPUMaxV2(t *testing.T) {
	_, _, unlimited := parseCPUMaxV2("max 100000")
	if !unlimited {
		t.Fatal("expected unlimited")
	}
	q, p, unlimited := parseCPUMaxV2("200000 100000")
	if unlimited || q != 200000 || p != 100000 {
		t.Fatalf("unexpected parse: %d %d %v", q, p, unlimited)
	}
	if got := cpuCoresFromQuota(q, p); got != 2 {
		t.Fatalf("cores = %v", got)
	}
}

func TestParseMemoryMaxV2(t *testing.T) {
	_, unlimited := parseMemoryMaxV2("max")
	if !unlimited {
		t.Fatal("expected unlimited memory")
	}
	bytes, unlimited := parseMemoryMaxV2("4294967296")
	if unlimited || bytes != 4294967296 {
		t.Fatalf("bytes = %d unlimited=%v", bytes, unlimited)
	}
}

func TestParseIOMaxV2(t *testing.T) {
	if parseIOMaxV2("max") {
		t.Fatal("max should not be limited")
	}
	raw := "8:0 rbps=max wbps=1048576 riops=max wiops=max"
	if !parseIOMaxV2(raw) {
		t.Fatal("expected io limited")
	}
}

func TestFinalizeCgroupShrink(t *testing.T) {
	out := finalizeCgroupLimits(cgroupLimits{
		VisibleCPUs: 4,
		CPULimited:  true,
		CPUMaxCores: 2,
	})
	if out.CPUStatus != "可能缩水" {
		t.Fatalf("status = %q", out.CPUStatus)
	}
	if out.CPUText == "" || out.CPUText == "无限制" {
		t.Fatalf("text = %q", out.CPUText)
	}
}
