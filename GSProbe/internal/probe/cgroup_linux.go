//go:build linux

package probe

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func collectCgroupLimitsLinux(base cgroupLimits) cgroupLimits {
	if path := selfCgroupV2Path(); path != "" && cgroupV2Mounted() {
		base.Version = "v2"
		base.CgroupPath = path
		base = scanCgroupV2Path(base, path)
		return finalizeCgroupLimits(base)
	}
	if v1 := selfCgroupV1Paths(); len(v1) > 0 {
		base.Version = "v1"
		base = scanCgroupV1(base, v1)
		return finalizeCgroupLimits(base)
	}
	return finalizeCgroupLimits(base)
}

func cgroupV2Mounted() bool {
	_, err := os.Stat("/sys/fs/cgroup/cgroup.controllers")
	return err == nil
}

func selfCgroupV2Path() string {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		if parts[0] == "0" && parts[1] == "" {
			return parts[2]
		}
	}
	return ""
}

func selfCgroupV1Paths() map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return out
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 || parts[1] == "" {
			continue
		}
		for _, ctrl := range strings.Split(parts[1], ",") {
			ctrl = strings.TrimSpace(ctrl)
			if ctrl != "" {
				out[ctrl] = parts[2]
			}
		}
	}
	return out
}

func scanCgroupV2Path(base cgroupLimits, path string) cgroupLimits {
	for p := path; ; {
		if raw := readCgroupV2File(p, "cpu.max"); raw != "" {
			quota, period, unlimited := parseCPUMaxV2(raw)
			if !unlimited {
				base = mergeCPULimit(base, cpuCoresFromQuota(quota, period))
			}
		}
		if raw := readCgroupV2File(p, "memory.max"); raw != "" {
			bytes, unlimited := parseMemoryMaxV2(raw)
			if !unlimited && !nearUnlimitedMemory(bytes) {
				base = mergeMemoryLimit(base, bytes)
			}
		}
		if raw := readCgroupV2File(p, "io.max"); raw != "" && parseIOMaxV2(raw) {
			base.IOLimited = true
		}
		parent := parentCgroupPath(p)
		if parent == "" || parent == p {
			break
		}
		p = parent
	}
	return base
}

func scanCgroupV1(base cgroupLimits, paths map[string]string) cgroupLimits {
	cpuPath := firstNonEmpty(paths["cpu"], paths["cpuacct"])
	if cpuPath != "" {
		if cores, limited := readV1CPULimit(cpuPath); limited {
			base = mergeCPULimit(base, cores)
		}
	}
	memPath := paths["memory"]
	if memPath != "" {
		if bytes, limited := readV1MemoryLimit(memPath); limited {
			base = mergeMemoryLimit(base, bytes)
		}
	}
	blkPath := firstNonEmpty(paths["blkio"], paths["io"])
	if blkPath != "" && readV1IOLimited(blkPath) {
		base.IOLimited = true
	}
	return base
}

func readCgroupV2File(cgroupPath, name string) string {
	base := "/sys/fs/cgroup"
	p := filepath.Join(base, strings.TrimPrefix(cgroupPath, "/"), name)
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func parentCgroupPath(path string) string {
	path = strings.TrimSuffix(path, "/")
	if path == "" || path == "/" {
		return ""
	}
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func readV1CPULimit(path string) (float64, bool) {
	for _, root := range []string{"/sys/fs/cgroup/cpu", "/sys/fs/cgroup/cpu,cpuacct"} {
		quota, ok := readIntFile(filepath.Join(root, strings.TrimPrefix(path, "/"), "cpu.cfs_quota_us"))
		if !ok {
			continue
		}
		if quota < 0 {
			return 0, false
		}
		period, ok := readIntFile(filepath.Join(root, strings.TrimPrefix(path, "/"), "cpu.cfs_period_us"))
		if !ok || period <= 0 {
			period = 100000
		}
		return cpuCoresFromQuota(uint64(quota), uint64(period)), true
	}
	return 0, false
}

func readV1MemoryLimit(path string) (uint64, bool) {
	for _, root := range []string{"/sys/fs/cgroup/memory"} {
		val, ok := readIntFile(filepath.Join(root, strings.TrimPrefix(path, "/"), "memory.limit_in_bytes"))
		if !ok {
			continue
		}
		if nearUnlimitedMemory(uint64(val)) {
			return 0, false
		}
		return uint64(val), true
	}
	return 0, false
}

func readV1IOLimited(path string) bool {
	for _, root := range []string{"/sys/fs/cgroup/blkio", "/sys/fs/cgroup/io"} {
		p := filepath.Join(root, strings.TrimPrefix(path, "/"))
		for _, name := range []string{
			"blkio.throttle.read_bps_device",
			"blkio.throttle.write_bps_device",
			"blkio.throttle.read_iops_device",
			"blkio.throttle.write_iops_device",
		} {
			data, err := os.ReadFile(filepath.Join(p, name))
			if err != nil {
				continue
			}
			if strings.TrimSpace(string(data)) != "" {
				return true
			}
		}
	}
	return false
}

func readIntFile(path string) (int64, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func readProcFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
