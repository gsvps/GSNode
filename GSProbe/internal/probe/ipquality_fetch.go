package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"
)

const ipQualityUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 GSProbe/0.1"

func fetchJSON(ctx context.Context, client *http.Client, url string, headers map[string]string) map[string]any {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", ipQualityUA)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	var out map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&out); err != nil {
		return nil
	}
	return out
}

func fetchText(ctx context.Context, client *http.Client, url string, headers map[string]string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", ipQualityUA)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return ""
	}
	return string(b)
}

func fetchIPInfoWidget(ctx context.Context, client *http.Client, ip string) map[string]any {
	return fetchJSON(ctx, client, "https://ipinfo.io/widget/demo/"+ip, nil)
}

func fetchIPAPIis(ctx context.Context, client *http.Client, ip string) map[string]any {
	return fetchJSON(ctx, client, "https://api.ipapi.is/?q="+ip, nil)
}

func fetchIPRegistry(ctx context.Context, client *http.Client, ip string) map[string]any {
	key := "sb69ksjcajfs4c"
	if html := fetchText(ctx, client, "https://ipregistry.co/", map[string]string{"Accept": "text/html"}); html != "" {
		if m := regexp.MustCompile(`apiKey="([a-zA-Z0-9]+)"`).FindStringSubmatch(html); len(m) == 2 {
			key = m[1]
		}
	}
	url := fmt.Sprintf("https://api.ipregistry.co/%s?hostname=true&key=%s", ip, key)
	return fetchJSON(ctx, client, url, map[string]string{
		"origin":  "https://ipregistry.co",
		"referer": "https://ipregistry.co/",
	})
}

func fetchDBIP(ctx context.Context, client *http.Client, ip string) map[string]any {
	html := fetchText(ctx, client, "https://db-ip.com/"+ip, nil)
	if html == "" {
		return nil
	}
	out := map[string]any{}
	if m := regexp.MustCompile(`Estimated threat level for this IP address is\s*<span[^>]*>([^<]+)`).FindStringSubmatch(html); len(m) == 2 {
		out["threat"] = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`"countryCode"\s*:\s*"([^"]+)"`).FindStringSubmatch(html); len(m) == 2 {
		out["countryCode"] = m[1]
	}
	return out
}

func jsonPath(v any, keys ...string) any {
	cur := v
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = m[key]
		if !ok {
			return nil
		}
	}
	return cur
}

func jsonStr(v any, keys ...string) string {
	cur := jsonPath(v, keys...)
	if cur == nil {
		return ""
	}
	switch t := cur.(type) {
	case string:
		return t
	case float64:
		if t == float64(int(t)) {
			return fmt.Sprintf("%d", int(t))
		}
		return fmt.Sprintf("%v", t)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", t)
	}
}

func jsonBool(v any, keys ...string) *bool {
	cur := jsonPath(v, keys...)
	if cur == nil {
		return nil
	}
	switch t := cur.(type) {
	case bool:
		return &t
	case string:
		switch strings.ToLower(t) {
		case "true", "yes", "是":
			b := true
			return &b
		case "false", "no", "否":
			b := false
			return &b
		}
	}
	return nil
}

func boolLabel(v *bool) string {
	if v == nil {
		return "无"
	}
	if *v {
		return "是"
	}
	return "否"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" && value != "null" {
			return value
		}
	}
	return ""
}

func mergeBool(labels ...*bool) *bool {
	set := false
	val := false
	for _, b := range labels {
		if b == nil {
			continue
		}
		set = true
		if *b {
			val = true
		}
	}
	if !set {
		return nil
	}
	return &val
}

func usageLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "hosting", "data center/web hosting/transit", "dch":
		return "机房"
	case "isp", "fixed line isp", "mobile isp":
		return "ISP"
	case "business", "commercial", "com":
		return "商业"
	case "education", "edu", "university/college/school":
		return "教育"
	case "government", "gov":
		return "政府"
	case "organization", "org":
		return "组织"
	case "content delivery network", "cdn":
		return "CDN"
	case "search engine spider", "ses":
		return "爬虫"
	case "reserved", "rsv":
		return "保留"
	default:
		if raw == "" || raw == "null" {
			return "无数据"
		}
		return raw
	}
}

func riskLabel(score int) string {
	switch {
	case score >= 90:
		return "极高风险"
	case score >= 75:
		return "高风险"
	case score >= 45:
		return "中风险"
	case score >= 20:
		return "略高风险"
	default:
		return "低风险"
	}
}

