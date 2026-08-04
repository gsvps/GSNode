package probe

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"gsprobe/internal/model"
)

type Route struct{}

func (Route) ID() string   { return "route" }
func (Route) Name() string { return "Route" }

type routeTarget struct {
	City    string
	Carrier string
	Address string
	PingIPs []string
}

type routeMapNode struct {
	Region string `json:"region,omitempty"`
	Label  string `json:"label"`
}

type routeResult struct {
	City      string         `json:"city"`
	Carrier   string         `json:"carrier"`
	Address   string         `json:"address"`
	Direction string         `json:"direction"`
	Line      string         `json:"line"`
	Labels    []string       `json:"labels,omitempty"`
	Path      []routeMapNode `json:"path,omitempty"`
	Hops      []string       `json:"hops,omitempty"`
	PingMS    float64        `json:"pingMs,omitempty"`
	PingText  string         `json:"pingText"`
	Output    string         `json:"output"`
	Error     string         `json:"error,omitempty"`
}

func buildRouteTargets(db *PingTargetDB) []routeTarget {
	cities := []string{"北京", "上海", "广州"}
	out := make([]routeTarget, 0, len(cities)*len(chinaCarrierNames))
	for _, city := range cities {
		for _, carrier := range chinaCarrierNames {
			ips := db.RoutePingIPs(city, carrier)
			addr := ""
			if len(ips) > 0 {
				addr = ips[0]
			}
			out = append(out, routeTarget{City: city, Carrier: carrier, Address: addr, PingIPs: ips})
		}
	}
	return out
}

func (p Route) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	tool := routeTool()
	installMessage := ""
	_, hasNextTrace := exec.LookPath("nexttrace")
	if runtime.GOOS == "linux" && hasNextTrace != nil {
		log("Route: 未找到 NextTrace，正在自动安装以生成线路识别和线路图")
		installCtx, installCancel := context.WithTimeout(ctx, 90*time.Second)
		err := installRouteTool(installCtx)
		installCancel()
		if err != nil {
			installMessage = err.Error()
			log("Route: 自动安装失败: " + installMessage)
		} else {
			log("Route: 路由工具安装完成")
			tool = routeTool()
		}
	}
	if tool == "" {
		s.Status = model.StatusSkipped
		s.Summary = "未找到路由探测工具"
		if installMessage != "" {
			s.Error = "自动安装失败: " + installMessage
		} else if runtime.GOOS != "linux" {
			s.Error = "请安装 nexttrace、tracepath、traceroute 或 mtr"
		}
		return Finish(s, started)
	}
	if runtime.GOOS == "linux" && !commandExists("ping") {
		log("Route: 未找到 Ping 工具，正在自动安装 iputils-ping")
		installCtx, installCancel := context.WithTimeout(ctx, 90*time.Second)
		pingInstallErr := installPingTool(installCtx)
		installCancel()
		if pingInstallErr != nil {
			log("Route: Ping 工具自动安装失败: " + pingInstallErr.Error())
		} else {
			log("Route: Ping 工具安装完成")
		}
	}
	pingDB, pingDBErr := LoadPingTargetDB(opts.DataDir)
	if pingDBErr != nil {
		log("Route: Ping 目标库加载失败，使用内置默认: " + pingDBErr.Error())
		pingDB, _ = parsePingTargetDB(embeddedPingTargets)
	}
	log(fmt.Sprintf("Route: Ping 目标库 %s", pingDB.Version))
	targets := buildRouteTargets(pingDB)
	perRoute := 40 * time.Second
	ctx, cancel := context.WithTimeout(ctx, time.Duration(len(targets))*perRoute+30*time.Second)
	defer cancel()
	results := make([]routeResult, 0, len(targets))
	for _, target := range targets {
		log(fmt.Sprintf("Route: 回程 %s%s", target.City, target.Carrier))
		results = append(results, runChinaRoute(ctx, tool, target, pingDB))
	}
	success := 0
	pingSuccess := 0
	metrics := make([]model.Metric, 0, len(results))
	for _, result := range results {
		if result.Output != "" && result.Error == "" {
			success++
		}
		if result.PingMS > 0 {
			pingSuccess++
		}
		metrics = append(metrics, model.Metric{Name: result.City + result.Carrier, Text: result.Line})
		if result.PingText != "" {
			metrics[len(metrics)-1].Text += " · " + result.PingText
		}
	}
	s.Metrics = metrics
	details := map[string]any{
		"tool":         tool,
		"returnRoutes": results,
		"pingTargets": map[string]string{
			"version":   pingDB.Version,
			"updatedAt": pingDB.UpdatedAt,
		},
	}
	s.Summary = fmt.Sprintf("三地三网回程 %d/%d，Ping %d/%d", success, len(results), pingSuccess, len(results))
	s.Details = details
	routeScore := int((float64(success) + float64(pingSuccess)) / float64(len(results)*2) * 10000)
	s.Score = routeScore
	if success < len(results) || pingSuccess < len(results) {
		s.Status = model.StatusWarning
	}
	return Finish(s, started)
}

