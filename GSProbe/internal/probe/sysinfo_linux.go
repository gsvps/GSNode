//go:build linux

package probe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	sysctlAllCache map[string]string
	sysctlAllOnce  sync.Once
)

func kernelVersion() string {
	if out, err := exec.Command("uname", "-r").Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return "未知"
}

func systemUptime() string {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return "未知"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "未知"
	}
	sec, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "未知"
	}
	d := time.Duration(sec) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%d 天 %d 小时 %d 分钟", days, hours, mins)
}

func loadAverage() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "未知"
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		return fmt.Sprintf("%s, %s, %s", fields[0], fields[1], fields[2])
	}
	return "未知"
}

func processStats() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "未知"
	}
	fields := strings.Fields(string(data))
	running, total := "?", "?"
	if len(fields) >= 4 {
		parts := strings.Split(fields[3], "/")
		if len(parts) == 2 {
			running, total = parts[0], parts[1]
		}
	}
	procs := "?"
	if entries, err := os.ReadDir("/proc"); err == nil {
		n := 0
		for _, e := range entries {
			if _, err := strconv.Atoi(e.Name()); err == nil {
				n++
			}
		}
		procs = strconv.Itoa(n)
	}
	return fmt.Sprintf("%s 个进程 · 活跃/总数 %s/%s", procs, running, total)
}

func localeInfo() string {
	lang := os.Getenv("LANG")
	if lang == "" {
		lang = "未知"
	}
	tz := "未知"
	if out, err := exec.Command("timedatectl", "show", "-p", "Timezone", "--value").Output(); err == nil {
		tz = strings.TrimSpace(string(out))
	} else if data, err := os.ReadFile("/etc/timezone"); err == nil {
		tz = strings.TrimSpace(string(data))
	}
	offset := time.Now().Format("MST -0700")
	return fmt.Sprintf("%s · %s %s", lang, tz, offset)
}

func virtLabel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "kvm":
		return "KVM 虚拟机"
	case "qemu":
		return "QEMU 虚拟机"
	case "vmware":
		return "VMware 虚拟机"
	case "xen":
		return "Xen 虚拟机"
	case "microsoft":
		return "Hyper-V 虚拟机"
	case "docker":
		return "Docker 容器"
	case "lxc", "lxc-libvirt":
		return "LXC 容器"
	case "none", "host":
		return "物理机 / 裸金属"
	default:
		if raw == "" || raw == "unknown" {
			return "未知"
		}
		return raw
	}
}

func motherboardInfo() (bios, chipset, nic string) {
	bios = readFirstExisting(
		"/sys/class/dmi/id/bios_vendor",
		"/sys/class/dmi/id/bios_version",
	)
	if v := readFirstExisting("/sys/class/dmi/id/bios_version"); v != "" {
		if vendor := readFirstExisting("/sys/class/dmi/id/bios_vendor"); vendor != "" {
			bios = vendor + ", " + v
		} else {
			bios = v
		}
	}
	chipset = readFirstExisting("/sys/class/dmi/id/board_name")
	if chipset == "" {
		chipset = readFirstExisting("/sys/class/dmi/id/product_name")
	}
	nic = firstNetworkDevice()
	if bios == "" {
		bios = "未知"
	}
	if chipset == "" {
		chipset = "未知"
	}
	if nic == "" {
		nic = "未知"
	}
	return
}

func firstNetworkDevice() string {
	entries, err := os.ReadDir("/sys/class/net")
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if name == "lo" {
			continue
		}
		vendorPath := filepath.Join("/sys/class/net", name, "device/vendor")
		devicePath := filepath.Join("/sys/class/net", name, "device/device")
		vendor := strings.TrimSpace(readFileOrEmpty(vendorPath))
		device := strings.TrimSpace(readFileOrEmpty(devicePath))
		if vendor != "" {
			return fmt.Sprintf("%s (%s)", name, vendor)
		}
		if device != "" {
			return name
		}
		return name
	}
	return ""
}

