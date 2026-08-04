package probe

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// ipQualityBundle holds raw responses from GSProbe's built-in multi-source aggregator.
type ipQualityBundle struct {
	Who        ipWhoResponse
	IPInfo     map[string]any
	IPRegistry map[string]any
	IPAPIis    map[string]any
	IPAPIcom   map[string]any
	Scam       map[string]any
	Abuse      map[string]any
	IP2Loc     map[string]any
	IPQS       map[string]any
	DBIP       map[string]any
}

func aggregateIPQuality(ctx context.Context, client *http.Client, ip string, who ipWhoResponse) ipQualityBundle {
	var b ipQualityBundle
	b.Who = who
	var wg sync.WaitGroup
	run := func(fn func()) { wg.Add(1); go func() { defer wg.Done(); fn() }() }
	run(func() { b.IPInfo = fetchIPInfoWidget(ctx, client, ip) })
	run(func() { b.IPRegistry = fetchIPRegistry(ctx, client, ip) })
	run(func() { b.IPAPIis = fetchIPAPIis(ctx, client, ip) })
	run(func() { b.IPAPIcom = fetchIPAPIcom(ctx, client, ip) })
	run(func() { b.Scam = fetchScamalyticsDirect(ctx, client, ip) })
	run(func() { b.Abuse = fetchAbuseIPDBDirect(ctx, client, ip) })
	run(func() { b.IP2Loc = fetchIP2LocationDirect(ctx, client, ip) })
	run(func() { b.IPQS = fetchIPQSDirect(ctx, client, ip) })
	run(func() { b.DBIP = fetchDBIP(ctx, client, ip) })
	wg.Wait()
	return b
}

func bundleRecords(b ipQualityBundle) []ipDBRecord {
	return []ipDBRecord{
		parseWhoRecord(b.Who),
		parseIPInfoWidget(b.IPInfo),
		parseIPRegistry(b.IPRegistry),
		parseIPAPIis(b.IPAPIis),
		parseIPAPIcom(b.IPAPIcom),
		parseScamalyticsFromMap(b.Scam),
		parseAbuseIPDBFromMap(b.Abuse),
		parseIP2LocationFromMap(b.IP2Loc),
		parseIPQSFromMap(b.IPQS),
		parseDBIP(b.DBIP),
	}
}

func parseWhoRecord(who ipWhoResponse) ipDBRecord {
	return ipDBRecord{
		Name:    "Maxmind",
		Country: fmt.Sprintf("[%s] %s", who.CountryCode, who.Country),
		City: strings.Trim(strings.Join([]string{who.Region, who.City, who.Postal}, ", "), ", "),
		Region:  who.CountryCode,
		ASN:     fmt.Sprintf("AS%d", who.Connection.ASN),
		Org:     who.Connection.Org,
	}
}

func fetchIPAPIcom(ctx context.Context, client *http.Client, ip string) map[string]any {
	fields := "status,country,countryCode,regionName,city,lat,lon,timezone,isp,org,as,proxy,hosting,mobile,query"
	return fetchJSON(ctx, client, "http://ip-api.com/json/"+ip+"?fields="+fields, nil)
}

func parseIPAPIcom(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "ip-api.com"}
	if raw == nil || jsonStr(raw, "status") != "success" {
		return r
	}
	r.Region = jsonStr(raw, "countryCode")
	r.Country = jsonStr(raw, "countryCode")
	r.City = jsonStr(raw, "city")
	r.ASN = jsonStr(raw, "as")
	r.Org = firstNonEmpty(jsonStr(raw, "org"), jsonStr(raw, "isp"))
	r.Proxy = jsonBool(raw, "proxy")
	r.Server = jsonBool(raw, "hosting")
	if hosting := jsonBool(raw, "hosting"); hosting != nil {
		if *hosting {
			r.UseType = "机房"
		} else {
			r.UseType = "ISP"
		}
		r.CompanyType = r.UseType
	}
	if mobile := jsonBool(raw, "mobile"); mobile != nil && *mobile {
		r.UseType = "移动 ISP"
	}
	return r
}

