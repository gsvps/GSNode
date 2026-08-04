package probe

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"gsprobe/internal/model"
)

type System struct{}

func (System) ID() string   { return "system" }
func (System) Name() string { return "System" }
func (p System) Run(_ context.Context, _ model.Options, log Logger) model.Section {
	started := time.Now()
	s := Base(p)
	host, _ := os.Hostname()
	virt := "unknown"
	bbr := "unknown"
	congestion := "unknown"
	qdisc := "unknown"
	rmem := "unknown"
	wmem := "unknown"
	log("System: 识别操作系统、虚拟化与 TCP 配置")
	configured := sysctlConfigured()
	if runtime.GOOS == "linux" {
		if out, err := exec.Command("systemd-detect-virt").Output(); err == nil {
			virt = strings.TrimSpace(string(out))
		}
		congestion = sysctlValue("net.ipv4.tcp_congestion_control", configured)
		rmem = sysctlValue("net.core.rmem_max", configured)
		wmem = sysctlValue("net.core.wmem_max", configured)
		if q := sysctlValue("net.core.default_qdisc", configured); q != "unknown" {
			qdisc = q
		}
		if strings.Contains(strings.ToLower(congestion), "bbr") {
			bbr = "enabled"
		} else if congestion != "unknown" {
			bbr = "disabled"
		}
		if out, err := exec.Command("sh", "-c", "tc qdisc show 2>/dev/null | sed -n '1s/^qdisc \\([^ ]*\\).*/\\1/p'").Output(); err == nil && strings.TrimSpace(string(out)) != "" {
			qdisc = strings.TrimSpace(string(out))
		}
	} else if runtime.GOOS == "windows" {
		virt = "windows-host"
	}
	osName := runtime.GOOS
	if f, err := os.Open("/etc/os-release"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "PRETTY_NAME=") {
				osName = strings.Trim(strings.TrimPrefix(sc.Text(), "PRETTY_NAME="), "\"")
				break
			}
		}
	}
	kernel := kernelVersion()
	if kernel != "未知" && runtime.GOOS == "linux" {
		osName = osName + " · " + kernel
	}
	container := "no"
	if _, err := os.Stat("/.dockerenv"); err == nil {
		container = "docker"
	}
	bios, chipset, nic := motherboardInfo()
	log("System: 检测 cgroup 资源限制")
	cg := collectCgroupLimits()
	s.Score = 7000
	s.Stars = 4
	s.Summary = osName
	s.Metrics = []model.Metric{
		{Name: "虚拟化", Text: virtLabel(virt)},
		{Name: "架构", Text: runtime.GOARCH},
		{Name: "操作系统", Text: osName},
		{Name: "运行时长", Text: systemUptime()},
		{Name: "系统负载", Text: loadAverage()},
		{Name: "进程", Text: processStats()},
		{Name: "区域设置", Text: localeInfo()},
		{Name: "主机名", Text: host},
		{Name: "处理器核心", Text: fmt.Sprintf("%d 核", runtime.NumCPU())},
		{Name: "物理内存", Text: totalMemoryGiB()},
		{Name: "容器", Text: container},
		{Name: "TCP 拥塞算法", Text: congestion},
		{Name: "队列调度算法", Text: qdisc},
		{Name: "TCP 接收缓冲区上限", Text: rmem + " bytes"},
		{Name: "TCP 发送缓冲区上限", Text: wmem + " bytes"},
		{Name: "BBR", Text: bbr},
		{Name: "CPU 限额", Text: cg.CPUText},
		{Name: "CPU 状态", Text: cg.CPUStatus},
		{Name: "内存限额", Text: cg.MemText},
		{Name: "内存状态", Text: cg.MemStatus},
		{Name: "I/O 限额", Text: cg.IOText},
	}
	s.Details = map[string]any{
		"motherboard": map[string]string{
			"bios":    bios,
			"chipset": chipset,
			"network": nic,
		},
		"cgroup": map[string]any{
			"version":     cg.Version,
			"path":        cg.CgroupPath,
			"visibleCPUs": cg.VisibleCPUs,
			"cpuLimited":  cg.CPULimited,
			"cpuMaxCores": cg.CPUMaxCores,
			"cpuStatus":   cg.CPUStatus,
			"cpuText":     cg.CPUText,
			"memLimited":  cg.MemLimited,
			"memMaxBytes": cg.MemMaxBytes,
			"memStatus":   cg.MemStatus,
			"memText":     cg.MemText,
			"ioLimited":   cg.IOLimited,
			"ioText":      cg.IOText,
		},
		"sysctl": map[string]any{
			"configured": configured,
			"effective": map[string]string{
				"net.ipv4.tcp_congestion_control": sysctlPlain(congestion),
				"net.core.default_qdisc":          sysctlPlain(qdisc),
				"net.core.rmem_max":               sysctlPlain(strings.TrimSuffix(rmem, " bytes")),
				"net.core.wmem_max":               sysctlPlain(strings.TrimSuffix(wmem, " bytes")),
			},
		},
	}
	return Finish(s, started)
}

func totalMemoryGiB() string {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return "未知"
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "MemTotal:" {
			kb, err := strconv.ParseFloat(fields[1], 64)
			if err == nil {
				return fmt.Sprintf("%.1f GiB", kb/1024/1024)
			}
		}
	}
	return "未知"
}

func sysctlPlain(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, " (配置)")
	v = strings.TrimSuffix(v, " (sysctl.conf)")
	return v
}