func routeTool() string {
	if path, err := exec.LookPath("nexttrace"); err == nil {
		return path
	} else if runtime.GOOS == "windows" {
		return "cmd"
	} else if path, err := exec.LookPath("tracepath"); err == nil {
		return path
	} else if path, err := exec.LookPath("traceroute"); err == nil {
		return path
	} else if path, err := exec.LookPath("mtr"); err == nil {
		return path
	} else if path, err := exec.LookPath("busybox"); err == nil {
		return path
	}
	return ""
}

func runChinaRoute(ctx context.Context, tool string, target routeTarget, db *PingTargetDB) routeResult {
	traceIP := target.Address
	pingIPs := target.PingIPs
	if db != nil {
		if merged := db.RoutePingIPs(target.City, target.Carrier); len(merged) > 0 {
			pingIPs = merged
		}
	}
	if traceIP == "" && len(pingIPs) > 0 {
		traceIP = pingIPs[0]
	}
	result := routeResult{City: target.City, Carrier: target.Carrier, Address: traceIP, Direction: "回程", Line: "Unknown"}
	if traceIP == "" {
		result.PingText = "不可达"
		result.Line = "Unknown"
		return result
	}

	type pingOutcome struct {
		host string
		ms   float64
		text string
	}
	pingCh := make(chan pingOutcome, 1)
	go func() {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 14*time.Second)
		defer pingCancel()
		host, ms, text := probeTargetFailoverFast(pingCtx, pingIPs)
		pingCh <- pingOutcome{host, ms, text}
	}()

	name := strings.ToLower(filepathBase(tool))
	var args []string
	switch {
	case strings.Contains(name, "nexttrace"):
		args = []string{"--no-color", "--language", "cn", "--queries", "1", "--max-hops", "20", traceIP}
	case name == "cmd":
		args = []string{"/C", "chcp 65001>nul & tracert -d -w 700 -h 20 " + traceIP}
	case name == "tracepath":
		args = []string{"-m", "20", traceIP}
	case name == "mtr":
		args = []string{"-n", "-r", "-c", "1", "-m", "20", traceIP}
	case name == "busybox":
		args = []string{"traceroute", "-n", "-m", "20", "-w", "1", "-q", "1", traceIP}
	default:
		args = []string{"-n", "-m", "20", "-w", "1", "-q", "1", traceIP}
	}
	perRoute := 40 * time.Second
	traceCtx, cancel := context.WithTimeout(ctx, perRoute)
	defer cancel()
	out, err := exec.CommandContext(traceCtx, tool, args...).CombinedOutput()
	result.Output = strings.TrimSpace(string(out))
	if len(result.Output) > 24000 {
		result.Output = result.Output[:24000]
	}
	result.Labels = DetectRouteLabels(result.Output)
	result.Hops = detectRouteHops(result.Output)
	result.Path = parseRoutePath(result.Output)
	if len(result.Path) == 0 && len(result.Labels) > 0 {
		for _, label := range result.Labels {
			result.Path = append(result.Path, routeMapNode{Label: label})
		}
	}
	result.Line = routeLine(result.Labels)
	po := <-pingCh
	if po.host != "" {
		result.Address = po.host
	}
	result.PingMS, result.PingText = po.ms, po.text
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

func pingRouteTarget(ctx context.Context, address string) (float64, string) {
	ms, text, _ := pingTargetDetailed(ctx, address, 3, 2, 8*time.Second)
	return ms, text
}

