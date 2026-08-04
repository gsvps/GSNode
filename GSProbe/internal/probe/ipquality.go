package probe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"gsprobe/internal/model"
)

type IPQuality struct{}

func (IPQuality) ID() string   { return "ip-quality" }
func (IPQuality) Name() string { return "IP Quality" }

type ipWhoResponse struct {
	IP            string  `json:"ip"`
	Type          string  `json:"type"`
	Continent     string  `json:"continent"`
	ContinentCode string  `json:"continent_code"`
	Country       string  `json:"country"`
	CountryCode   string  `json:"country_code"`
	Region        string  `json:"region"`
	City          string  `json:"city"`
	Postal        string  `json:"postal"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Connection    struct {
		ASN    int    `json:"asn"`
		Org    string `json:"org"`
		ISP    string `json:"isp"`
		Domain string `json:"domain"`
	} `json:"connection"`
	Timezone struct {
		ID string `json:"id"`
	} `json:"timezone"`
}

var dnsBLZones = []string{
	"zen.spamhaus.org", "bl.spamcop.net", "b.barracudacentral.org", "dnsbl.sorbs.net",
	"psbl.surriel.com", "dnsbl-1.uceprotect.net", "spam.dnsbl.sorbs.net", "cbl.abuseat.org",
	"all.s5h.net", "truncate.gbudb.net", "dnsbl.dronebl.org", "rbl.efnetrbl.org",
}

func (p IPQuality) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	client := ipv4HTTPClient(10 * time.Second)

	log("IP Quality: 查询公网 IPv4")
	who, err := lookupPublicIPv4Who(ctx, client)
	if err != nil || net.ParseIP(who.IP).To4() == nil {
		s.Status = model.StatusWarning
		s.Summary = "无法取得公网 IPv4 信息"
		if err != nil {
			s.Error = err.Error()
		}
		return Finish(s, started)
	}
	ip := who.IP

	log("IP Quality: 内置多源聚合查询")
	bundle := aggregateIPQuality(ctx, client, ip, who)
	records := bundleRecords(bundle)

	log("IP Quality: 检查 DNS 黑名单")
	zones := loadDNSBLZoneList(ctx, client)
	valid, normal, listed, blacklistRows := checkDNSBLParallel(ctx, ip, zones)

	log("IP Quality: 检查邮件端口与服务商")
	portLabel, outbound25 := checkLocalPort25(ctx)
	mailOK, mailRows := checkMailProviders(ctx)
	if !mailOK {
		mailOK = outbound25
	}

	// 与 IPQuality 精简模式一致：使用地优先 IPinfo，注册地取 IPinfo abuse/company country。
	useCountry := firstNonEmpty(records[1].Country, who.CountryCode)
	useCountryName := countryDisplayName(useCountry, who.Country)
	regCountryRaw := records[1].RegCountry
	regCountry := firstNonEmpty(regCountryRaw, who.CountryCode)
	regCountryName := countryDisplayName(regCountry, who.Country)

	mapURL := fmt.Sprintf("https://www.openstreetmap.org/?mlat=%.4f&mlon=%.4f#map=10/%.4f/%.4f", who.Latitude, who.Longitude, who.Latitude, who.Longitude)

	ipType := classifyNativeOrBroadcastIP(useCountry, regCountryRaw)

	proxyText := boolLabel(mergeBool(records[1].Proxy, records[3].Proxy, records[4].Proxy))
	hostingText := boolLabel(mergeBool(records[1].Server, records[3].Server, records[4].Server))
	ipCategories := recordsToIPCategories(records, hostingText, proxyText)
	purity, hasPurity := computeIPPurity(records, listed, valid, purityFactorsFromRecords(records))

	base := map[string]any{
		"ip": maskIP(ip), "rawIP": ip,
		"asn":          fmt.Sprintf("AS%d", who.Connection.ASN),
		"organization": firstNonEmpty(who.Connection.Org),
		"isp":          who.Connection.ISP,
		"coordinates":  fmt.Sprintf("%.4f, %.4f", who.Latitude, who.Longitude),
		"map":          mapURL,
		"city":         strings.Trim(strings.Join([]string{who.Region, who.City, who.Postal}, ", "), ", "),
		"country":      formatRegion(useCountry, useCountryName),
		"regCountry":   formatRegion(regCountry, regCountryName),
		"timezone":     firstNonEmpty(who.Timezone.ID),
		"ipType":       ipType,
		"port25Local":  portLabel,
		"ipCategories": ipCategories,
	}

	details := map[string]any{
		"base":         base,
		"sources":      recordsToSources(records),
		"riskScores":   recordsToRiskScores(records),
		"riskFactors":  recordsToRiskFactors(records),
		"blacklist":    map[string]any{"valid": valid, "normal": normal, "listed": listed, "results": blacklistRows},
		"mail":         map[string]any{"outbound25": mailOK, "localPort25": portLabel, "results": mailRows},
		"sourceMethod": "GSProbe 内置聚合 · ipinfo.io / ipapi.is / ipregistry.co / ip-api.com / DB-IP",
	}
	if hasPurity {
		details["ipPurity"] = map[string]any{
			"percent": purity.Percent,
			"text":    purity.Text,
			"level":   purity.Level,
			"source":  purity.Source,
		}
	}
	s.Details = details

	s.Metrics = []model.Metric{
		{Name: "公网 IP", Text: maskIP(ip)},
		{Name: "自治系统", Text: fmt.Sprintf("AS%d", who.Connection.ASN)},
		{Name: "组织", Text: who.Connection.Org},
		{Name: "位置", Text: strings.TrimSpace(useCountry + " " + who.City)},
		{Name: "代理", Text: proxyText},
		{Name: "机房", Text: hostingText},
		{Name: "DNSBL", Text: fmt.Sprintf("%d/%d 已标记", listed, valid)},
		{Name: "25端口", Text: map[bool]string{true: "可用", false: portLabel}[mailOK]},
	}
	if hasPurity {
		s.Metrics = append(s.Metrics, model.Metric{
			Name: "IP纯净度", Text: purity.Text, Value: float64(purity.Percent), Unit: "%",
		})
	}

	s.Score = 10000 - listed*120
	for _, r := range records {
		if r.Score >= 75 {
			s.Score -= 800
		} else if r.Score >= 45 {
			s.Score -= 400
		}
	}
	if proxyText == "是" {
		s.Score -= 1200
	}
	if s.Score < 0 {
		s.Score = 0
	}

	s.Summary = fmt.Sprintf("%s · %s · 黑名单 %d/%d · 25端口%s", maskIP(ip), who.Connection.Org, listed, valid, map[bool]string{true: "可用", false: "不可用"}[mailOK])
	if listed > 0 || proxyText == "是" {
		s.Status = model.StatusWarning
	}
	return Finish(s, started)
}

func getJSON(ctx context.Context, client *http.Client, url string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "GSProbe/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d", url, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(target)
}

func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func maskIP(value string) string {
	ip := net.ParseIP(value)
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.*.*", v4[0], v4[1])
	}
	if ip != nil {
		first := uint16(ip[0])<<8 | uint16(ip[1])
		second := uint16(ip[2])<<8 | uint16(ip[3])
		return fmt.Sprintf("%x:%x:*:*:*:*:*:*", first, second)
	}
	return value
}

// legacy helper kept for tests
func checkDNSBL(ctx context.Context, address string) (valid, normal, listed int, rows []map[string]any) {
	return checkDNSBLParallel(ctx, address, dnsBLZones)
}

func checkMailPort(ctx context.Context) (bool, []map[string]any) {
	ok, rows := checkMailProviders(ctx)
	if ok {
		return true, rows
	}
	_, outbound := checkLocalPort25(ctx)
	return outbound, rows
}
