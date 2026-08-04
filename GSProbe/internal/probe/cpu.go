package probe

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync"
	"time"

	"gsprobe/internal/model"
)

type CPU struct{}

func (CPU) ID() string   { return "cpu" }
func (CPU) Name() string { return "CPU" }

func (p CPU) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	log("CPU: 采样偷取时间")
	stealPct, stealText := cpuStealPercent(1500 * time.Millisecond)
	duration := 1200 * time.Millisecond
	workers := runtime.NumCPU()
	log("CPU: SHA-256 多核吞吐测试")
	deadline := time.Now().Add(duration)
	block := bytes.Repeat([]byte("GSProbe"), 8192)
	var hashes uint64
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := uint64(0)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				_ = sha256.Sum256(block)
				local++
			}
			mu.Lock()
			hashes += local
			mu.Unlock()
		}()
	}
	wg.Wait()
	mbps := float64(hashes*uint64(len(block))) / duration.Seconds() / 1024 / 1024
	log("CPU: gzip 压缩测试")
	gzipStart := time.Now()
	compressed := int64(0)
	for time.Since(gzipStart) < duration/2 {
		var out bytes.Buffer
		zw := gzip.NewWriter(&out)
		_, _ = zw.Write(block)
		_ = zw.Close()
		compressed += int64(len(block))
	}
	gzipMBps := float64(compressed) / time.Since(gzipStart).Seconds() / 1024 / 1024
	score := min(10000, int(mbps*3+gzipMBps*5))
	s.Score = score
	spec := cpuSpecInfo()
	modelName, _ := spec["model"].(string)
	physical, _ := spec["physicalCores"].(int)
	logical, _ := spec["logicalCores"].(int)
	mhz, _ := spec["mhz"].(string)
	util, _ := spec["utilization"].(string)
	if modelName == "" {
		modelName = "未知"
	}
	s.Summary = modelName
	flags := cpuFlags()
	s.Metrics = []model.Metric{
		{Name: "型号", Text: modelName},
		{Name: "物理核心", Text: fmt.Sprintf("%d 核", physical)},
		{Name: "逻辑线程", Text: fmt.Sprintf("%d 线程", logical)},
		{Name: "频率", Text: map[bool]string{true: mhz + " MHz", false: "未知"}[mhz != ""]},
		{Name: "利用率", Text: util},
		{Name: "偷取时间", Text: stealText, Value: stealPct, Unit: "%"},
		{Name: "缓存", Text: cpuCache()},
		{Name: "SHA-256", Value: mbps, Unit: "MiB/s", Higher: true},
		{Name: "gzip", Value: gzipMBps, Unit: "MiB/s", Higher: true},
	}
	s.Details = map[string]any{
		"flags": flags,
		"cache": cpuCache(),
		"spec":  spec,
	}
	return Finish(s, started)
}
