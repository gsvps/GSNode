package probe

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gsprobe/pkg/latency"
)

//go:embed ping_targets_default.json
var embeddedPingTargets []byte

type PingTargetEntry struct {
	IP     string `json:"ip"`
	Score  int    `json:"score"`
	Source string `json:"source,omitempty"`
	ICMP   bool   `json:"icmp,omitempty"`
	TCP80  bool   `json:"tcp80,omitempty"`
}

type PingTargetDB struct {
	Version   string                                      `json:"version"`
	UpdatedAt string                                      `json:"updatedAt,omitempty"`
	Provinces map[string]map[string][]PingTargetEntry     `json:"provinces"`
	Routes    map[string]map[string][]PingTargetEntry     `json:"routes"`
}

func ParsePingTargetDB(raw []byte) (*PingTargetDB, error) {
	return parsePingTargetDB(raw)
}

// FilterDeadTargets removes unreachable entries (score 0). If all fail, keeps the top entry.
func FilterDeadTargets(db *PingTargetDB) (removed int) {
	if db == nil {
		return 0
	}
	filter := func(entries []PingTargetEntry) []PingTargetEntry {
		if len(entries) == 0 {
			return entries
		}
		good := make([]PingTargetEntry, 0, len(entries))
		for _, e := range entries {
			if e.Score > 0 {
				good = append(good, e)
			}
		}
		if len(good) > 0 {
			removed += len(entries) - len(good)
			return good
		}
		// 验证环境不可达时保留原始前 3 个备选，供运行时 failover
		if len(entries) > 3 {
			return entries[:3]
		}
		return entries
	}
	for code, carriers := range db.Provinces {
		for carrier, entries := range carriers {
			db.Provinces[code][carrier] = filter(entries)
		}
	}
	for city, carriers := range db.Routes {
		for carrier, entries := range carriers {
			db.Routes[city][carrier] = filter(entries)
		}
	}
	return removed
}

func ValidatePingTargetDB(ctx context.Context, db *PingTargetDB, perIP time.Duration) int {
	if db == nil {
		return 0
	}
	updated := 0
	validate := func(entries []PingTargetEntry) []PingTargetEntry {
		out := append([]PingTargetEntry(nil), entries...)
		for i := range out {
			score, icmp, tcp80 := validatePingTarget(ctx, out[i].IP, perIP)
			out[i].Score = score
			out[i].ICMP = icmp
			out[i].TCP80 = tcp80
			updated++
		}
		sort.Slice(out, func(a, b int) bool {
			if out[a].Score != out[b].Score {
				return out[a].Score > out[b].Score
			}
			return out[a].IP < out[b].IP
		})
		return out
	}
	for code, carriers := range db.Provinces {
		for carrier, entries := range carriers {
			db.Provinces[code][carrier] = validate(entries)
		}
	}
	for city, carriers := range db.Routes {
		for carrier, entries := range carriers {
			db.Routes[city][carrier] = validate(entries)
		}
	}
	db.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return updated
}

func ProbePingTarget(ctx context.Context, ip string, timeout time.Duration) (score int, icmp, tcp80 bool) {
	return validatePingTarget(ctx, ip, timeout)
}

func validatePingTarget(ctx context.Context, ip string, timeout time.Duration) (score int, icmp, tcp80 bool) {
	if ip == "" {
		return 0, false, false
	}
	perCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if ms, _ := pingTarget(perCtx, ip); ms > 0 {
		return latency.ScoreFromLatency(ms), true, false
	}
	if ms, _ := tcpDialMs(perCtx, ip, "80"); ms > 0 {
		return max(0, latency.ScoreFromLatency(ms)-5), false, true
	}
	return 0, false, false
}

func LoadPingTargetDB(dataDir string) (*PingTargetDB, error) {
	if raw, err := fetchRemotePingTargets(); err == nil && len(raw) > 0 {
		db, err := parsePingTargetDB(raw)
		if err == nil {
			_ = cachePingTargets(dataDir, raw)
			return db, nil
		}
	}
	if dataDir != "" {
		for _, name := range []string{"ping_targets.json", "ping_targets.cache.json"} {
			raw, err := os.ReadFile(filepath.Join(dataDir, name))
			if err == nil {
				if db, err := parsePingTargetDB(raw); err == nil {
					return db, nil
				}
			}
		}
	}
	return parsePingTargetDB(embeddedPingTargets)
}

func fetchRemotePingTargets() ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	raw, url, err := fetchBytesFirstOK(ctx, http.DefaultClient, pingTargetsURLs(), 4<<20)
	if err != nil {
		return nil, fmt.Errorf("ping targets: %w", err)
	}
	if _, err := parsePingTargetDB(raw); err != nil {
		return nil, fmt.Errorf("ping targets from %s: %w", url, err)
	}
	return raw, nil
}

func cachePingTargets(dataDir string, raw []byte) error {
	if dataDir == "" {
		return nil
	}
	path := filepath.Join(dataDir, "ping_targets.cache.json")
	return os.WriteFile(path, raw, 0644)
}

