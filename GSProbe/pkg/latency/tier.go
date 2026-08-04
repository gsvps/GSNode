package latency

import (
	"regexp"
	"strconv"
	"strings"
)

// PING 延迟分档（与报告延迟图例一致）：
// ≤50 · 51-100 · 101-200 · 201-250 · >250 · 超时
const (
	TierExcellentMax = 50
	TierGoodMax      = 100
	TierFairMax      = 200
	TierMediumMax    = 250
)

var msPattern = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*ms`)

func IsTimeout(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(t, "超时") || strings.Contains(t, "timeout")
}

func ParseMS(ms float64, text string) float64 {
	if ms > 0 {
		return ms
	}
	if m := msPattern.FindStringSubmatch(text); len(m) == 2 {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v
		}
	}
	return ms
}

// Tier returns: excellent | good | fair | medium | high | timeout | unknown
func Tier(ms float64, text string) string {
	if IsTimeout(text) {
		return "timeout"
	}
	ms = ParseMS(ms, text)
	switch {
	case ms <= 0 && (text == "" || text == "—"):
		return "unknown"
	case ms <= TierExcellentMax:
		return "excellent"
	case ms <= TierGoodMax:
		return "good"
	case ms <= TierFairMax:
		return "fair"
	case ms <= TierMediumMax:
		return "medium"
	default:
		return "high"
	}
}

func ScoreFromLatency(ms float64) int {
	switch {
	case ms <= TierExcellentMax:
		return 100
	case ms <= TierGoodMax:
		return 95
	case ms <= TierFairMax:
		return 85
	case ms <= TierMediumMax:
		return 70
	default:
		return 55
	}
}
