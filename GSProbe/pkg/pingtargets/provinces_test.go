package pingtargets

import (
	"testing"
)

func TestValidateChinaProvinces(t *testing.T) {
	db := &DB{
		Provinces: map[string]map[string][]Entry{},
	}
	for i, code := range []string{
		"BJ", "TJ", "HE", "SX", "NM", "LN", "JL", "HL", "SH", "JS", "ZJ", "AH", "FJ", "JX", "SD", "HA",
		"HB", "HN", "GD", "GX", "HI", "CQ", "SC", "GZ", "YN", "XZ", "SN", "GS", "QH", "NX", "XJ", "HK", "MO", "TW",
	} {
		if i >= ChinaProvinceCount {
			break
		}
		db.Provinces[code] = map[string][]Entry{"电信": {{IP: "1.2.3.4"}}}
	}
	if err := ValidateChinaProvinces(db); err != nil {
		t.Fatalf("valid db: %v", err)
	}
	db.Provinces = map[string]map[string][]Entry{"BJ": {"电信": {{IP: "1.2.3.4"}}}}
	if err := ValidateChinaProvinces(db); err == nil {
		t.Fatal("expected error for incomplete provinces")
	}
	delete(db.Provinces, "BJ")
	for _, code := range []string{
		"BJ", "TJ", "HE", "SX", "NM", "LN", "JL", "HL", "SH", "JS", "ZJ", "AH", "FJ", "JX", "SD", "HA",
		"HB", "HN", "GD", "GX", "HI", "CQ", "SC", "GZ", "YN", "XZ", "SN", "GS", "QH", "NX", "XJ", "HK", "MO",
	} {
		db.Provinces[code] = map[string][]Entry{"电信": {{IP: "1.2.3.4"}}}
	}
	if err := ValidateChinaProvinces(db); err == nil {
		t.Fatal("expected error when TW missing")
	}
}