func fetchScamalyticsDirect(ctx context.Context, client *http.Client, ip string) map[string]any {
	html := fetchText(ctx, client, "https://scamalytics.com/ip/"+ip, nil)
	if html == "" {
		return nil
	}
	out := map[string]any{}
	if score, ok := parseScamalyticsScore(html); ok {
		out["score"] = strconv.Itoa(score)
	}
	if m := regexp.MustCompile(`(?i)isp[^<]{0,20}</[^>]+>\s*<[^>]+>([^<]+)`).FindStringSubmatch(html); len(m) == 2 {
		out["org"] = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`(?i)country[^<]{0,20}</[^>]+>\s*<[^>]+>([^<]+)`).FindStringSubmatch(html); len(m) == 2 {
		out["country"] = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`(?i)city[^<]{0,20}</[^>]+>\s*<[^>]+>([^<]+)`).FindStringSubmatch(html); len(m) == 2 {
		out["city"] = strings.TrimSpace(m[1])
	}
	out["proxy"] = regexp.MustCompile(`(?i)(?:is a proxy|proxy ip|proxy detected)`).MatchString(html)
	out["vpn"] = regexp.MustCompile(`(?i)(?:is a vpn|vpn ip|vpn detected)`).MatchString(html)
	out["tor"] = regexp.MustCompile(`(?i)(?:tor exit|is tor|detected as tor)`).MatchString(html)
	out["hosting"] = regexp.MustCompile(`(?i)(?:data center|datacenter|hosting provider)`).MatchString(html)
	return out
}

// parseScamalyticsScore extracts Fraud Score from Scamalytics HTML.
// Prefer "Fraud Score: N" / JSON "score" — never the loose fraud…digit pattern that
// matches "</h1>" after "Fraud Risk" and always yields 1.
func parseScamalyticsScore(html string) (int, bool) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)Fraud\s*Score\s*:\s*(\d{1,3})`),
		regexp.MustCompile(`(?i)scamalytics_score["'\s:=]+(\d{1,3})`),
		regexp.MustCompile(`"score"\s*:\s*"?(\d{1,3})"?`),
	}
	for _, re := range patterns {
		m := re.FindStringSubmatch(html)
		if len(m) < 2 {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil || n < 0 || n > 100 {
			continue
		}
		return n, true
	}
	return 0, false
}

func fetchAbuseIPDBDirect(ctx context.Context, client *http.Client, ip string) map[string]any {
	html := fetchText(ctx, client, "https://www.abuseipdb.com/check/"+ip, map[string]string{"Accept": "text/html"})
	if html == "" {
		return nil
	}
	out := map[string]any{}
	if m := regexp.MustCompile(`abuseConfidenceScore["'\s:=]+(\d+)`).FindStringSubmatch(html); len(m) == 2 {
		out["score"] = m[1]
	}
	if m := regexp.MustCompile(`usageType["'\s:=]+"([^"]+)"`).FindStringSubmatch(html); len(m) == 2 {
		out["usageType"] = m[1]
	}
	if m := regexp.MustCompile(`(?i)country[^<]{0,30}</[^>]+>\s*<[^>]+>([^<]+)`).FindStringSubmatch(html); len(m) == 2 {
		out["country"] = strings.TrimSpace(m[1])
	}
	if m := regexp.MustCompile(`(?i)isp[^<]{0,30}</[^>]+>\s*<[^>]+>([^<]+)`).FindStringSubmatch(html); len(m) == 2 {
		out["org"] = strings.TrimSpace(m[1])
	}
	return out
}

