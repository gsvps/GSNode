package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type sectionInfo struct {
	label string
	desc  string
}

var sectionCatalog = map[string]sectionInfo{
	"System":         {label: "系统", desc: "识别操作系统、虚拟化与 TCP 配置"},
	"CPU":            {label: "CPU", desc: "SHA-256 多核吞吐与 gzip 压缩测试"},
	"Memory":         {label: "内存", desc: "分配、复制吞吐与随机延迟测试"},
	"Disk":           {label: "磁盘", desc: "顺序读写与 4K 随机 IOPS 测试"},
	"IP Quality":     {label: "IP 质量", desc: "公网 IP、黑名单与邮件端口检测"},
	"Streaming & AI": {label: "流媒体 & AI", desc: "Netflix、ChatGPT 等解锁检测"},
	"Network":        {label: "网络", desc: "网卡、GeoIP、NAT 与上下行测速"},
	"ChinaLatency":   {label: "全国延迟", desc: "34 省级三网延迟测试"},
	"Route":          {label: "回程路由", desc: "三地三网 Ping 与线路识别"},
}

func DefaultProbeNames() []string {
	return []string{
		"System", "CPU", "Memory", "Disk",
		"IP Quality", "Streaming & AI", "Network", "ChinaLatency", "Route",
	}
}

func sectionDisplay(name string) (label, desc string) {
	if info, ok := sectionCatalog[name]; ok {
		return info.label, info.desc
	}
	return name, "检测中"
}

func doneLine(name, status string, durationSec float64) string {
	label, desc := sectionDisplay(name)
	icon := colorSectionIcon(status)
	line := fmt.Sprintf("%s %s  %s", icon, colorHeading(label), paint(colorDim, desc))
	if durationSec > 0 {
		line += paint(colorDim, fmt.Sprintf("  %.1fs", durationSec))
	}
	return line
}

func PrintProgressHeading(w io.Writer) {
	fmt.Fprintln(w, colorHeading("检测进度"))
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ProgressTracker prints one permanent line per finished probe, plus a single
// ephemeral "still running" status line that's rewritten in place (via \r,
// never a multi-line cursor-up) and ticks on its own so a long-running probe
// never looks stuck. It deliberately does not print a permanent line when a
// probe starts — with several probes finishing at different times, a
// start+finish pair for every one of them roughly doubled the output and
// made it hard to tell what's actually still in flight; the always-current
// ephemeral line answers that directly instead.
//
// Only ever one line is manipulated with \r/\033[K, so unlike the old
// multi-line redraw there's no "N rows" arithmetic to break if a line wraps.
// The summary text is also capped to a handful of names specifically so it
// stays short regardless of terminal width.
type ProgressTracker struct {
	w        io.Writer
	mu       sync.Mutex
	names    []string
	running  map[int]bool
	spinIdx  int
	stopTick chan struct{}
}

func NewProgressTracker(w io.Writer) *ProgressTracker {
	return &ProgressTracker{w: w, running: map[int]bool{}}
}

func (t *ProgressTracker) Init(names []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.names = append([]string(nil), names...)
	t.running = map[int]bool{}
	PrintProgressHeading(t.w)
}

func (t *ProgressTracker) Start(index int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.running[index] = true
	t.renderEphemeralLocked()
	if t.stopTick == nil {
		stop := make(chan struct{})
		t.stopTick = stop
		go t.tickLoop(stop)
	}
}

func (t *ProgressTracker) Done(index int, name, status string, durationSec float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.running, index)
	t.clearEphemeralLocked()
	fmt.Fprintln(t.w, doneLine(name, status, durationSec))
	if len(t.running) == 0 && t.stopTick != nil {
		close(t.stopTick)
		t.stopTick = nil
		return
	}
	t.renderEphemeralLocked()
}

// tickLoop only keeps the ephemeral line's spinner/content fresh; it never
// blocks anything else on its own exit, so unlike the earlier per-probe
// animate() goroutines there's no lock-then-wait-for-goroutine pattern here
// for a concurrent tick to deadlock against.
func (t *ProgressTracker) tickLoop(stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			t.mu.Lock()
			t.spinIdx++
			t.renderEphemeralLocked()
			t.mu.Unlock()
		}
	}
}

func (t *ProgressTracker) clearEphemeralLocked() {
	fmt.Fprint(t.w, "\r\033[K")
}

func (t *ProgressTracker) renderEphemeralLocked() {
	t.clearEphemeralLocked()
	summary := t.runningSummaryLocked()
	if summary == "" {
		return
	}
	fmt.Fprint(t.w, summary)
}

const maxRunningNamesShown = 3

func (t *ProgressTracker) runningSummaryLocked() string {
	if len(t.running) == 0 {
		return ""
	}
	shown := make([]string, 0, maxRunningNamesShown)
	total := 0
	for i, name := range t.names {
		if !t.running[i] {
			continue
		}
		total++
		if len(shown) < maxRunningNamesShown {
			label, _ := sectionDisplay(name)
			shown = append(shown, label)
		}
	}
	text := strings.Join(shown, "、")
	if total > len(shown) {
		text += fmt.Sprintf(" 等 %d 项", total)
	}
	spin := spinnerFrames[t.spinIdx%len(spinnerFrames)]
	return paint(colorDim, spin) + " " + paint(colorDim, "进行中: ") + colorHeading(text)
}
