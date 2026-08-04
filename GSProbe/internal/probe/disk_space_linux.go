//go:build linux

package probe

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type diskVolume struct {
	Total, Free       uint64
	InodesTotal       uint64
	InodesFree        uint64
	FSType            string
	BlockDevice       string
	IOScheduler       string
}

func diskSpace(path string) (total, free uint64) {
	v := diskVolumeInfo(path)
	return v.Total, v.Free
}

func diskVolumeInfo(path string) diskVolume {
	var v diskVolume
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return v
	}
	v.Total = stat.Blocks * uint64(stat.Bsize)
	v.Free = stat.Bavail * uint64(stat.Bsize)
	v.InodesTotal = stat.Files
	v.InodesFree = stat.Ffree
	fstype, source := mountInfo(path)
	v.FSType = fstype
	if source != "" {
		v.BlockDevice = source
	} else {
		v.BlockDevice = diskDevice(path)
	}
	v.IOScheduler = ioSchedulerForDevice(v.BlockDevice)
	return v
}

func mountInfo(path string) (fstype, source string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return "", ""
	}
	bestLen := 0
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mp := unescapeMountPath(fields[1])
		if mp == abs || strings.HasPrefix(abs, mp+"/") {
			if len(mp) >= bestLen {
				bestLen = len(mp)
				source = fields[0]
				fstype = fields[2]
			}
		}
	}
	return fstype, source
}

func unescapeMountPath(s string) string {
	s = strings.ReplaceAll(s, "\\040", " ")
	s = strings.ReplaceAll(s, "\\011", "\t")
	return s
}

func blockDeviceName(dev string) string {
	base := filepath.Base(strings.TrimSpace(dev))
	if base == "" || base == "." {
		return ""
	}
	if strings.HasPrefix(base, "nvme") {
		if idx := strings.LastIndex(base, "p"); idx > 0 {
			suffix := base[idx+1:]
			allDigits := true
			for _, c := range suffix {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return base[:idx]
			}
		}
		return base
	}
	for len(base) > 0 {
		last := base[len(base)-1]
		if last >= '0' && last <= '9' {
			base = base[:len(base)-1]
			continue
		}
		break
	}
	return base
}

func ioSchedulerForDevice(dev string) string {
	block := blockDeviceName(dev)
	if block == "" {
		return "未知"
	}
	data, err := os.ReadFile(filepath.Join("/sys/block", block, "queue/scheduler"))
	if err != nil {
		return "未知"
	}
	s := string(data)
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.Index(s[i:], "]"); j > 1 {
			return strings.TrimSpace(s[i+1 : i+j])
		}
	}
	return strings.TrimSpace(s)
}
