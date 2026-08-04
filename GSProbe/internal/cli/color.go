package cli

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"gsprobe/pkg/latency"
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorGood      = "\033[38;2;71;223;140m"  // #47df8c 51-100ms
	colorExcellent = "\033[38;2;30;163;78m"   // #1ea34e ≤50ms
	colorFair      = "\033[38;2;154;230;176m" // #9ae6b0 101-200ms
	colorWarn   = "\033[38;2;255;200;87m"   // #ffc857
	colorHigh   = "\033[38;2;255;159;67m"   // #ff9f43
	colorBad    = "\033[38;2;255;107;120m"  // #ff6b78
	colorMuted  = "\033[38;2;141;168;178m"  // #8da8b2
	colorAccent = "\033[38;2;126;200;227m"  // #7ec8e3
	colorTelecom = "\033[38;2;255;159;67m" // #ff9f43
	colorUnicom  = "\033[38;2;255;200;87m" // #ffc857
	colorCMI     = "\033[38;2;180;140;255m" // #b48cff
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(os.Getenv("GSNODE_NO_COLOR"), "1") || strings.EqualFold(os.Getenv("GSNODE_NO_COLOR"), "true") {
		return false
	}
	return true
}

func paint(color, text string) string {
	if !colorEnabled() || text == "" {
		return text
	}
	return color + text + colorReset
}

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func visibleWidth(s string) int {
	return runeWidth(stripANSI(s))
}

func colorHeading(text string) string {
	return paint(colorAccent+colorBold, text)
}

func colorScore(score int) string {
	switch {
	case score >= 8000:
		return paint(colorGood, strconv.Itoa(score))
	case score >= 6000:
		return paint(colorWarn, strconv.Itoa(score))
	case score >= 4000:
		return paint(colorHigh, strconv.Itoa(score))
	default:
		return paint(colorBad, strconv.Itoa(score))
	}
}

func colorStatus(status string) string {
	s := strings.TrimSpace(status)
	switch {
	case s == "" || s == "—":
		return paint(colorMuted, "—")
	case strings.Contains(s, "passed"):
		return paint(colorGood, s)
	case strings.Contains(s, "warning"):
		return paint(colorWarn, s)
	case strings.Contains(s, "failed"), strings.Contains(s, "skipped"):
		return paint(colorBad, s)
	default:
		return s
	}
}

func latencyClass(ms float64, text string) string {
	if latency.IsTimeout(text) {
		return colorBad
	}
	ms = latency.ParseMS(ms, text)
	switch latency.Tier(ms, text) {
	case "unknown":
		return colorMuted
	case "excellent":
		return colorExcellent
	case "good":
		return colorGood
	case "fair":
		return colorFair
	case "medium":
		return colorWarn
	case "high":
		return colorHigh
	default:
		return colorBad
	}
}

func colorLatency(ms float64, text string) string {
	t := strings.TrimSpace(text)
	if t == "" {
		t = "—"
	}
	return paint(latencyClass(ms, t), t)
}

func colorizeMsInText(text string) string {
	if text == "" {
		return paint(colorMuted, "—")
	}
	re := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*ms`)
	out := ""
	last := 0
	for _, loc := range re.FindAllStringSubmatchIndex(text, -1) {
		out += text[last:loc[0]]
		chunk := text[loc[0]:loc[1]]
		ms, _ := strconv.ParseFloat(text[loc[2]:loc[3]], 64)
		out += colorLatency(ms, chunk)
		last = loc[1]
	}
	out += text[last:]
	if out == text {
		return text
	}
	return out
}

func colorStatusValue(text string) string {
	s := strings.TrimSpace(text)
	switch {
	case s == "" || s == "—":
		return paint(colorMuted, "—")
	case strings.Contains(s, "不可用"), strings.Contains(s, "已标记"), strings.Contains(s, "封禁"),
		strings.Contains(s, "失败"), strings.Contains(s, "阻断"), strings.Contains(s, "是"),
		strings.Contains(s, "广播"):
		return paint(colorBad, s)
	case strings.Contains(s, "自制剧"), strings.Contains(s, "仅网页"), strings.Contains(s, "仅APP"):
		return paint(colorWarn, s)
	case strings.Contains(s, "解锁"), s == "可用", s == "正常", s == "否", strings.Contains(s, "原生"):
		return paint(colorGood, s)
	case strings.Contains(s, "查询失败"), strings.Contains(s, "查询受限"), strings.Contains(s, "远端不可达"):
		return paint(colorWarn, s)
	default:
		return paint(colorMuted, s)
	}
}

// colorPurityValue colors IP纯净度 (higher is better): green ≥80, yellow ≥40, else red.
func colorPurityValue(text string) string {
	s := strings.TrimSpace(text)
	if s == "" || s == "—" {
		return paint(colorMuted, "—")
	}
	num := s
	if i := strings.IndexByte(s, '%'); i >= 0 {
		num = strings.TrimSpace(s[:i])
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return paint(colorMuted, s)
	}
	switch {
	case n >= 80:
		return paint(colorGood, s)
	case n >= 40:
		return paint(colorWarn, s)
	default:
		return paint(colorBad, s)
	}
}

func colorBoolValue(text string) string {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "true":
		return paint(colorGood, text)
	case "false":
		return paint(colorBad, text)
	default:
		return colorStatusValue(text)
	}
}

func colorRouteLabel(label string) string {
	s := strings.TrimSpace(label)
	switch {
	case s == "" || s == "—" || strings.EqualFold(s, "unknown"):
		return paint(colorMuted, "未知")
	case strings.Contains(s, "163"), strings.Contains(s, "电信"):
		return paint(colorTelecom, s)
	case strings.Contains(s, "4837"), strings.Contains(s, "联通"):
		return paint(colorUnicom, s)
	case strings.Contains(s, "CMI"), strings.Contains(s, "移动"):
		return paint(colorCMI, s)
	default:
		return paint(colorMuted, s)
	}
}

func colorSectionIcon(status string) string {
	switch status {
	case "passed":
		return paint(colorGood, "✓")
	case "warning":
		return paint(colorWarn, "!")
	case "failed", "skipped":
		return paint(colorBad, "✗")
	default:
		return "·"
	}
}