func parsePingTargetDB(raw []byte) (*PingTargetDB, error) {
	var db PingTargetDB
	if err := json.Unmarshal(raw, &db); err != nil {
		return nil, err
	}
	if db.Provinces == nil {
		db.Provinces = map[string]map[string][]PingTargetEntry{}
	}
	if db.Routes == nil {
		db.Routes = map[string]map[string][]PingTargetEntry{}
	}
	return &db, nil
}

func (db *PingTargetDB) ProvinceIPs(code, carrier string) []string {
	return db.sortedIPs(db.Provinces[strings.ToUpper(code)][carrier])
}

func (db *PingTargetDB) RouteIPs(city, carrier string) []string {
	return db.sortedIPs(db.Routes[city][carrier])
}

func (db *PingTargetDB) sortedIPs(entries []PingTargetEntry) []string {
	if len(entries) == 0 {
		return nil
	}
	cp := append([]PingTargetEntry(nil), entries...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Score != cp[j].Score {
			return cp[i].Score > cp[j].Score
		}
		return cp[i].IP < cp[j].IP
	})
	out := make([]string, 0, len(cp))
	seen := map[string]bool{}
	for _, e := range cp {
		ip := strings.TrimSpace(e.IP)
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	return out
}

var routeCityProvince = map[string]string{
	"北京": "BJ",
	"上海": "SH",
	"广州": "GD",
}

func (db *PingTargetDB) RoutePingIPs(city, carrier string) []string {
	return mergeIPList(db.RouteIPs(city, carrier), db.ProvinceIPs(routeCityProvince[city], carrier))
}

func mergeIPList(lists ...[]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	for _, list := range lists {
		for _, ip := range list {
			ip = strings.TrimSpace(ip)
			if ip == "" || seen[ip] {
				continue
			}
			seen[ip] = true
			out = append(out, ip)
		}
	}
	return out
}

func probeMatrixOnce(ctx context.Context, ip string) (float64, string, float64) {
	return probeMatrixPing(ctx, ip, 5, 6*time.Second, 3*time.Second)
}

// probeChinaMatrixOnce 全国矩阵专用：3 包 ping，缩短单点耗时。
func probeChinaMatrixOnce(ctx context.Context, ip string) (float64, string, float64) {
	return probeMatrixPing(ctx, ip, 3, 5*time.Second, 2*time.Second)
}

func probeMatrixPing(ctx context.Context, ip string, packets int, pingTimeout, tcpTimeout time.Duration) (float64, string, float64) {
	if ip == "" {
		return 0, "不可达", -1
	}
	perCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	waitSec := int(pingTimeout.Seconds())
	if waitSec < 1 {
		waitSec = 1
	}
	if ms, text, loss := pingTargetDetailed(perCtx, ip, packets, waitSec, pingTimeout); ms > 0 {
		return ms, text, loss
	}
	tcpCtx, tcpCancel := context.WithTimeout(ctx, tcpTimeout)
	defer tcpCancel()
	if ms, text := tcpDialMs(tcpCtx, ip, "80"); ms > 0 {
		return ms, text, 0
	}
	return 0, "超时", 100
}

func probeTargetFailoverFast(ctx context.Context, ips []string) (host string, ms float64, text string) {
	host, ms, text, _ = probeTargetFailoverDetail(ctx, ips)
	return host, ms, text
}

func probeTargetFailoverDetail(ctx context.Context, ips []string) (host string, ms float64, text string, loss float64) {
	return probeTargetFailoverDetailLimit(ctx, ips, len(ips), nil)
}

func probeChinaTargetFailover(ctx context.Context, ips []string) (host string, ms float64, text string, loss float64) {
	maxTry := len(ips)
	if maxTry > 4 {
		maxTry = 4
	}
	return probeTargetFailoverDetailLimit(ctx, ips, maxTry, probeChinaMatrixOnce)
}

func probeTargetFailoverDetailLimit(ctx context.Context, ips []string, maxTry int, probe func(context.Context, string) (float64, string, float64)) (host string, ms float64, text string, loss float64) {
	if len(ips) == 0 {
		return "", 0, "不可达", -1
	}
	if probe == nil {
		probe = probeMatrixOnce
	}
	if maxTry < 1 || maxTry > len(ips) {
		maxTry = len(ips)
	}
	for _, ip := range ips[:maxTry] {
		if ip == "" {
			continue
		}
		ms, text, loss = probe(ctx, ip)
		if ms > 0 {
			return ip, ms, text, loss
		}
	}
	return ips[0], 0, textOrDefault(text, "超时"), 100
}

func probeTargetFailover(ctx context.Context, ips []string) (host string, ms float64, text string) {
	return probeTargetFailoverFast(ctx, ips)
}

func textOrDefault(text, fallback string) string {
	if strings.TrimSpace(text) == "" {
		return fallback
	}
	return text
}
