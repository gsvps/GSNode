package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"gsprobe/internal/model"
	"gsprobe/internal/runner"
	"gsprobe/internal/store"
	"gsprobe/internal/webapp"
)

func main() {
	addr := flag.String("addr", ":8899", "web listen address")
	data := flag.String("data", "./data", "report data directory")
	runFlag := flag.Bool("run", false, "run full benchmark")
	jsonFlag := flag.Bool("json", false, "with -run, print full JSON instead of terminal summary")
	providerFlag := flag.String("provider", "", "optional host provider name (max 64 chars)")
	flag.Parse()
	abs, _ := filepath.Abs(*data)
	st, err := store.New(abs)
	if err != nil {
		log.Fatal(err)
	}
	hub := runner.NewHub()
	run := runner.New(st, hub)
	if *runFlag {
		r, e := run.Run(context.Background(), model.Options{DataDir: abs, HostProvider: *providerFlag})
		if e != nil {
			log.Fatal(e)
		}
		if *jsonFlag {
			b, _ := json.MarshalIndent(r, "", "  ")
			fmt.Println(string(b))
		}
		return
	}
	srv := &http.Server{
		Addr:              *addr,
		Handler:           webapp.New(run).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		log.Printf("GSProbe %s listening on http://0.0.0.0%s (data: %s)", runner.Version, *addr, abs)
		if e := srv.ListenAndServe(); e != nil && e != http.ErrServerClosed {
			log.Fatal(e)
		}
	}()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
