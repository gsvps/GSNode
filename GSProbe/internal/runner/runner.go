package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"gsprobe/internal/cli"
	"gsprobe/internal/model"
	"gsprobe/internal/probe"
	"gsprobe/internal/store"
	"gsprobe/internal/upload"
)

const Version = "0.1.29"

type Hub struct {
	mu   sync.RWMutex
	subs map[chan model.Event]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[chan model.Event]struct{}{}} }
func (h *Hub) Subscribe() (chan model.Event, func()) {
	ch := make(chan model.Event, 64)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() { h.mu.Lock(); delete(h.subs, ch); close(ch); h.mu.Unlock() }
}
func (h *Hub) Publish(e model.Event) {
	e.Timestamp = time.Now()
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

type Runner struct {
	Store   *store.Store
	Hub     *Hub
	mu      sync.Mutex
	running bool
}

func New(s *store.Store, h *Hub) *Runner { return &Runner{Store: s, Hub: h} }
func ID() string                         { b := make([]byte, 6); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (r *Runner) IsRunning() bool        { r.mu.Lock(); defer r.mu.Unlock(); return r.running }
func (r *Runner) Run(ctx context.Context, opts model.Options) (model.Report, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return model.Report{}, context.Canceled
	}
	r.running = true
	r.mu.Unlock()
	defer func() { r.mu.Lock(); r.running = false; r.mu.Unlock() }()

	cliMode := cli.IsRunMode()
	jsonMode := cli.IsJSONMode()
	host, _ := os.Hostname()
	report := model.Report{
		ID:           ID(),
		Version:      Version,
		Mode:         "full",
		Hostname:     host,
		Platform:     runtime.GOOS + "/" + runtime.GOARCH,
		HostProvider: resolveHostProvider(opts),
		StartedAt:    time.Now(),
		Sections:     []model.Section{},
	}

	if cliMode {
		cli.PrintBanner(os.Stdout, Version, report.Platform)
	} else {
		log.Printf("[%s] benchmark started mode=%s host=%s", report.ID, report.Mode, report.Hostname)
	}
	r.Hub.Publish(model.Event{Type: "started", ReportID: report.ID, Message: "GSProbe benchmark started"})

	emit := func(msg string) {
		if !cliMode {
			log.Printf("[%s] %s", report.ID, msg)
		}
		r.Hub.Publish(model.Event{Type: "log", ReportID: report.ID, Message: msg})
	}

	var progress *cli.ProgressTracker
	if cliMode {
		progress = cli.NewProgressTracker(os.Stdout)
		progress.Init(cli.DefaultProbeNames())
	}

	probes := []probe.Probe{
		probe.System{}, probe.CPU{}, probe.Memory{}, probe.Disk{},
		probe.IPQuality{}, probe.Services{}, probe.Network{}, probe.ChinaLatency{}, probe.Route{},
	}
	const (
		idxSystem       = 0
		idxCPU          = 1
		idxMemory       = 2
		idxDisk         = 3
		idxIPQuality    = 4
		idxServices     = 5
		idxNetwork      = 6
		idxChinaLatency = 7
		idxRoute        = 8
	)
	sections := make([]model.Section, len(probes))

	runProbe := func(i int) {
		p := probes[i]
		if cliMode {
			progress.Start(i)
		} else {
			emit("Checking " + p.Name() + "...")
		}
		section := p.Run(ctx, model.Options{DataDir: r.Store.DataRoot()}, emit)
		sections[i] = section
		durationSec := float64(section.Duration) / 1000
		if cliMode {
			progress.Done(i, section.Name, string(section.Status), durationSec)
		} else {
			log.Printf("[%s] %s done status=%s score=%d duration=%.1fs", report.ID, section.Name, section.Status, section.Score, durationSec)
		}
		r.Hub.Publish(model.Event{Type: "section", ReportID: report.ID, Section: &section})
	}
	runGroup := func(indices ...int) {
		var wg sync.WaitGroup
		for _, i := range indices {
			i := i
			wg.Add(1)
			go func() {
				defer wg.Done()
				runProbe(i)
			}()
		}
		wg.Wait()
	}

	// CPU (multi-core SHA-256 stress) and Memory (self-sizes its buffers to a
	// share of *currently* available memory) each assume they own the whole
	// machine's spare CPU/RAM budget while measuring, so they run in isolation.
	// Network's bandwidth speed test would saturate the link and skew the
	// latency-sensitive probes' RTT readings, so it also runs alone. Everything
	// else is either local/cheap or a modest, self-limited network check, and
	// none of it meaningfully perturbs the others, so it runs concurrently.
	runGroup(idxCPU)
	runGroup(idxMemory)
	runGroup(idxSystem, idxDisk, idxIPQuality, idxServices, idxChinaLatency, idxRoute)
	runGroup(idxNetwork)

	report.Sections = append(report.Sections, sections...)
	if cliMode {
		fmt.Fprintln(os.Stdout)
	}

	report.EndedAt = time.Now()
	total, count := 0, 0
	for _, s := range report.Sections {
		if s.Status != model.StatusSkipped {
			total += s.Score
			count++
		}
	}
	if count > 0 {
		report.Score = total / count
	}
	report.Stars = max(1, min(5, (report.Score+1999)/2000))

	saveErr := r.Store.Save(report)
	if !cliMode {
		log.Printf("[%s] benchmark completed score=%d stars=%d duration=%.1fs", report.ID, report.Score, report.Stars, report.EndedAt.Sub(report.StartedAt).Seconds())
	}
	r.Hub.Publish(model.Event{Type: "completed", ReportID: report.ID, Message: "Benchmark completed"})

	localPath := filepath.Join(r.Store.DataRoot(), report.ID+".json")
	if cliMode && !jsonMode {
		cli.PrintSummary(os.Stdout, report)
	}

	reportURL, uploadErr := upload.TryUpload(ctx, report)
	if cliMode && !jsonMode {
		local := ""
		if saveErr == nil {
			local = localPath
		}
		cli.PrintUploadResult(os.Stdout, reportURL, uploadErr, local)
		if saveErr != nil {
			fmt.Fprintf(os.Stdout, "本地保存 : 失败 (%v)\n\n", saveErr)
		}
	}

	return report, saveErr
}

func resolveHostProvider(opts model.Options) string {
	if v := strings.TrimSpace(opts.HostProvider); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("GSPROBE_HOST_PROVIDER"))
}