func riskLabelCN(score int) string {
	switch {
	case score >= 90:
		return "极高风险"
	case score >= 75:
		return "高风险"
	case score >= 45:
		return "中风险"
	case score >= 20:
		return "略高风险"
	default:
		return "低风险"
	}
}

func scoreText(score int, label string) string {
	if label == "" {
		label = riskLabelCN(score)
	}
	return fmt.Sprintf("%d | %s", score, label)
}

func formatRegion(code, name string) string {
	c := normalizeCountryCode(firstNonEmpty(code, name))
	if c == "" {
		return "—"
	}
	if cn := countryDisplayName(c, name); cn != "" {
		return fmt.Sprintf("[%s] %s", c, cn)
	}
	return fmt.Sprintf("[%s]", c)
}

func formatCountryCode(raw string) string {
	c := normalizeCountryCode(raw)
	if c == "" {
		return "—"
	}
	if cn := countryDisplayName(c, raw); cn != "" {
		return fmt.Sprintf("[%s] %s", c, cn)
	}
	return fmt.Sprintf("[%s]", c)
}

func normalizeCountryCode(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "—" {
		return ""
	}
	if m := regexp.MustCompile(`(?i)\[([A-Za-z]{2,3})\]`).FindStringSubmatch(raw); len(m) == 2 {
		return strings.ToUpper(m[1])
	}
	up := strings.ToUpper(raw)
	if len(up) == 2 || len(up) == 3 {
		if up == raw || raw == strings.ToUpper(raw) {
			return up
		}
	}
	lower := strings.ToLower(raw)
	for name, code := range countryNameToCode {
		if lower == name || strings.Contains(lower, name) {
			return code
		}
	}
	return up
}

var countryNameToCode = map[string]string{
	"united states": "US", "美国": "US", "usa": "US",
	"hong kong": "HK", "香港": "HK",
	"united kingdom": "GB", "英国": "GB", "great britain": "GB",
	"japan": "JP", "日本": "JP",
	"china": "CN", "中国": "CN",
	"taiwan": "TW", "台湾": "TW",
	"singapore": "SG", "新加坡": "SG",
	"germany": "DE", "德国": "DE",
	"france": "FR", "法国": "FR",
	"canada": "CA", "加拿大": "CA",
	"australia": "AU", "澳大利亚": "AU",
	"netherlands": "NL", "荷兰": "NL",
	"south korea": "KR", "韩国": "KR", "korea": "KR",
	"india": "IN", "印度": "IN",
	"brazil": "BR", "巴西": "BR",
	"russia": "RU", "俄罗斯": "RU",
	"italy": "IT", "意大利": "IT",
	"spain": "ES", "西班牙": "ES",
	"mexico": "MX", "墨西哥": "MX",
	"thailand": "TH", "泰国": "TH",
	"vietnam": "VN", "越南": "VN",
	"malaysia": "MY", "马来西亚": "MY",
	"indonesia": "ID", "印度尼西亚": "ID",
	"philippines": "PH", "菲律宾": "PH",
	"ireland": "IE", "爱尔兰": "IE",
	"sweden": "SE", "瑞典": "SE",
	"norway": "NO", "挪威": "NO",
	"finland": "FI", "芬兰": "FI",
	"poland": "PL", "波兰": "PL",
	"türkiye": "TR", "turkey": "TR", "土耳其": "TR",
	"uae": "AE", "united arab emirates": "AE",
	"saudi arabia": "SA", "沙特": "SA",
}

var countryCodeToCN = CountryCodeToCN

func countryDisplayName(code, fallback string) string {
	c := normalizeCountryCode(firstNonEmpty(code, fallback))
	if c != "" {
		if cn := countryCodeToCN[c]; cn != "" {
			return cn
		}
	}
	fallback = strings.TrimSpace(fallback)
	if fallback == "" {
		return ""
	}
	if containsHan(fallback) {
		return fallback
	}
	if cn := countryCodeToCN[normalizeCountryCode(fallback)]; cn != "" {
		return cn
	}
	lower := strings.ToLower(fallback)
	for name, cc := range countryNameToCode {
		if lower == name {
			if cn := countryCodeToCN[cc]; cn != "" {
				return cn
			}
		}
	}
	return fallback
}

func containsHan(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