func fetchIP2LocationDirect(ctx context.Context, client *http.Client, ip string) map[string]any {
	endpoints := []string{
		"https://api.ip2location.io/?ip=" + ip + "&key=demo",
		"https://api.ip2location.io/v2/?ip=" + ip,
	}
	for _, url := range endpoints {
		raw := fetchJSON(ctx, client, url, nil)
		if raw != nil && jsonStr(raw, "ip") != "" {
			return raw
		}
	}
	return nil
}

func fetchIPQSDirect(ctx context.Context, client *http.Client, ip string) map[string]any {
	html := fetchText(ctx, client, "https://www.ipqualityscore.com/ip-risk-check/lookup/"+ip, nil)
	if html == "" {
		return nil
	}
	out := map[string]any{}
	if m := regexp.MustCompile(`fraud_score["'\s:=]+(\d+)`).FindStringSubmatch(html); len(m) == 2 {
		out["fraud_score"] = m[1]
	}
	if m := regexp.MustCompile(`"fraud_score":(\d+)`).FindStringSubmatch(html); len(m) == 2 {
		out["fraud_score"] = m[1]
	}
	low := strings.ToLower(html)
	out["proxy"] = strings.Contains(low, `"proxy":true`) || strings.Contains(low, "proxy detected")
	out["vpn"] = strings.Contains(low, `"vpn":true`)
	out["tor"] = strings.Contains(low, `"tor":true`)
	return out
}

func parseScamalyticsFromMap(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "Scamalytics"}
	if raw == nil {
		return r
	}
	if s, err := strconv.Atoi(jsonStr(raw, "score")); err == nil {
		r.Score = s
		r.ScoreText = scoreText(s, riskLabelCN(s))
	}
	r.Country = normalizeCountryCode(jsonStr(raw, "country"))
	r.City = jsonStr(raw, "city")
	r.Org = jsonStr(raw, "org")
	if r.Country != "" {
		r.Region = r.Country
	}
	if v, ok := raw["proxy"].(bool); ok {
		r.Proxy = &v
	}
	if v, ok := raw["vpn"].(bool); ok {
		r.VPN = &v
	}
	if v, ok := raw["tor"].(bool); ok {
		r.Tor = &v
	}
	if v, ok := raw["hosting"].(bool); ok {
		r.Server = &v
	}
	return r
}

func parseAbuseIPDBFromMap(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "AbuseIPDB"}
	if raw == nil {
		return r
	}
	r.UseType = usageLabel(jsonStr(raw, "usageType"))
	r.Country = normalizeCountryCode(jsonStr(raw, "country"))
	r.Org = jsonStr(raw, "org")
	if r.Country != "" {
		r.Region = r.Country
	}
	if s, err := strconv.Atoi(jsonStr(raw, "score")); err == nil {
		r.Score = s
		r.ScoreText = scoreText(s, riskLabelCN(s))
	}
	return r
}

func parseIP2LocationFromMap(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "IP2Location"}
	if raw == nil {
		return r
	}
	r.Region = jsonStr(raw, "country_code")
	r.Country = jsonStr(raw, "country_name")
	r.City = jsonStr(raw, "city_name")
	r.UseType = usageLabel(jsonStr(raw, "usage_type"))
	if s, err := strconv.Atoi(jsonStr(raw, "fraud_score")); err == nil {
		r.Score = s
		r.ScoreText = scoreText(s, riskLabelCN(s))
	}
	r.Proxy = jsonBool(raw, "is_proxy")
	return r
}

func parseIPQSFromMap(raw map[string]any) ipDBRecord {
	r := ipDBRecord{Name: "IPQS"}
	if raw == nil {
		return r
	}
	if s, err := strconv.Atoi(jsonStr(raw, "fraud_score")); err == nil {
		r.Score = s
		r.ScoreText = scoreText(s, riskLabelCN(s))
	}
	if v, ok := raw["proxy"].(bool); ok {
		r.Proxy = &v
	}
	if v, ok := raw["vpn"].(bool); ok {
		r.VPN = &v
	}
	if v, ok := raw["tor"].(bool); ok {
		r.Tor = &v
	}
	return r
}
