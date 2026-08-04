package probe

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gsprobe/internal/model"
)

type Disk struct{}

func (Disk) ID() string   { return "disk" }
func (Disk) Name() string { return "Disk" }
func (p Disk) Run(ctx context.Context, opts model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	total := int64(256 << 20)
	path := filepath.Join(os.TempDir(), "gsprobe-disk.tmp")
	defer os.Remove(path)
	buf := make([]byte, 1<<20)
	small := make([]byte, 4096)
	_, _ = rand.Read(buf)
	_, _ = rand.Read(small)
	log("Disk: 顺序写入测试")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		s.Status = model.StatusFailed
		s.Error = err.Error()
		return Finish(s, started)
	}
	t0 := time.Now()
	var written int64
	for written < total {
		select {
		case <-ctx.Done():
			break
		default:
		}
		n, e := f.Write(buf)
		written += int64(n)
		if e != nil {
			err = e
			break
		}
	}
	_ = f.Sync()
	writeRate := float64(written) / time.Since(t0).Seconds() / 1024 / 1024
	log("Disk: 顺序读取测试")
	_, _ = f.Seek(0, 0)
	t1 := time.Now()
	read, _ := io.Copy(io.Discard, f)
	readRate := float64(read) / time.Since(t1).Seconds() / 1024 / 1024
	log("Disk: 4K 随机读 IOPS 测试")
	readDeadline := time.Now().Add(time.Second)
	readOps := 0
	t2 := time.Now()
	for time.Now().Before(readDeadline) {
		off := int64((readOps*7919)%max(1, int(total/4096))) * 4096
		if _, e := f.ReadAt(small, off); e != nil && e != io.EOF {
			break
		}
		readOps++
	}
	readIOPS := float64(readOps) / time.Since(t2).Seconds()
	log("Disk: 4K 随机写 IOPS 测试")
	writeDeadline := time.Now().Add(time.Second)
	writeOps := 0
	t3 := time.Now()
	for time.Now().Before(writeDeadline) {
		off := int64((writeOps*6271)%max(1, int(total/4096))) * 4096
		if _, e := f.WriteAt(small, off); e != nil {
			break
		}
		writeOps++
	}
	writeIOPS := float64(writeOps) / time.Since(t3).Seconds()
	log("Disk: fsync 延迟测试")
	fsyncP50, fsyncP99 := measureFsyncLatency(f, small, 30)
	_ = f.Close()
	vol := diskVolumeInfo(os.TempDir())
	device := vol.BlockDevice
	if device == "" {
		device = diskDevice(os.TempDir())
	}
	if err != nil {
		s.Status = model.StatusWarning
		s.Error = err.Error()
	}
	s.Score = min(10000, int(writeRate*6+readRate*3+readIOPS/20))
	s.Summary = "文件系统容量与临时文件原生 I/O 测试"
	s.Metrics = []model.Metric{}
	if vol.Total > 0 {
		usedPercent := float64(vol.Total-vol.Free) / float64(vol.Total) * 100
		s.Metrics = append(s.Metrics,
			model.Metric{Name: "测试设备", Text: device},
			model.Metric{Name: "文件系统", Text: vol.FSType},
			model.Metric{Name: "块设备", Text: vol.BlockDevice},
			model.Metric{Name: "I/O 调度器", Text: vol.IOScheduler},
			model.Metric{Name: "磁盘总容量", Value: float64(vol.Total) / 1024 / 1024 / 1024, Unit: "GiB"},
			model.Metric{Name: "磁盘可用容量", Value: float64(vol.Free) / 1024 / 1024 / 1024, Unit: "GiB"},
			model.Metric{Name: "磁盘使用率", Value: usedPercent, Unit: "%"},
		)
	}
	if vol.InodesTotal > 0 {
		inodeUsed := vol.InodesTotal - vol.InodesFree
		inodePct := float64(inodeUsed) / float64(vol.InodesTotal) * 100
		s.Metrics = append(s.Metrics,
			model.Metric{Name: "Inode 总量", Value: float64(vol.InodesTotal), Unit: ""},
			model.Metric{Name: "Inode 可用", Value: float64(vol.InodesFree), Unit: ""},
			model.Metric{Name: "Inode 使用率", Value: inodePct, Unit: "%"},
		)
	}
	s.Metrics = append(s.Metrics,
		model.Metric{Name: "顺序写", Value: writeRate, Unit: "MiB/s", Higher: true},
		model.Metric{Name: "顺序读", Value: readRate, Unit: "MiB/s", Higher: true},
		model.Metric{Name: "4K 随机读", Value: readIOPS, Unit: "IOPS", Higher: true},
		model.Metric{Name: "4K 随机写", Value: writeIOPS, Unit: "IOPS", Higher: true},
		model.Metric{Name: "fsync P50", Value: fsyncP50, Unit: "ms"},
		model.Metric{Name: "fsync P99", Value: fsyncP99, Unit: "ms"},
	)
	s.Details = map[string]any{
		"device": device,
		"volume": map[string]any{
			"fstype":      vol.FSType,
			"blockDevice": vol.BlockDevice,
			"scheduler":   vol.IOScheduler,
			"inodesTotal": vol.InodesTotal,
			"inodesFree":  vol.InodesFree,
		},
		"profiles": []map[string]any{
			{
				"name":   "RND4K/Q1",
				"read":   fmt.Sprintf("%.1f MB/s (%.1fk IOPS)", readIOPS*4096/1024/1024, readIOPS/1000),
				"write":  fmt.Sprintf("%.1f MB/s (%.1fk IOPS)", writeIOPS*4096/1024/1024, writeIOPS/1000),
				"status": iopsStatus(readIOPS, writeIOPS),
			},
			{
				"name":   "SEQ1M/Q1",
				"read":   fmt.Sprintf("%.0f MB/s", readRate),
				"write":  fmt.Sprintf("%.0f MB/s", writeRate),
				"status": seqStatus(readRate, writeRate),
			},
		},
		"gpu": gpuInfo(),
	}
	return Finish(s, started)
}

func iopsStatus(read, write float64) string {
	avg := (read + write) / 2
	switch {
	case avg >= 50000:
		return "good"
	case avg >= 10000:
		return "warn"
	default:
		return "bad"
	}
}

func seqStatus(read, write float64) string {
	avg := (read + write) / 2
	switch {
	case avg >= 500:
		return "good"
	case avg >= 100:
		return "warn"
	default:
		return "bad"
	}
}

func measureFsyncLatency(f *os.File, buf []byte, n int) (p50, p99 float64) {
	if f == nil || n <= 0 {
		return 0, 0
	}
	lats := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		off := int64(i*4096) % max(int64(1<<20), 4096)
		if _, err := f.WriteAt(buf, off); err != nil {
			break
		}
		t0 := time.Now()
		_ = f.Sync()
		lats = append(lats, time.Since(t0))
	}
	if len(lats) == 0 {
		return 0, 0
	}
	sort.Slice(lats, func(i, j int) bool { return lats[i] < lats[j] })
	p50 = float64(lats[len(lats)/2].Microseconds()) / 1000
	idx99 := int(float64(len(lats)-1) * 0.99)
	if idx99 < 0 {
		idx99 = 0
	}
	if idx99 >= len(lats) {
		idx99 = len(lats) - 1
	}
	p99 = float64(lats[idx99].Microseconds()) / 1000
	return p50, p99
}

func boolEnabledLabel(on bool) string {
	if on {
		return "已启用"
	}
	return "未启用"
}
