package probe

import "testing"

func TestComputeIPPurityWeightedBlend(t *testing.T) {
	records := []ipDBRecord{
		{Name: "ipapi", Score: 80, ScoreText: scoreText(80, riskLabelCN(80))},
		{Name: "Scamalytics", Score: 21, ScoreText: scoreText(21, riskLabelCN(21))},
		{Name: "IPQS", Score: 90, ScoreText: scoreText(90, riskLabelCN(90))},
	}
	// weights 15/35/30 → avg = (15*80+35*21+30*90)/80 = 57
	// max=90, dnsbl=50 → risk = (57*65+90*20+50*15)/100 = 62 → purity 38 bad
	got, ok := computeIPPurity(records, 5, 10, purityFactors{})
	if !ok {
		t.Fatal("expected ok")
	}
	if got.Percent != 38 || got.Source != "weighted" || got.Level != "bad" {
		t.Fatalf("got %+v", got)
	}
}

func TestComputeIPPurityBlendWithDNSBL(t *testing.T) {
	records := []ipDBRecord{
		{Name: "IPQS", Score: 40, ScoreText: scoreText(40, riskLabelCN(40))},
		{Name: "AbuseIPDB", Score: 20, ScoreText: scoreText(20, riskLabelCN(20))},
	}
	// weights 30+20=50 → avg=(30*40+20*20)/50=32, max=40, dnsbl=50
	// risk=(32*65+40*20+50*15)/100=36 → purity 64 warn
	got, ok := computeIPPurity(records, 5, 10, purityFactors{})
	if !ok {
		t.Fatal("expected ok")
	}
	if got.Percent != 64 || got.Source != "weighted" || got.Level != "warn" {
		t.Fatalf("got %+v", got)
	}
}

func TestComputeIPPurityScamalyticsOnlyClean(t *testing.T) {
	records := []ipDBRecord{
		{Name: "Scamalytics", Score: 10, ScoreText: scoreText(10, riskLabelCN(10))},
	}
	got, ok := computeIPPurity(records, 0, 0, purityFactors{})
	if !ok {
		t.Fatal("expected ok")
	}
	// avg=10, max=10 → (10*80+10*20)/100=10 → purity 90 good
	if got.Percent != 90 || got.Source != "weighted" || got.Level != "good" {
		t.Fatalf("got %+v", got)
	}
}

func TestComputeIPPurityProxyPenalty(t *testing.T) {
	records := []ipDBRecord{
		{Name: "Scamalytics", Score: 10, ScoreText: scoreText(10, riskLabelCN(10))},
	}
	got, ok := computeIPPurity(records, 0, 0, purityFactors{Proxy: true})
	if !ok {
		t.Fatal("expected ok")
	}
	// base 10 + proxy 15 = 25 → purity 75 warn
	if got.Percent != 75 || got.Source != "weighted" || got.Level != "warn" {
		t.Fatalf("got %+v", got)
	}
}

func TestComputeIPPurityTorHeavyPenalty(t *testing.T) {
	records := []ipDBRecord{
		{Name: "Scamalytics", Score: 5, ScoreText: scoreText(5, riskLabelCN(5))},
	}
	got, ok := computeIPPurity(records, 0, 0, purityFactors{Tor: true, VPN: true})
	if !ok {
		t.Fatal("expected ok")
	}
	// base 5 + tor 25 + vpn 12 = 42 → purity 58 warn
	if got.Percent != 58 || got.Level != "warn" {
		t.Fatalf("got %+v", got)
	}
}

func TestComputeIPPurityNoData(t *testing.T) {
	_, ok := computeIPPurity(nil, 0, 0, purityFactors{})
	if ok {
		t.Fatal("expected no result")
	}
}

func TestComputeIPPurityFactorsOnly(t *testing.T) {
	got, ok := computeIPPurity(nil, 0, 0, purityFactors{Proxy: true})
	if !ok {
		t.Fatal("expected ok")
	}
	if got.Percent != 85 || got.Source != "factors" {
		t.Fatalf("got %+v", got)
	}
}

func TestPurityLevel(t *testing.T) {
	if purityLevel(80) != "good" || purityLevel(40) != "warn" || purityLevel(39) != "bad" {
		t.Fatal("level thresholds mismatch")
	}
}

func TestPurityFactorsFromRecords(t *testing.T) {
	tr, fa := true, false
	records := []ipDBRecord{
		{Name: "IPinfo", Proxy: &tr, VPN: &fa},
		{Name: "ipapi", Proxy: &fa, Tor: &tr},
	}
	f := purityFactorsFromRecords(records)
	if !f.Proxy || !f.Tor || f.VPN {
		t.Fatalf("factors = %+v", f)
	}
}
