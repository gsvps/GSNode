package probe

import (
	"math"
	"strconv"
	"strings"
)

type ipDBRecord struct {
	Name        string
	UseType     string
	CompanyType string
	Country     string
	City        string
	ASN         string
	Org         string
	RegCountry  string
	Score       int
	ScoreText   string
	Region      string
	Proxy       *bool
	Tor         *bool
	VPN         *bool
	Server      *bool
	Abuse       *bool
	Robot       *bool
}

func parseIPInfoWidget(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "IPinfo"}
	if raw == nil {
		return r
	}
	data, _ := raw["data"].(map[string]any)
	if data == nil {
		return r
	}
	r.UseType = usageLabel(jsonStr(data, "asn", "type"))
	r.CompanyType = usageLabel(jsonStr(data, "company", "type"))
	r.Country = jsonStr(data, "country")
	r.City = jsonStr(data, "city")
	r.ASN = jsonStr(data, "asn", "asn")
	if r.ASN != "" && !strings.HasPrefix(r.ASN, "AS") {
		r.ASN = "AS" + strings.TrimPrefix(r.ASN, "AS")
	}
	r.Org = jsonStr(data, "asn", "name")
	r.Region = jsonStr(data, "country")
	r.RegCountry = firstNonEmpty(jsonStr(data, "abuse", "country"), jsonStr(data, "company", "country"))
	r.Proxy = jsonBool(data, "privacy", "proxy")
	r.Tor = jsonBool(data, "privacy", "tor")
	r.VPN = jsonBool(data, "privacy", "vpn")
	r.Server = jsonBool(data, "privacy", "hosting")
	return r
}

func parseIPRegistry(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "ipregistry"}
	if raw == nil {
		return r
	}
	r.UseType = usageLabel(jsonStr(raw, "connection", "type"))
	r.CompanyType = usageLabel(jsonStr(raw, "company", "type"))
	r.Country = jsonStr(raw, "location", "country", "code")
	r.City = jsonStr(raw, "location", "city")
	r.ASN = jsonStr(raw, "connection", "asn")
	r.Org = jsonStr(raw, "company", "name")
	r.Region = jsonStr(raw, "location", "country", "code")
	r.Proxy = jsonBool(raw, "security", "is_proxy")
	r.Tor = mergeBool(jsonBool(raw, "security", "is_tor"), jsonBool(raw, "security", "is_tor_exit"))
	r.VPN = jsonBool(raw, "security", "is_vpn")
	r.Server = jsonBool(raw, "security", "is_cloud_provider")
	r.Abuse = jsonBool(raw, "security", "is_abuser")
	return r
}

func parseIPAPIis(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "ipapi"}
	if raw == nil {
		return r
	}
	r.UseType = usageLabel(jsonStr(raw, "asn", "type"))
	r.CompanyType = usageLabel(jsonStr(raw, "company", "type"))
	r.Country = jsonStr(raw, "location", "country_code")
	r.City = jsonStr(raw, "location", "city")
	r.ASN = jsonStr(raw, "asn", "asn")
	r.Org = firstNonEmpty(jsonStr(raw, "company", "name"), jsonStr(raw, "asn", "org"))
	r.Region = jsonStr(raw, "location", "country_code")
	scoreText := jsonStr(raw, "company", "abuser_score")
	if parts := strings.Fields(scoreText); len(parts) > 0 {
		if f, err := strconv.ParseFloat(parts[0], 64); err == nil {
			r.Score = int(math.Round(f * 100))
			if r.Score > 100 {
				r.Score = int(f)
			}
		}
	}
	if r.Score > 0 {
		r.ScoreText = scoreText
	}
	r.Proxy = jsonBool(raw, "is_proxy")
	r.Tor = jsonBool(raw, "is_tor")
	r.VPN = jsonBool(raw, "is_vpn")
	r.Server = jsonBool(raw, "is_datacenter")
	r.Abuse = jsonBool(raw, "is_abuser")
	r.Robot = jsonBool(raw, "is_crawler")
	return r
}

func parseDBIP(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "DB-IP"}
	if raw == nil {
		return r
	}
	r.Region = normalizeCountryCode(jsonStr(raw, "countryCode"))
	threat := strings.ToLower(jsonStr(raw, "threat"))
	switch threat {
	case "high":
		r.Score = 100
		r.ScoreText = "100 | 高风险"
	case "medium":
		r.Score = 50
		r.ScoreText = "50 | 中风险"
	case "low":
		r.Score = 0
		r.ScoreText = "0 | 低风险"
	}
	return r
}

func recordsToSources(records []ipDBRecord) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, r := range records {
		if r.Name == "Maxmind" || !recordHasGeo(r) {
			continue
		}
		out = append(out, map[string]any{
			"name": r.Name, "type": displayField(r.UseType), "companyType": displayField(r.CompanyType),
			"country": formatCountryCode(firstNonEmpty(r.Region, r.Country)), "city": r.City, "asn": r.ASN, "org": r.Org,
		})
	}
	return out
}

