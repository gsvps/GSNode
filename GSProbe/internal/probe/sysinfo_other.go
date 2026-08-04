//go:build !linux

package probe

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func kernelVersion() string { return "未知" }

func systemUptime() string { return "未知" }

func loadAverage() string { return "未知" }

func processStats() string {
	return fmt.Sprintf("%d 逻辑核心", runtime.NumCPU())
}

func localeInfo() string {
	return time.Now().Format("MST -0700")
}

func virtLabel(raw string) string {
	if raw == "" || raw == "unknown" {
		return "未知"
	}
	return raw
}

func motherboardInfo() (bios, chipset, nic string) {
	return "未知", "未知", "未知"
}

func cpuModel() string {
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("wmic", "cpu", "get", "Name", "/value").Output(); err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "Name=") {
					name := strings.TrimSpace(strings.TrimPrefix(line, "Name="))
					if name != "" {
						return name
					}
				}
			}
		}
	}
	return fmt.Sprintf("%d 核", runtime.NumCPU())
}

func cpuSpecInfo() map[string]any {
	return map[string]any{
		"model":         cpuModel(),
		"physicalCores": runtime.NumCPU(),
		"logicalCores":  runtime.NumCPU(),
		"mhz":           "",
		"utilization":   "未知",
	}
}

func sysctlConfigured() map[string]string { return map[string]string{} }

func sysctlValue(key string, configured map[string]string) string {
	_ = key
	_ = configured
	return "unknown"
}

func cpuCache() string { return "未知" }

func cpuFlags() map[string]bool {
	return map[string]bool{
		"VT-x/AMD-V": false,
		"AES-NI":     false,
		"AVX2":       false,
		"BMI1/2":     false,
		"EPT/NPT":    false,
	}
}

func memUsage() (total, used, avail, swapTotal, swapUsed string) {
	return "未知", "未知", "未知", "未知", "未知"
}

func memAvailableBytes() uint64 { return 0 }

func ballooningEnabled() bool { return false }

func ksmEnabled() bool { return false }

func gpuInfo() string { return "未检测到独立显卡" }

func diskDevice(path string) string { return path }

func cpuStealPercent(time.Duration) (float64, string) { return 0, "N/A" }
