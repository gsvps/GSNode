package probe

import (
	"context"
	"fmt"
	"time"

	"gsprobe/internal/model"
)

type ChinaLatency struct{}

func (ChinaLatency) ID() string   { return "china-latency" }
func (ChinaLatency) Name() string { return "ChinaLatency" }

func (p ChinaLatency) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	pingDB, err := LoadPingTargetDB(opts.DataDir)
	if err != nil {
		pingDB, _ = parsePingTargetDB(embeddedPingTargets)
	}
	log(fmt.Sprintf("ChinaLatency: 目标库 %s", pingDB.Version))
	total := len(chinaProvinces) * len(chinaCarrierNames)
	concurrency := chinaLatencyConcurrency()
	timeout := chinaLatencyProbeTimeout(concurrency, total)
	log(fmt.Sprintf("ChinaLatency: 超时上限 %s", timeout.Round(time.Second)))
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	results := probeChinaProvinces(ctx, log, pingDB)
	provOK, provTotal := chinaLatencySummary(results)
	s.Summary = fmt.Sprintf("全国34省级三网 %d/%d", provOK, provTotal)
	if provTotal > 0 {
		s.Score = int(float64(provOK) / float64(provTotal) * 10000)
	}
	s.Details = map[string]any{
		"chinaLatency": results,
		"pingTargets": map[string]string{
			"version":   pingDB.Version,
			"updatedAt": pingDB.UpdatedAt,
		},
	}
	if provOK < provTotal {
		s.Status = model.StatusWarning
	}
	return Finish(s, started)
}