func parsePingPacketLoss(output string) float64 {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)%\s*packet\s*loss`),
		regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)%\s*loss`),
		regexp.MustCompile(`(?i)Lost\s*=\s*\d+\s*\((\d+(?:\.\d+)?)%`),
	}
	for _, pattern := range patterns {
		if m := pattern.FindStringSubmatch(output); len(m) > 1 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil {
				return v
			}
		}
	}
	return -1
}

func pingTargetDetailed(ctx context.Context, address string, count, waitSec int, timeout time.Duration) (float64, string, float64) {
	return pingTargetWithLoss(ctx, address, count, waitSec, timeout)
}

func pingTargetWithOpts(ctx context.Context, address string, count, waitSec int, timeout time.Duration) (float64, string) {
	pingPath, err := exec.LookPath("ping")
	if err != nil {
		return 0, "Ping 工具不可用"
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"-c", strconv.Itoa(count), "-W", strconv.Itoa(waitSec), address}
	if runtime.GOOS == "windows" {
		args = []string{"-n", strconv.Itoa(count), "-w", strconv.Itoa(waitSec * 1000), address}
	}
	out, _ := exec.CommandContext(pingCtx, pingPath, args...).CombinedOutput()
	raw := string(out)
	matches := pingTimePattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return 0, "超时"
	}
	total := 0.0
	replies := 0
	for _, match := range matches {
		value, parseErr := strconv.ParseFloat(match[1], 64)
		if parseErr == nil {
			total += value
			replies++
		}
	}
	if replies == 0 {
		return 0, "超时"
	}
	average := total / float64(replies)
	return average, fmt.Sprintf("%.1f ms", average)
}

func pingTargetWithLoss(ctx context.Context, address string, count, waitSec int, timeout time.Duration) (float64, string, float64) {
	pingPath, err := exec.LookPath("ping")
	if err != nil {
		return 0, "Ping 工具不可用", -1
	}
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"-c", strconv.Itoa(count), "-W", strconv.Itoa(waitSec), address}
	if runtime.GOOS == "windows" {
		args = []string{"-n", strconv.Itoa(count), "-w", strconv.Itoa(waitSec * 1000), address}
	}
	out, _ := exec.CommandContext(pingCtx, pingPath, args...).CombinedOutput()
	raw := string(out)
	matches := pingTimePattern.FindAllStringSubmatch(raw, -1)
	if len(matches) == 0 {
		return 0, "超时", 100
	}
	total := 0.0
	replies := 0
	for _, match := range matches {
		value, parseErr := strconv.ParseFloat(match[1], 64)
		if parseErr == nil {
			total += value
			replies++
		}
	}
	if replies == 0 {
		return 0, "超时", 100
	}
	loss := parsePingPacketLoss(raw)
	if loss < 0 {
		loss = float64(count-replies) / float64(count) * 100
	}
	average := total / float64(replies)
	return average, fmt.Sprintf("%.1f ms", average), loss
}

func pingTarget(ctx context.Context, address string) (float64, string) {
	return pingTargetWithOpts(ctx, address, 4, 3, 10*time.Second)
}

