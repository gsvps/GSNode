package cli

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestSectionDisplayChinese(t *testing.T) {
	label, desc := sectionDisplay("System")
	if label != "系统" || desc == "" {
		t.Fatalf("got %q %q", label, desc)
	}
	label, desc = sectionDisplay("Streaming & AI")
	if label != "流媒体 & AI" {
		t.Fatalf("got %q", label)
	}
}

func TestProgressTrackerDone(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	tracker := NewProgressTracker(&buf)
	tracker.Init([]string{"System", "CPU"})
	tracker.Start(0)
	out := buf.String()
	if !strings.Contains(out, "进行中") || !strings.Contains(out, "系统") {
		t.Fatalf("missing ephemeral running line: %q", out)
	}
	tracker.Done(0, "System", "passed", 0.1)
	out = buf.String()
	if !strings.Contains(out, "✓ 系统") {
		t.Fatalf("missing done line: %q", out)
	}
	// Once the only running probe finishes, nothing should be appended after
	// its done line (no ephemeral re-render, since t.running is now empty) —
	// note a plain bytes.Buffer keeps the raw escape-code stream, so an
	// earlier ephemeral write is still literally present in `out`; what
	// matters is that the buffer ends with the done line, not that the
	// substring never occurred.
	if !strings.HasSuffix(out, "0.1s\n") {
		t.Fatalf("expected nothing written after the done line, got: %q", out)
	}
}

// TestProgressTrackerRunningSummaryCapped verifies the ephemeral line stays
// short (a handful of names + a count) even with many probes running at
// once, since that's what keeps it safe from wrapping regardless of
// terminal width.
func TestProgressTrackerRunningSummaryCapped(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	tracker := NewProgressTracker(&buf)
	names := DefaultProbeNames()
	tracker.Init(names)
	for i := range names {
		tracker.Start(i)
	}
	summary := tracker.runningSummaryLocked()
	if !strings.Contains(summary, fmt.Sprintf("等 %d 项", len(names))) {
		t.Fatalf("expected capped summary with total count, got %q", summary)
	}
	shown := strings.Count(summary, "、") + 1
	if shown > maxRunningNamesShown {
		t.Fatalf("expected at most %d names shown, got %d in %q", maxRunningNamesShown, shown, summary)
	}
}

// TestProgressTrackerConcurrentSafe exercises many indices being started and
// finished at once, each repeatedly, under -race. Since probes now run
// concurrently (see runner.go), Start/Done for different indices can be
// called from different goroutines simultaneously; this only needs to prove
// that's race- and panic-free, not that any particular interleaving of
// output occurs.
func TestProgressTrackerConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	tracker := NewProgressTracker(&buf)
	names := make([]string, 8)
	for i := range names {
		names[i] = "System"
	}
	tracker.Init(names)

	var wg sync.WaitGroup
	for i := range names {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := 0; r < 30; r++ {
				tracker.Start(i)
				tracker.Done(i, "System", "passed", 0.1)
			}
		}()
	}
	wg.Wait()
}

func TestDefaultProbeOrder(t *testing.T) {
	names := DefaultProbeNames()
	if len(names) != 9 || names[4] != "IP Quality" || names[8] != "Route" {
		t.Fatalf("unexpected order: %#v", names)
	}
}

func TestProgressIgnoresDetailMessages(t *testing.T) {
	var buf bytes.Buffer
	Progress(&buf, "CPU: SHA-256 多核吞吐测试")
	if buf.Len() != 0 {
		t.Fatalf("Progress should be no-op in CLI, got %q", buf.String())
	}
}
