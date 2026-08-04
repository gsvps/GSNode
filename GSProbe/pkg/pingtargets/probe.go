package pingtargets

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"time"
)

var pingTimePattern = regexp.MustCompile(`(?i)(?:time|时间)[=<]\s*([0-9]+(?:\.[0-9]+)?)\s*ms`)

func pingOnce(ctx context.Context, address string) (float64, string) {
	pingPath, err := exec.LookPath("ping")
	if err != nil {
		return 0, "Ping 工具不可用"
	}
	count, waitSec := 4, 3
	timeout := 10 * time.Second
	pingCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{"-c", strconv.Itoa(count), "-W", strconv.Itoa(waitSec), address}
	if runtime.GOOS == "windows" {
		args = []string{"-n", strconv.Itoa(count), "-w", strconv.Itoa(waitSec * 1000), address}
	}
	out, _ := exec.CommandContext(pingCtx, pingPath, args...).CombinedOutput()
	matches := pingTimePattern.FindAllStringSubmatch(string(out), -1)
	if len(matches) == 0 {
		return 0, "超时"
	}
	total := 0.0
	replies := 0
	for _, match := range matches {
		value, parseErr := strconv.ParseFloat(match[1], 64)
		if parseErr == nil {
			total += value
			replies++
		}
	}
	if replies == 0 {
		return 0, "超时"
	}
	average := total / float64(replies)
	return average, fmt.Sprintf("%.1f ms", average)
}

func tcpDialMs(ctx context.Context, host, port string) (float64, string) {
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	start := time.Now()
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	if err != nil {
		return 0, "不可达"
	}
	_ = conn.Close()
	ms := float64(time.Since(start).Microseconds()) / 1000
	return ms, fmt.Sprintf("%.1f ms", ms)
}

// ProbeIntlEndpoint mirrors GSProbe intl latency probing: TCP to the speedtest
// port first, then ICMP ping, with optional alternate host/port.
func ProbeIntlEndpoint(ctx context.Context, host, port, altHost, altPort string) bool {
	perCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	if ms, _ := tcpDialMs(perCtx, host, port); ms > 0 {
		return true
	}
	if ms, _ := pingOnce(perCtx, host); ms > 0 {
		return true
	}
	if altHost != "" {
		p := altPort
		if p == "" {
			p = port
		}
		if ms, _ := tcpDialMs(perCtx, altHost, p); ms > 0 {
			return true
		}
		if ms, _ := pingOnce(perCtx, altHost); ms > 0 {
			return true
		}
	}
	return false
}