func cpuModel() string {
	spec := cpuSpecInfo()
	if spec["model"] == "" {
		return fmt.Sprintf("%d 核", runtime.NumCPU())
	}
	return spec["model"].(string)
}

func cpuSpecInfo() map[string]any {
	out := map[string]any{
		"model":          "",
		"physicalCores":  0,
		"logicalCores":   runtime.NumCPU(),
		"mhz":            "",
		"utilization":    cpuUtilization(),
	}
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return out
	}
	model := ""
	mhz := ""
	logical := 0
	socketCores := map[string]int{}
	sockets := map[string]bool{}
	currentPhysical := ""
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && model == "" {
				model = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "cpu MHz") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && mhz == "" {
				mhz = strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(line, "processor") {
			logical++
		}
		if strings.HasPrefix(line, "physical id") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentPhysical = strings.TrimSpace(parts[1])
				sockets[currentPhysical] = true
			}
		}
		if strings.HasPrefix(line, "cpu cores") && currentPhysical != "" {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				if n, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					socketCores[currentPhysical] = n
				}
			}
		}
	}
	if logical == 0 {
		logical = runtime.NumCPU()
	}
	physical := 0
	for id := range sockets {
		if n := socketCores[id]; n > 0 {
			physical += n
		}
	}
	if physical == 0 {
		physical = logical
	}
	out["model"] = model
	out["physicalCores"] = physical
	out["logicalCores"] = logical
	out["mhz"] = mhz
	return out
}

func cpuUtilization() string {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return "未知"
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return "未知"
	}
	load1, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return "未知"
	}
	cpus := runtime.NumCPU()
	if cpus <= 0 {
		cpus = 1
	}
	pct := load1 / float64(cpus) * 100
	if pct > 100 {
		pct = 100
	}
	return fmt.Sprintf("%.0f%%", pct)
}

func inDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func sysctlConfigRoots() []string {
	seen := map[string]bool{}
	var roots []string
	add := func(root string) {
		if root == "" {
			if !seen[""] {
				seen[""] = true
				roots = append(roots, "")
			}
			return
		}
		if seen[root] {
			return
		}
		if _, err := os.Stat(filepath.Join(root, "etc")); err != nil {
			return
		}
		seen[root] = true
		roots = append(roots, root)
	}
	add("")
	if inDocker() {
		for _, root := range []string{"/proc/1/root", "/host", "/hostfs", "/rootfs"} {
			add(root)
		}
	}
	return roots
}

func sysctlConfigured() map[string]string {
	out := map[string]string{}
	for _, root := range sysctlConfigRoots() {
		for _, path := range sysctlConfigPathsForRoot(root) {
			parseSysctlFile(path, out)
		}
	}
	if inDocker() {
		parseSysctlText(nsenterFile("/etc/sysctl.conf"), out)
		for _, dir := range []string{"/etc/sysctl.d", "/run/sysctl.d", "/usr/lib/sysctl.d"} {
			if text := nsenterGlob(dir, "*.conf"); text != "" {
				parseSysctlText(text, out)
			}
		}
	}
	return out
}

func sysctlConfigPathsForRoot(root string) []string {
	prefix := root
	if prefix == "" {
		prefix = "/"
	} else {
		prefix = strings.TrimRight(root, "/") + "/"
	}
	dirs := []string{
		prefix + "usr/lib/sysctl.d",
		prefix + "run/sysctl.d",
		prefix + "etc/sysctl.d",
	}
	var paths []string
	for _, dir := range dirs {
		entries, err := filepath.Glob(filepath.Join(dir, "*.conf"))
		if err != nil {
			continue
		}
		sort.Strings(entries)
		paths = append(paths, entries...)
	}
	paths = append(paths, prefix+"etc/sysctl.conf")
	return paths
}

func parseSysctlFile(path string, out map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	parseSysctlText(string(data), out)
}

