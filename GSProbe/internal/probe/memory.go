package probe

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"gsprobe/internal/model"
)

type Memory struct{}

func (Memory) ID() string   { return "memory" }
func (Memory) Name() string { return "Memory" }

func memoryBenchSize() (size, loops int) {
	size = 128 << 20
	loops = 8
	if avail := memAvailableBytes(); avail > 0 {
		// 两块缓冲区合计不超过可用内存的 35%，为运行时与其它模块留余量。
		maxEach := int(avail * 35 / 100 / 2)
		if maxEach > 0 && maxEach < size {
			size = maxEach
		}
	}
	const minSize = 4 << 20
	if size < minSize {
		size = minSize
	}
	size = (size >> 20) << 20
	if size < minSize {
		size = minSize
	}
	return size, loops
}

func (p Memory) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	size, loops := memoryBenchSize()
	log(fmt.Sprintf("Memory: 分配测试缓冲区 (%d MiB)", size>>20))
	src := make([]byte, size)
	dst := make([]byte, size)
	for i := range src {
		src[i] = byte(i)
	}
	log("Memory: 顺序复制吞吐测试")
	t0 := time.Now()
copyLoop:
	for range loops {
		select {
		case <-ctx.Done():
			break copyLoop
		default:
			copy(dst, src)
		}
	}
	copyRate := float64(size*loops) / time.Since(t0).Seconds() / 1024 / 1024
	log("Memory: 随机访问延迟测试")
	const accesses = 1_000_000
	idx := uint64(1)
	sum := byte(0)
	t1 := time.Now()
	for range accesses {
		idx = idx*6364136223846793005 + 1
		sum ^= src[int(idx%uint64(len(src)))]
	}
	latency := float64(time.Since(t1).Nanoseconds()) / accesses
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	runtime.KeepAlive(sum)
	s.Score = min(10000, int(copyRate*1.2))
	total, used, avail, swapTotal, swapUsed := memUsage()
	s.Summary = fmt.Sprintf("内存 %s · 可用 %s", total, avail)
	s.Metrics = []model.Metric{
		{Name: "内存总量", Text: total},
		{Name: "已用内存", Text: used},
		{Name: "可用内存", Text: avail},
		{Name: "Swap 总量", Text: swapTotal},
		{Name: "Swap 已用", Text: swapUsed},
		{Name: "内存气球", Text: boolEnabledLabel(ballooningEnabled())},
		{Name: "KSM", Text: boolEnabledLabel(ksmEnabled())},
		{Name: "复制", Value: copyRate, Unit: "MiB/s", Higher: true},
		{Name: "随机延迟", Value: latency, Unit: "ns/op"},
		{Name: "测试块", Value: float64(size >> 20), Unit: "MiB"},
	}
	s.Details = map[string]any{
		"ballooning": ballooningEnabled(),
		"ksm":        ksmEnabled(),
	}
	return Finish(s, started)
}