func recordsToRiskScores(records []ipDBRecord) map[string]any {
	out := map[string]any{}
	for _, r := range records {
		if r.ScoreText != "" {
			out[r.Name] = r.ScoreText
		} else if r.Score > 0 {
			out[r.Name] = scoreText(r.Score, riskLabelCN(r.Score))
		}
	}
	if len(out) == 0 {
		out["提示"] = "数据源暂不可用"
	}
	return out
}

func recordsToRiskFactors(records []ipDBRecord) map[string]any {
	rows := []string{"地区", "代理", "Tor", "VPN", "服务器", "滥用", "机器人"}
	out := map[string]any{}
	active := activeRiskRecords(records)
	for _, row := range rows {
		rowMap := map[string]any{}
		for _, r := range active {
			switch row {
			case "地区":
				if v := firstNonEmpty(r.Region, r.Country); v != "" {
					rowMap[r.Name] = formatCountryCode(v)
				}
			case "代理":
				if r.Proxy != nil {
					rowMap[r.Name] = boolLabel(r.Proxy)
				}
			case "Tor":
				if r.Tor != nil {
					rowMap[r.Name] = boolLabel(r.Tor)
				}
			case "VPN":
				if r.VPN != nil {
					rowMap[r.Name] = boolLabel(r.VPN)
				}
			case "服务器":
				if r.Server != nil {
					rowMap[r.Name] = boolLabel(r.Server)
				}
			case "滥用":
				if r.Abuse != nil {
					rowMap[r.Name] = boolLabel(r.Abuse)
				}
			case "机器人":
				if r.Robot != nil {
					rowMap[r.Name] = boolLabel(r.Robot)
				}
			}
		}
		if len(rowMap) > 0 {
			out[row] = rowMap
		}
	}
	return out
}

func recordHasGeo(r ipDBRecord) bool {
	if displayField(r.UseType) != "—" || displayField(r.CompanyType) != "—" {
		return true
	}
	return r.Country != "" || r.City != "" || r.ASN != "" || r.Org != ""
}

func activeRiskRecords(records []ipDBRecord) []ipDBRecord {
	out := make([]ipDBRecord, 0, len(records))
	for _, r := range records {
		if r.Name == "Maxmind" {
			continue
		}
		if recordHasGeo(r) || r.ScoreText != "" || r.Score > 0 ||
			r.Proxy != nil || r.Tor != nil || r.VPN != nil || r.Server != nil || r.Abuse != nil || r.Robot != nil {
			out = append(out, r)
		}
	}
	return out
}

func displayField(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "无数据" || v == "null" {
		return "—"
	}
	return v
}

func recordsToIPCategories(records []ipDBRecord, hosting, proxy string) []map[string]any {
	votes := map[string]int{}
	for _, r := range records {
		for _, raw := range []string{r.UseType, r.CompanyType} {
			switch usageLabel(raw) {
			case "机房":
				votes["机房IP"]++
			case "ISP":
				votes["家宽IP"]++
			case "移动 ISP":
				votes["移动IP"]++
			case "教育":
				votes["教育IP"]++
			case "商业":
				votes["商业IP"]++
			case "政府":
				votes["政府IP"]++
			case "CDN":
				votes["CDN"]++
			case "爬虫":
				votes["爬虫IP"]++
			case "组织":
				votes["组织IP"]++
			}
		}
		if r.Server != nil && *r.Server {
			votes["机房IP"]++
		}
		if r.Proxy != nil && *r.Proxy {
			votes["代理IP"]++
		}
	}
	if hosting == "是" {
		votes["机房IP"] += 2
	}
	if proxy == "是" {
		votes["代理IP"] += 2
	}

	// 家宽 vs 机房互斥：取票数高者，避免多源分歧同时点亮
	if votes["机房IP"] > 0 && votes["家宽IP"] > 0 {
		if votes["机房IP"] >= votes["家宽IP"] {
			votes["家宽IP"] = 0
		} else {
			votes["机房IP"] = 0
		}
	}
	if votes["移动IP"] > 0 && votes["家宽IP"] > 0 && votes["家宽IP"] >= votes["移动IP"] {
		votes["移动IP"] = 0
	}

	labels := []struct {
		label string
		kind  string
	}{
		{"家宽IP", "ok"},
		{"机房IP", "warn"},
		{"移动IP", "ok"},
		{"教育IP", "ok"},
		{"商业IP", "muted"},
		{"政府IP", "muted"},
		{"CDN", "warn"},
		{"代理IP", "bad"},
		{"爬虫IP", "bad"},
		{"组织IP", "muted"},
	}
	out := make([]map[string]any, 0, len(labels))
	for _, item := range labels {
		out = append(out, map[string]any{
			"label":  item.label,
			"active": votes[item.label] > 0,
			"kind":   item.kind,
		})
	}
	return out
}