func parseSysctlText(text string, out map[string]string) {
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasPrefix(trimmed, "-") {
			if eq := strings.Index(trimmed, "="); eq > 1 {
				unsetKey := strings.TrimSpace(trimmed[1:eq])
				if unsetKey != "" {
					delete(out, unsetKey)
				}
			}
			continue
		}
		key, val, ok := parseSysctlLine(line)
		if ok {
			out[key] = val
		}
	}
}

func parseSysctlLine(line string) (key, val string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
		return "", "", false
	}
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	eq := strings.Index(line, "=")
	if eq < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:eq])
	val = strings.TrimSpace(line[eq+1:])
	val = strings.Trim(val, "\"'")
	if key == "" || val == "" {
		return "", "", false
	}
	return key, val, true
}

func sysctlProcRoots() []string {
	roots := []string{""}
	if inDocker() {
		for _, root := range []string{"/proc/1/root", "/host", "/hostfs", "/rootfs"} {
			if _, err := os.Stat(filepath.Join(root, "proc/sys")); err == nil {
				roots = append(roots, root)
			}
		}
	}
	return roots
}

func sysctlAllFromCommand() map[string]string {
	sysctlAllOnce.Do(func() {
		sysctlAllCache = map[string]string{}
		cmd := exec.Command("sysctl", "-a")
		cmd.Env = append(os.Environ(), "PATH=/usr/sbin:/sbin:/usr/bin:/bin")
		if raw, err := cmd.Output(); err == nil {
			parseSysctlText(string(raw), sysctlAllCache)
		}
	})
	return sysctlAllCache
}

func sysctlProcPath(root, rel string) string {
	if root == "" {
		return "/proc/sys/" + rel
	}
	return filepath.Join(root, "proc/sys", rel)
}

func readProcSysValue(rel string) string {
	for _, root := range sysctlProcRoots() {
		path := sysctlProcPath(root, rel)
		if data, err := os.ReadFile(path); err == nil {
			if v := strings.TrimSpace(string(data)); v != "" {
				return v
			}
		}
	}
	cmd := exec.Command("sh", "-c", "cat /proc/sys/"+rel+" 2>/dev/null")
	if out, err := cmd.Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	return ""
}

func nsenterFile(path string) string {
	if !inDocker() {
		return ""
	}
	for _, args := range [][]string{
		{"-t", "1", "-m", "--", "cat", path},
		{"--target", "1", "--mount", "--", "cat", path},
	} {
		cmd := exec.Command("nsenter", args...)
		if out, err := cmd.Output(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				return v
			}
		}
	}
	return ""
}

func nsenterGlob(dir, pattern string) string {
	if !inDocker() {
		return ""
	}
	script := fmt.Sprintf("for f in %s/%s; do [ -f \"$f\" ] && cat \"$f\"; done", dir, pattern)
	for _, args := range [][]string{
		{"-t", "1", "-m", "--", "sh", "-c", script},
		{"--target", "1", "--mount", "--", "sh", "-c", script},
	} {
		cmd := exec.Command("nsenter", args...)
		if out, err := cmd.Output(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				return v
			}
		}
	}
	return ""
}

func nsenterSysctl(key string) string {
	if !inDocker() {
		return ""
	}
	rel := strings.ReplaceAll(key, ".", "/")
	for _, args := range [][]string{
		{"-t", "1", "-n", "--", "sysctl", "-n", key},
		{"-t", "1", "-n", "--", "cat", "/proc/sys/" + rel},
		{"-t", "1", "-m", "-n", "--", "sysctl", "-n", key},
	} {
		cmd := exec.Command("nsenter", args...)
		cmd.Env = append(os.Environ(), "PATH=/usr/sbin:/sbin:/usr/bin:/bin")
		if out, err := cmd.Output(); err == nil {
			if v := strings.TrimSpace(string(out)); v != "" {
				return v
			}
		}
	}
	return ""
}

