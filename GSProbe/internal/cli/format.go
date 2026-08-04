package cli

import (
	"fmt"
	"math"
	"strings"

	"gsprobe/internal/model"
)

func stars(n int) string {
	if n < 1 {
		n = 1
	}
	if n > 5 {
		n = 5
	}
	return strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
}

func formatMetric(m model.Metric) string {
	if m.Text != "" {
		return m.Text
	}
	if m.Unit != "" {
		if m.Unit == "ms" || m.Unit == "ns/op" || m.Unit == "%" {
			return fmt.Sprintf("%.1f %s", m.Value, m.Unit)
		}
		if m.Value == math.Trunc(m.Value) {
			return fmt.Sprintf("%.0f %s", m.Value, m.Unit)
		}
		return fmt.Sprintf("%.2f %s", m.Value, m.Unit)
	}
	if m.Value != 0 {
		return fmt.Sprintf("%.2f", m.Value)
	}
	return "—"
}
