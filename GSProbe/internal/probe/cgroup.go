package probe

import (
	"fmt"
	"math"
	"runtime"
	"strings"
)

type cgroupLimits struct {
	Version     string
	CgroupPath  string
	VisibleCPUs int
	CPULimited  bool
	CPUMaxCores float64
	CPUStatus   string
	CPUText     string
	MemLimited  bool
	MemMaxBytes uint64
	MemText     string
	MemStatus   string
	IOLimited   bool
	IOText      string
}

func collectCgroupLimits() cgroupLimits {
	out := cgroupLimits{
		Version:     "none",
		VisibleCPUs: runtime.NumCPU(),
		CPUStatus:   "正常",
		CPUText:     "无限制",
		MemText:     "无硬限制",
		MemStatus:   "正常",
		IOText:      "无限制",
	}
	if runtime.GOOS == "linux" {
		return collectCgroupLimitsLinux(out)
	}
	out.CPUText = "N/A"
	out.MemText = "N/A"
	out.IOText = "N/A"
	return out
}

func finalizeCgroupLimits(l cgroupLimits) cgroupLimits {
	visible := l.VisibleCPUs
	if visible <= 0 {
		visible = 1
	}
	if l.CPULimited && l.CPUMaxCores > 0 {
		if l.CPUMaxCores < float64(visible)-0.05 {
			l.CPUStatus = "可能缩水"
			l.CPUText = fmt.Sprintf("%.1f 核 (可见 %d 核)", l.CPUMaxCores, visible)
		} else {
			l.CPUStatus = "有限额"
			l.CPUText = fmt.Sprintf("%.1f 核", l.CPUMaxCores)
		}
	} else {
		l.CPUStatus = "正常"
		l.CPUText = "无限制"
	}
	if l.MemLimited && l.MemMaxBytes > 0 {
		l.MemText = formatBytesGiB(l.MemMaxBytes) + " 硬上限"
		if hostMem := memTotalBytes(); hostMem > 0 && l.MemMaxBytes+16<<20 < hostMem {
			l.MemStatus = "低于宿主机"
		} else {
			l.MemStatus = "已限额"
		}
	} else {
		l.MemStatus = "正常"
		l.MemText = "无硬限制"
	}
	if l.IOLimited {
		l.IOText = "已限速"
	} else {
		l.IOText = "无限制"
	}
	return l
}

func parseCPUMaxV2(raw string) (quota uint64, period uint64, unlimited bool) {
	period = 100000
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return 0, period, true
	}
	if strings.EqualFold(fields[0], "max") {
		return 0, period, true
	}
	if len(fields) == 1 {
		q, err := parseUint(fields[0])
		if err != nil {
			return 0, period, true
		}
		return q, period, false
	}
	q, err1 := parseUint(fields[0])
	p, err2 := parseUint(fields[1])
	if err1 != nil || err2 != nil || p == 0 {
		return 0, period, true
	}
	return q, p, false
}

func parseMemoryMaxV2(raw string) (bytes uint64, unlimited bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "max") {
		return 0, true
	}
	v, err := parseUint(raw)
	if err != nil {
		return 0, true
	}
	return v, false
}

func parseIOMaxV2(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.EqualFold(line, "max") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		for _, part := range fields[1:] {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			val := strings.TrimSpace(kv[1])
			if val == "" || strings.EqualFold(val, "max") {
				continue
			}
			return true
		}
	}
	return false
}

func cpuCoresFromQuota(quota, period uint64) float64 {
	if period == 0 {
		return 0
	}
	return float64(quota) / float64(period)
}

func mergeCPULimit(current cgroupLimits, cores float64) cgroupLimits {
	if cores <= 0 {
		return current
	}
	if !current.CPULimited || cores < current.CPUMaxCores {
		current.CPULimited = true
		current.CPUMaxCores = cores
	}
	return current
}

func mergeMemoryLimit(current cgroupLimits, bytes uint64) cgroupLimits {
	if bytes == 0 {
		return current
	}
	if !current.MemLimited || bytes < current.MemMaxBytes {
		current.MemLimited = true
		current.MemMaxBytes = bytes
	}
	return current
}

func formatBytesGiB(bytes uint64) string {
	gb := float64(bytes) / 1024 / 1024 / 1024
	if gb >= 1 {
		return fmt.Sprintf("%.1f GiB", gb)
	}
	mb := float64(bytes) / 1024 / 1024
	if mb >= 1 {
		return fmt.Sprintf("%.0f MiB", mb)
	}
	return fmt.Sprintf("%d KiB", bytes/1024)
}

func parseUint(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	v, err := parseUintBase10(s)
	return v, err
}

func parseUintBase10(s string) (uint64, error) {
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid")
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}

func memTotalBytes() uint64 {
	data, err := readProcFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := parseUintBase10(fields[1])
			if err != nil {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}

func nearUnlimitedMemory(v uint64) bool {
	return v == 0 || v > math.MaxInt64/2
}