func sysctlEffective(key string) string {
	rel := strings.ReplaceAll(key, ".", "/")
	if v := readProcSysValue(rel); v != "" {
		return v
	}
	if v := nsenterSysctl(key); v != "" {
		return v
	}
	cmd := exec.Command("sysctl", "-n", key)
	cmd.Env = append(os.Environ(), "PATH=/usr/sbin:/sbin:/usr/bin:/bin")
	if out, err := cmd.Output(); err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	return ""
}

func sysctlConfiguredValue(key string, configured map[string]string) string {
	if configured == nil {
		return ""
	}
	if v := configured[key]; v != "" {
		return v
	}
	return ""
}

func sysctlValue(key string, configured map[string]string) string {
	if v := sysctlEffective(key); v != "" {
		return v
	}
	if v := sysctlConfiguredValue(key, configured); v != "" {
		return v + " (配置)"
	}
	if all := sysctlAllFromCommand(); len(all) > 0 {
		if v := all[key]; v != "" {
			return v
		}
	}
	return "unknown"
}

func cpuCache() string {
	base := "/sys/devices/system/cpu/cpu0/cache"
	entries, err := os.ReadDir(base)
	if err != nil {
		return "未知"
	}
	type part struct {
		level int
		kind  string
		size  string
	}
	var parts []part
	for _, e := range entries {
		dir := filepath.Join(base, e.Name())
		level, _ := strconv.Atoi(strings.TrimSpace(readFileOrEmpty(filepath.Join(dir, "level"))))
		kind := strings.TrimSpace(readFileOrEmpty(filepath.Join(dir, "type")))
		size := strings.TrimSpace(readFileOrEmpty(filepath.Join(dir, "size")))
		if size == "" {
			continue
		}
		parts = append(parts, part{level: level, kind: kind, size: size})
	}
	if len(parts) == 0 {
		return "未知"
	}
	labels := make([]string, 0, len(parts))
	for _, p := range parts {
		tag := fmt.Sprintf("L%d", p.level)
		if p.kind != "" {
			tag = fmt.Sprintf("L%d%s", p.level, strings.ToLower(p.kind[:1]))
		}
		labels = append(labels, fmt.Sprintf("%s %s", tag, p.size))
	}
	return strings.Join(labels, ", ")
}

func cpuFlags() map[string]bool {
	out := map[string]bool{
		"VT-x/AMD-V": false,
		"AES-NI":     false,
		"AVX2":       false,
		"BMI1/2":     false,
		"EPT/NPT":    false,
	}
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return out
	}
	set := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "flags") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		for _, f := range strings.Fields(parts[1]) {
			set[f] = true
		}
		break
	}
	out["VT-x/AMD-V"] = set["vmx"] || set["svm"]
	out["AES-NI"] = set["aes"]
	out["AVX2"] = set["avx2"]
	out["BMI1/2"] = set["bmi1"] && set["bmi2"]
	out["EPT/NPT"] = set["ept"] || set["npt"]
	return out
}

func memUsage() (total, used, avail, swapTotal, swapUsed string) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "未知", "未知", "未知", "未知", "未知"
	}
	vals := map[string]float64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, err := strconv.ParseFloat(fields[1], 64)
		if err == nil {
			vals[key] = v
		}
	}
	format := func(kb float64) string {
		if kb <= 0 {
			return "0"
		}
		gb := kb / 1024 / 1024
		if gb >= 1 {
			return fmt.Sprintf("%.1f GB", gb)
		}
		return fmt.Sprintf("%.0f MB", kb/1024)
	}
	totalKB := vals["MemTotal"]
	availKB := vals["MemAvailable"]
	if availKB == 0 {
		availKB = vals["MemFree"]
	}
	usedKB := totalKB - availKB
	total = format(totalKB)
	used = fmt.Sprintf("%s (%.0f%%)", format(usedKB), pct(usedKB, totalKB))
	avail = fmt.Sprintf("%s (%.0f%%)", format(availKB), pct(availKB, totalKB))
	swapTotal = format(vals["SwapTotal"])
	swapUsedKB := vals["SwapTotal"] - vals["SwapFree"]
	swapUsed = fmt.Sprintf("%s (%.0f%%)", format(swapUsedKB), pct(swapUsedKB, vals["SwapTotal"]))
	return
}