var pingTimePattern = regexp.MustCompile(`(?i)(?:time|时间)[=<]\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)
var routeIPv4Pattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
var nextTraceHopLine = regexp.MustCompile(`(?m)^\s*(\d+)\s+(\S+)\s+(.+)$`)
var bracketTagPattern = regexp.MustCompile(`\[([^\]]+)\]`)

func parseRoutePath(output string) []routeMapNode {
	if output == "" {
		return nil
	}
	out := make([]routeMapNode, 0, 8)
	var prevKey string
	for _, line := range strings.Split(output, "\n") {
		match := nextTraceHopLine.FindStringSubmatch(line)
		if len(match) < 4 {
			continue
		}
		ip, rest := match[2], strings.TrimSpace(match[3])
		if shouldSkipRouteHop(ip, rest) {
			continue
		}
		region := extractRouteRegion(rest)
		label := hopRouteLabel(rest)
		if label == "" || label == "?" {
			continue
		}
		key := region + "|" + label
		if key == prevKey {
			continue
		}
		prevKey = key
		out = append(out, routeMapNode{Region: region, Label: label})
		if len(out) >= 12 {
			break
		}
	}
	return out
}

func shouldSkipRouteHop(ip, rest string) bool {
	if ip == "*" {
		return true
	}
	upper := strings.ToUpper(rest)
	if strings.Contains(upper, "RFC1918") {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	if parsed.IsLoopback() || parsed.IsPrivate() || parsed.IsLinkLocalUnicast() {
		return true
	}
	return false
}

func extractRouteRegion(rest string) string {
	geo := rest
	if idx := strings.Index(geo, "]"); idx >= 0 {
		geo = strings.TrimSpace(geo[idx+1:])
	} else {
		geo = regexp.MustCompile(`^AS\d+\s+`).ReplaceAllString(geo, "")
		geo = strings.TrimSpace(geo)
	}
	if idx := strings.Index(geo, "  "); idx > 0 {
		geo = strings.TrimSpace(geo[:idx])
	}
	parts := strings.Fields(geo)
	countryWords := map[string]bool{
		"中国": true, "美国": true, "日本": true, "英国": true, "德国": true,
		"法国": true, "新加坡": true, "香港": true, "台湾": true,
	}
	region := ""
	for _, p := range parts {
		if strings.Contains(p, ".") {
			break
		}
		if countryWords[p] {
			if p == "香港" || p == "台湾" || p == "新加坡" {
				region = p
			}
			continue
		}
		region = p
	}
	return region
}

func hopRouteLabel(rest string) string {
	if labels := DetectRouteLabels(rest); len(labels) > 0 {
		return labels[0]
	}
	if m := bracketTagPattern.FindStringSubmatch(rest); len(m) > 1 {
		return simplifyBracketRouteTag(m[1])
	}
	return ""
}

func simplifyBracketRouteTag(tag string) string {
	upper := strings.ToUpper(tag)
	switch {
	case strings.Contains(upper, "CN2"):
		return "CN2 GIA"
	case strings.Contains(upper, "CMI"):
		return "CMI"
	case strings.Contains(upper, "CHINANET"), strings.Contains(upper, "CHINATELECOM"):
		return "电信 163"
	case strings.Contains(upper, "CU"), strings.Contains(upper, "UNICOM"):
		return "联通 4837"
	case strings.Contains(upper, "HURRICANE"):
		return "HE"
	case strings.Contains(upper, "CMIN2"):
		return "CMIN2"
	case strings.Contains(upper, "CMNET"):
		return "移动"
	default:
		return strings.TrimSpace(tag)
	}
}

func detectRouteHops(text string) []string {
	hops := make([]string, 0, 20)
	seen := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		matches := routeIPv4Pattern.FindAllString(line, -1)
		if len(matches) == 0 {
			continue
		}
		ip := matches[len(matches)-1]
		if seen[ip] {
			continue
		}
		seen[ip] = true
		hops = append(hops, ip)
		if len(hops) == 20 {
			break
		}
	}
	return hops
}

func filepathBase(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func routeLine(labels []string) string {
	if len(labels) == 0 {
		return "Unknown"
	}
	return strings.Join(labels, " → ")
}

func installRouteTool(ctx context.Context) error {
	if commandExists("curl") {
		out, err := exec.CommandContext(ctx, "sh", "-c", "curl -fsSL https://nxtrace.org/nt | bash").CombinedOutput()
		if err == nil && commandExists("nexttrace") {
			return nil
		}
		_ = out
	}
	var command string
	switch {
	case commandExists("apt-get"):
		command = "export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq traceroute"
	case commandExists("dnf"):
		command = "dnf install -y -q traceroute"
	case commandExists("yum"):
		command = "yum install -y -q traceroute"
	case commandExists("apk"):
		command = "apk add --no-cache traceroute"
	default:
		return fmt.Errorf("未找到支持的包管理器 apt-get/dnf/yum/apk")
	}
	out, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if len(message) > 500 {
			message = message[len(message)-500:]
		}
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s", message)
	}
	return nil
}

func installPingTool(ctx context.Context) error {
	var command string
	switch {
	case commandExists("apt-get"):
		command = "export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq iputils-ping"
	case commandExists("dnf"):
		command = "dnf install -y -q iputils"
	case commandExists("yum"):
		command = "yum install -y -q iputils"
	case commandExists("apk"):
		command = "apk add --no-cache iputils"
	default:
		return fmt.Errorf("未找到支持的包管理器 apt-get/dnf/yum/apk")
	}
	out, err := exec.CommandContext(ctx, "sh", "-c", command).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if len(message) > 500 {
			message = message[len(message)-500:]
		}
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s", message)
	}
	if !commandExists("ping") {
		return fmt.Errorf("安装命令已完成，但 PATH 中仍未找到 ping")
	}
	return nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

type Services struct{}

func (Services) ID() string   { return "services" }
func (Services) Name() string { return "Streaming & AI" }
func (p Services) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	targets := []struct{ name, url string }{
		{"Netflix", "https://www.netflix.com/title/80018499"},
		{"Disney+", "https://www.disneyplus.com/"},
		{"YouTube Premium", "https://www.youtube.com/premium"},
		{"TikTok", "https://www.tiktok.com/"},
		{"Spotify", "https://www.spotify.com/"},
		{"Prime Video", "https://www.primevideo.com/"},
		{"Max", "https://play.max.com/"},
		{"Hulu", "https://www.hulu.com/"},
		{"Paramount+", "https://www.paramountplus.com/"},
		{"Apple TV+", "https://tv.apple.com/"},
		{"BBC iPlayer", "https://www.bbc.co.uk/iplayer"},
		{"DAZN", "https://www.dazn.com/"},
		{"Abema", "https://abema.tv/"},
		{"ChatGPT", "https://chatgpt.com/"},
		{"Claude", "https://claude.ai/"},
		{"Gemini", "https://gemini.google.com/"},
		{"Microsoft Copilot", "https://copilot.microsoft.com/"},
		{"Reddit", "https://www.reddit.com/"},
	}
	requestTimeout := 8 * time.Second
	client := newMediaClient(requestTimeout, true)
	publicIP := getPublicIPv4(ctx, client)
	results := make([]model.Metric, len(targets))
	items := make([]map[string]any, len(targets))
	positive := 0
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, t := range targets {
		i, t := i, t
		wg.Add(1)
		go func() {
			defer wg.Done()
			log("Services: " + t.name)
			check := checkMediaService(ctx, client, t.name, t.url, publicIP)
			text := mediaMetricText(check.Status, check.Region, check.UnlockType)
			results[i] = model.Metric{Name: t.name, Text: text}
			items[i] = mediaItem(t.name, check.Status, check.Region, check.UnlockType)
			if check.ScorePositive {
				mu.Lock()
				positive++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	s.Metrics = results
	s.Details = map[string]any{"items": items}
	s.Score = int(float64(positive) / float64(len(targets)) * 10000)
	s.Summary = fmt.Sprintf("%d/%d 可用；采用平台 API / 片源探测（Netflix 双片源、Disney+ bamgrid 等）", positive, len(targets))
	if positive < len(targets) {
		s.Status = model.StatusWarning
	}
	return Finish(s, started)
}

func serviceUnlocked(status int, body string) bool {
	if status < 200 || status >= 400 {
		return false
	}
	lower := strings.ToLower(body)
	blocked := []string{
		"not available in your region", "not available in your country", "not currently available",
		"unsupported country", "geo-blocked", "geoblocked", "proxy or unblocker", "access denied",
		"service is not available", "content is not available", "地区不可用", "所在地区无法",
	}
	for _, marker := range blocked {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return true
}

func DetectRouteLabels(text string) []string {
	patterns := []struct {
		label   string
		needles []string
	}{
		{"CN2 GIA", []string{"CN2GIA", "CN2-GIA", "AS4809", "59.43."}},
		{"电信 163", []string{"AS4134", "202.97.", "CHINANET-BB"}},
		{"联通 9929", []string{"AS9929", "CUII"}},
		{"联通 4837", []string{"AS4837", "UNICOM-BACKBONE"}},
		{"CMIN2", []string{"AS58807", "CMIN2-NET"}},
		{"CMI", []string{"AS58453", "CMI.CHINAMOBILE"}},
		{"Cogent", []string{"AS174", "COGENT"}},
		{"HE", []string{"AS6939", "HURRICANE"}},
		{"NTT", []string{"AS2914", "NTT"}},
	}
	out := make([]string, 0)
	upper := strings.ToUpper(text)
	for _, pattern := range patterns {
		for _, needle := range pattern.needles {
			if strings.Contains(upper, strings.ToUpper(needle)) {
				out = append(out, pattern.label)
				break
			}
		}
	}
	return out
}
