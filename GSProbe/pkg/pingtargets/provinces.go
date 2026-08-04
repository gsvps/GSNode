package pingtargets

import (
	"fmt"
	"strings"
)

const ChinaProvinceCount = 34

var requiredChinaProvinceCodes = []string{"HK", "MO", "TW"}

// ValidateChinaProvinces ensures the ping target bundle covers all 34 provincial units.
func ValidateChinaProvinces(db *DB) error {
	if db == nil {
		return fmt.Errorf("ping targets: nil db")
	}
	if len(db.Provinces) != ChinaProvinceCount {
		return fmt.Errorf("ping targets: provinces = %d, want %d", len(db.Provinces), ChinaProvinceCount)
	}
	missing := make([]string, 0, 3)
	for _, code := range requiredChinaProvinceCodes {
		if _, ok := db.Provinces[strings.ToUpper(code)]; !ok {
			missing = append(missing, code)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("ping targets: missing required provinces: %s", strings.Join(missing, ", "))
	}
	return nil
}