func memAvailableBytes() uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	vals := map[string]float64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, err := strconv.ParseFloat(fields[1], 64)
		if err == nil {
			vals[key] = v
		}
	}
	availKB := vals["MemAvailable"]
	if availKB == 0 {
		availKB = vals["MemFree"]
	}
	if availKB <= 0 {
		return 0
	}
	return uint64(availKB * 1024)
}

func ballooningEnabled() bool {
	_, err := os.Stat("/sys/bus/virtio/drivers/virtio_balloon")
	return err == nil
}

func ksmEnabled() bool {
	data, err := os.ReadFile("/sys/kernel/mm/ksm/run")
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(data)) == "1"
}

func gpuInfo() string {
	if out, err := exec.Command("sh", "-c", "lspci 2>/dev/null | grep -iE 'VGA|3D|Display' | head -1").Output(); err == nil {
		line := strings.TrimSpace(string(out))
		if line != "" {
			if idx := strings.Index(line, ": "); idx >= 0 {
				return strings.TrimSpace(line[idx+2:])
			}
			return line
		}
	}
	return "未检测到独立显卡"
}

func diskDevice(path string) string {
	out, err := exec.Command("df", "-h", path).Output()
	if err != nil {
		return path
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return path
	}
	fields := strings.Fields(lines[1])
	if len(fields) >= 6 {
		return fmt.Sprintf("%s(%s) → %s", fields[0], fields[5], fields[1])
	}
	if len(fields) >= 1 {
		return fields[0]
	}
	return path
}

func readFirstExisting(paths ...string) string {
	for _, p := range paths {
		if v := strings.TrimSpace(readFileOrEmpty(p)); v != "" {
			return v
		}
	}
	return ""
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func pct(part, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return part / total * 100
}

type cpuStatSample struct {
	user, nice, system, idle, iowait, irq, softirq, steal, guest, guestNice uint64
}

func readCPUStat() (cpuStatSample, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStatSample{}, false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return cpuStatSample{}, false
		}
		parse := func(i int) uint64 {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			return v
		}
		out := cpuStatSample{
			user:     parse(1),
			nice:     parse(2),
			system:   parse(3),
			idle:     parse(4),
			iowait:   parse(5),
			irq:      parse(6),
			softirq:  parse(7),
			steal:    parse(8),
		}
		if len(fields) > 9 {
			out.guest = parse(9)
		}
		if len(fields) > 10 {
			out.guestNice = parse(10)
		}
		return out, true
	}
	return cpuStatSample{}, false
}

func (s cpuStatSample) total() uint64 {
	return s.user + s.nice + s.system + s.idle + s.iowait + s.irq + s.softirq + s.steal + s.guest + s.guestNice
}

func cpuStealPercent(sample time.Duration) (float64, string) {
	a, ok := readCPUStat()
	if !ok {
		return 0, "未知"
	}
	time.Sleep(sample)
	b, ok := readCPUStat()
	if !ok {
		return 0, "未知"
	}
	stealDelta := float64(b.steal - a.steal)
	totalDelta := float64(b.total() - a.total())
	if totalDelta <= 0 {
		return 0, "0.0% (正常)"
	}
	pctVal := stealDelta / totalDelta * 100
	label := fmt.Sprintf("%.1f%%", pctVal)
	switch {
	case pctVal < 1:
		label += " (正常)"
	case pctVal < 5:
		label += " (轻微)"
	case pctVal < 15:
		label += " (偏高)"
	default:
		label += " (严重)"
	}
	return pctVal, label
}
