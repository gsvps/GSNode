package probe

import "fmt"

// ipPurityResult is higher-is-better purity (0–100), derived from multi-source risk.
type ipPurityResult struct {
	Percent int    `json:"percent"`
	Text    string `json:"text"`
	Level   string `json:"level"`  // good | warn | bad
	Source  string `json:"source"` // weighted | dnsbl | factors
}

// purityFactors are consensus risk flags used as additive penalties.
// Inspired by residential-IP评估实践：黑名单 + 多源 fraud score + 代理/VPN/Tor 标记。
type purityFactors struct {
	Proxy bool
	VPN   bool
	Tor   bool
	Abuse bool
	Robot bool
}

// Source weights follow common VPS/IP质量检测口径：
// Scamalytics 常被当作「纯净度」主参考；IPQS / AbuseIPDB / ipapi 作交叉校验。
var ipPurityRiskWeights = map[string]int{
	"Scamalytics": 35,
	"IPQS":        30,
	"AbuseIPDB":   20,
	"ipapi":       15,
}

func computeIPPurity(records []ipDBRecord, listed, valid int, factors purityFactors) (ipPurityResult, bool) {
	risk, source, ok := blendIPRisk(records, listed, valid, factors)
	if !ok {
		return ipPurityResult{}, false
	}
	percent := 100 - risk
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	return ipPurityResult{
		Percent: percent,
		Text:    fmt.Sprintf("%d%%", percent),
		Level:   purityLevel(percent),
		Source:  source,
	}, true
}

func purityFactorsFromRecords(records []ipDBRecord) purityFactors {
	return purityFactors{
		Proxy: boolPtrTrue(mergeBool(collectBools(records, func(r ipDBRecord) *bool { return r.Proxy })...)),
		VPN:   boolPtrTrue(mergeBool(collectBools(records, func(r ipDBRecord) *bool { return r.VPN })...)),
		Tor:   boolPtrTrue(mergeBool(collectBools(records, func(r ipDBRecord) *bool { return r.Tor })...)),
		Abuse: boolPtrTrue(mergeBool(collectBools(records, func(r ipDBRecord) *bool { return r.Abuse })...)),
		Robot: boolPtrTrue(mergeBool(collectBools(records, func(r ipDBRecord) *bool { return r.Robot })...)),
	}
}

func collectBools(records []ipDBRecord, get func(ipDBRecord) *bool) []*bool {
	out := make([]*bool, 0, len(records))
	for _, r := range records {
		if v := get(r); v != nil {
			out = append(out, v)
		}
	}
	return out
}

func boolPtrTrue(v *bool) bool {
	return v != nil && *v
}

func blendIPRisk(records []ipDBRecord, listed, valid int, factors purityFactors) (risk int, source string, ok bool) {
	sumW, sumWS, maxScore, n := 0, 0, 0, 0
	for _, rec := range records {
		w, known := ipPurityRiskWeights[rec.Name]
		if !known || !recordHasRiskScore(rec) {
			continue
		}
		s := clampRisk(rec.Score)
		sumW += w
		sumWS += w * s
		if s > maxScore {
			maxScore = s
		}
		n++
	}

	dnsblRisk := 0
	hasDNSBL := valid > 0
	if hasDNSBL {
		dnsblRisk = clampRisk(listed * 100 / valid)
	}

	switch {
	case sumW > 0 && hasDNSBL:
		// 加权均值 + 最高分防「一家很低、一家很高被平均掉」+ DNSBL 占比
		avg := sumWS / sumW
		risk = (avg*65 + maxScore*20 + dnsblRisk*15) / 100
		source = "weighted"
		ok = true
	case sumW > 0:
		avg := sumWS / sumW
		risk = (avg*80 + maxScore*20) / 100
		source = "weighted"
		ok = true
	case hasDNSBL:
		risk = dnsblRisk
		source = "dnsbl"
		ok = true
	default:
		source = "factors"
	}

	penalty := purityFactorPenalty(factors)
	if !ok && penalty == 0 {
		return 0, "", false
	}
	if !ok {
		ok = true
	}
	return clampRisk(risk + penalty), source, true
}

func purityFactorPenalty(f purityFactors) int {
	// 与常见风控口径一致：Tor/代理/VPN 显著拉低纯净度；滥用与机器人次之。
	p := 0
	if f.Tor {
		p += 25
	}
	if f.Proxy {
		p += 15
	}
	if f.VPN {
		p += 12
	}
	if f.Abuse {
		p += 10
	}
	if f.Robot {
		p += 10
	}
	return p
}

func recordHasRiskScore(r ipDBRecord) bool {
	return r.ScoreText != ""
}

func clampRisk(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

// purityLevel 对齐 Scamalytics / IPQuality 风险档：risk<20 低、risk<60 中、否则高。
func purityLevel(percent int) string {
	switch {
	case percent >= 80:
		return "good"
	case percent >= 40:
		return "warn"
	default:
		return "bad"
	}
}
