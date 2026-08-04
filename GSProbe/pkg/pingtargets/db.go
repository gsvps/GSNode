package pingtargets

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"gsprobe/pkg/latency"
)

type Entry struct {
	IP     string `json:"ip"`
	Score  int    `json:"score"`
	Source string `json:"source,omitempty"`
	ICMP   bool   `json:"icmp,omitempty"`
	TCP80  bool   `json:"tcp80,omitempty"`
}

type DB struct {
	Version   string                         `json:"version"`
	UpdatedAt string                         `json:"updatedAt,omitempty"`
	Provinces map[string]map[string][]Entry  `json:"provinces"`
	Routes    map[string]map[string][]Entry  `json:"routes"`
}

func Parse(raw []byte) (*DB, error) {
	var db DB
	if err := json.Unmarshal(raw, &db); err != nil {
		return nil, err
	}
	if db.Provinces == nil {
		db.Provinces = map[string]map[string][]Entry{}
	}
	if db.Routes == nil {
		db.Routes = map[string]map[string][]Entry{}
	}
	return &db, nil
}

// FilterDead removes unreachable entries (score 0). If all fail, keeps the top entries.
func FilterDead(db *DB) (removed int) {
	if db == nil {
		return 0
	}
	filter := func(entries []Entry) []Entry {
		if len(entries) == 0 {
			return entries
		}
		good := make([]Entry, 0, len(entries))
		for _, e := range entries {
			if e.Score > 0 {
				good = append(good, e)
			}
		}
		if len(good) > 0 {
			removed += len(entries) - len(good)
			return good
		}
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

func Validate(ctx context.Context, db *DB, perIP time.Duration) int {
	if db == nil {
		return 0
	}
	updated := 0
	validate := func(entries []Entry) []Entry {
		out := append([]Entry(nil), entries...)
		for i := range out {
			score, icmp, tcp80 := Probe(ctx, out[i].IP, perIP)
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

func Probe(ctx context.Context, ip string, timeout time.Duration) (score int, icmp, tcp80 bool) {
	if ip == "" {
		return 0, false, false
	}
	perCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if ms, _ := pingOnce(perCtx, ip); ms > 0 {
		return latency.ScoreFromLatency(ms), true, false
	}
	if ms, _ := tcpDialMs(perCtx, ip, "80"); ms > 0 {
		return max(0, latency.ScoreFromLatency(ms)-5), false, true
	}
	return 0, false, false
}
