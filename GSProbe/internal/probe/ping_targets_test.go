package probe

import (
	"testing"
)

func TestLoadPingTargetDBEmbedded(t *testing.T) {
	db, err := parsePingTargetDB(embeddedPingTargets)
	if err != nil {
		t.Fatal(err)
	}
	if db.Version == "" {
		t.Fatal("version empty")
	}
	if len(db.Provinces) != chinaProvinceCount {
		t.Fatalf("provinces = %d, want %d", len(db.Provinces), chinaProvinceCount)
	}
	if len(db.Routes) != 3 {
		t.Fatalf("routes cities = %d, want 3", len(db.Routes))
	}
}

func TestPingTargetDBProvinceIPs(t *testing.T) {
	db, err := parsePingTargetDB(embeddedPingTargets)
	if err != nil {
		t.Fatal(err)
	}
	ips := db.ProvinceIPs("BJ", "电信")
	if len(ips) == 0 {
		t.Fatalf("BJ telecom ips empty")
	}
	if ips[0] != "220.181.173.35" {
		t.Fatalf("BJ telecom primary = %#v", ips)
	}
}

func TestPingTargetDBRoutePingIPs(t *testing.T) {
	db, err := parsePingTargetDB(embeddedPingTargets)
	if err != nil {
		t.Fatal(err)
	}
	ips := db.RoutePingIPs("北京", "电信")
	if len(ips) == 0 {
		t.Fatalf("beijing telecom merged empty")
	}
	if ips[0] != "220.181.173.35" {
		t.Fatalf("prefer cdn first, got %#v", ips)
	}
}

func TestProbeTargetFailoverEmpty(t *testing.T) {
	host, ms, text := probeTargetFailover(t.Context(), nil)
	if host != "" || ms != 0 || text != "不可达" {
		t.Fatalf("empty failover = %q %v %q", host, ms, text)
	}
}

func TestBuildRouteTargets(t *testing.T) {
	db, err := parsePingTargetDB(embeddedPingTargets)
	if err != nil {
		t.Fatal(err)
	}
	targets := buildRouteTargets(db)
	if len(targets) != 9 {
		t.Fatalf("route targets = %d", len(targets))
	}
}
