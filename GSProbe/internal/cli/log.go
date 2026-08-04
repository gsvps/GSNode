package cli

import (
	"fmt"
	"io"
)

func PrintBanner(w io.Writer, version, platform string) {
	fmt.Fprintln(w, "====================================")
	fmt.Fprintln(w, " GSNode 一键检测")
	fmt.Fprintln(w, "====================================")
	fmt.Fprintf(w, "版本     : v%s\n", version)
	fmt.Fprintf(w, "平台     : %s\n", platform)
	fmt.Fprintln(w)
}

// Progress 保留供 Web/日志模式使用；CLI 检测进度由 ProgressTracker 单行展示。
func Progress(w io.Writer, msg string) {
	_ = w
	_ = msg
}

func SectionDone(w io.Writer, name string, status string, score int, durationSec float64) {
	tracker := NewProgressTracker(w)
	tracker.Done(0, name, status, durationSec)
}
