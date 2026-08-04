package probe

import "testing"

const chinaProvinceCount = 34

func TestChinaProvincesCount(t *testing.T) {
	if len(chinaProvinces) != chinaProvinceCount {
		t.Fatalf("chinaProvinces len = %d, want %d", len(chinaProvinces), chinaProvinceCount)
	}
	db, err := parsePingTargetDB(embeddedPingTargets)
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Provinces) != chinaProvinceCount {
		t.Fatalf("ping target provinces = %d, want %d", len(db.Provinces), chinaProvinceCount)
	}
}

func TestChinaLatencyProbeTimeout(t *testing.T) {
	cases := []struct {
		concurrency, total int
		wantSec            int
	}{
		{20, 102, 140},
		{5, 102, 395},
		{3, 102, 590},
		{1, 102, 600},
		{5, 10, 120},
	}
	for _, tc := range cases {
		got := int(chinaLatencyProbeTimeout(tc.concurrency, tc.total).Seconds())
		if got != tc.wantSec {
			t.Fatalf("timeout(%d,%d)=%ds want %ds", tc.concurrency, tc.total, got, tc.wantSec)
		}
	}
}
