package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"gsprobe/internal/model"
)

func detailMap(section model.Section, key string) map[string]any {
	raw, ok := section.Details[key]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out map[string]any
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

func detailString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int(t)) {
			return fmt.Sprintf("%d", int(t))
		}
		return fmt.Sprintf("%.1f", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return detailString(m[key])
}

type serviceItem struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Region     string `json:"region"`
	UnlockType string `json:"unlockType"`
}

func parseServiceItems(s model.Section) []serviceItem {
	raw, ok := s.Details["items"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var items []serviceItem
	if json.Unmarshal(b, &items) != nil {
		return nil
	}
	return items
}

func serviceStatusText(it serviceItem) string {
	parts := []string{it.Status}
	if it.Region != "" {
		parts = append(parts, "["+it.Region+"]")
	}
	if it.UnlockType != "" {
		parts = append(parts, it.UnlockType)
	}
	return strings.Join(parts, " ")
}

type intlLatencyRow struct {
	City string  `json:"city"`
	Text string  `json:"text"`
	Ms   float64 `json:"ms,omitempty"`
}

func parseIntlLatency(s model.Section) []intlLatencyRow {
	raw, ok := s.Details["intlLatency"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var rows []intlLatencyRow
	if json.Unmarshal(b, &rows) != nil {
		return nil
	}
	return rows
}

func formatIPCategories(base map[string]any) string {
	raw, ok := base["ipCategories"]
	if !ok || raw == nil {
		return ""
	}
	b, _ := json.Marshal(raw)
	var cats []map[string]any
	if json.Unmarshal(b, &cats) != nil {
		return ""
	}
	var active []string
	for _, c := range cats {
		if c["active"] == true {
			active = append(active, mapString(c, "label"))
		}
	}
	return strings.Join(active, " · ")
}

func formatRiskScores(scores map[string]any) []string {
	if len(scores) == 0 {
		return nil
	}
	var lines []string
	for k, v := range scores {
		if k == "提示" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", k, detailString(v)))
	}
	return lines
}

func formatMailSummary(mail map[string]any) string {
	if mail == nil {
		return ""
	}
	results, _ := mail["results"].([]any)
	okCount := 0
	for _, r := range results {
		if m, isMap := r.(map[string]any); isMap && mapString(m, "status") == "可用" {
			okCount++
		}
	}
	if len(results) == 0 {
		return mapString(mail, "localPort25")
	}
	return fmt.Sprintf("邮件 %d/%d 可用", okCount, len(results))
}

func formatBlacklistSummary(bl map[string]any) string {
	if bl == nil {
		return ""
	}
	listed := mapString(bl, "listed")
	valid := mapString(bl, "valid")
	normal := mapString(bl, "normal")
	return fmt.Sprintf("有效 %s · 正常 %s · 标记 %s", valid, normal, listed)
}

func formatRouteHops(r routeRow) string {
	if len(r.Hops) > 0 {
		parts := make([]string, 0, len(r.Hops))
		for _, h := range r.Hops {
			h = strings.TrimSpace(h)
			if h != "" && h != "*" && h != "—" {
				parts = append(parts, h)
			}
		}
		if len(parts) > 0 {
			if len(parts) > 6 {
				return strings.Join(parts[:3], " → ") + " → … → " + strings.Join(parts[len(parts)-2:], " → ")
			}
			return strings.Join(parts, " → ")
		}
	}
	return ""
}

// terminalPublicIP 终端展示完整公网 IP（报告 JSON 中 ip 字段仍可脱敏供网页用）。
func terminalPublicIP(base map[string]any) string {
	if raw := mapString(base, "rawIP"); raw != "" {
		return raw
	}
	return mapString(base, "ip")
}

func terminalIPSummary(section model.Section, base map[string]any) string {
	summary := strings.TrimSpace(section.Summary)
	if summary == "" {
		return ""
	}
	raw := mapString(base, "rawIP")
	masked := mapString(base, "ip")
	if raw != "" && masked != "" && masked != raw {
		summary = strings.Replace(summary, masked, raw, 1)
	}
	return summary
}

func detailNestedString(section model.Section, keys ...string) string {
	cur := any(section.Details)
	for _, k := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[k]
	}
	return detailString(cur)
}

func cpuFlagsLine(section model.Section) string {
	flags := detailMap(section, "flags")
	if len(flags) == 0 {
		return ""
	}
	var on []string
	for k, v := range flags {
		if v == true {
			on = append(on, k)
		}
	}
	return strings.Join(on, " · ")
}

func diskProfilesLines(section model.Section) [][2]string {
	raw, ok := section.Details["profiles"]
	if !ok || raw == nil {
		return nil
	}
	b, _ := json.Marshal(raw)
	var profiles []map[string]any
	if json.Unmarshal(b, &profiles) != nil {
		return nil
	}
	var rows [][2]string
	for _, p := range profiles {
		name := mapString(p, "name")
		if name == "" {
			continue
		}
		rows = append(rows, [2]string{name + " 读", mapString(p, "read")})
		rows = append(rows, [2]string{name + " 写", mapString(p, "write")})
	}
	return rows
}
