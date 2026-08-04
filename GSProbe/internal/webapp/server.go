package webapp

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gsprobe/internal/model"
	"gsprobe/internal/runner"
)

//go:embed web/*
var assets embed.FS

type Server struct{ Runner *runner.Runner }

func New(r *runner.Runner) *Server { return &Server{Runner: r} }
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(assets, "web")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /report/{id}", s.index)
	mux.HandleFunc("GET /api/reports", s.list)
	mux.HandleFunc("GET /api/reports/latest", s.latest)
	mux.HandleFunc("GET /api/reports/{id}", s.get)
	mux.HandleFunc("GET /api/reports/{id}/markdown", s.markdown)
	mux.HandleFunc("POST /api/run", s.run)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /api/events", s.events)
	return security(mux)
}
func security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
func (s *Server) index(w http.ResponseWriter, _ *http.Request) {
	b, _ := assets.ReadFile("web/index.html")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
func (s *Server) list(w http.ResponseWriter, _ *http.Request) {
	rows, e := s.Runner.Store.List()
	if e != nil {
		http.Error(w, e.Error(), 500)
		return
	}
	for i := range rows {
		rows[i].Sections = nil
	}
	jsonOut(w, rows)
}
func (s *Server) latest(w http.ResponseWriter, _ *http.Request) {
	r, e := s.Runner.Store.Latest()
	if e != nil {
		jsonOut(w, nil)
		return
	}
	jsonOut(w, r)
}
func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	x, e := s.Runner.Store.Get(r.PathValue("id"))
	if e != nil {
		http.Error(w, "report not found", 404)
		return
	}
	jsonOut(w, x)
}
func (s *Server) run(w http.ResponseWriter, r *http.Request) {
	if s.Runner.IsRunning() {
		http.Error(w, "benchmark already running", 409)
		return
	}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("benchmark panic: %v", rec)
			}
		}()
		_, _ = s.Runner.Run(context.Background(), model.Options{DataDir: s.Runner.Store.DataRoot()})
	}()
	w.WriteHeader(http.StatusAccepted)
	jsonOut(w, map[string]any{"ok": true})
}
func writeSSE(w http.ResponseWriter, f http.Flusher, format string, args ...any) bool {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return false
	}
	f.Flush()
	return true
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	ch, done := s.Runner.Hub.Subscribe()
	defer done()
	if !writeSSE(w, f, "event: ready\ndata: {}\n\n") {
		return
	}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			b, err := json.Marshal(e)
			if err != nil {
				continue
			}
			if !writeSSE(w, f, "event: %s\ndata: %s\n\n", e.Type, b) {
				return
			}
		case <-ticker.C:
			if !writeSSE(w, f, ": ping\n\n") {
				return
			}
		}
	}
}
func (s *Server) markdown(w http.ResponseWriter, r *http.Request) {
	x, e := s.Runner.Store.Get(r.PathValue("id"))
	if e != nil {
		http.Error(w, "not found", 404)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=gsprobe-"+x.ID+".md")
	fmt.Fprintf(w, "# GSProbe Report %s\n\n- Host: %s\n- Platform: %s\n- Score: %d/10000\n- Stars: %s\n\n", x.ID, x.Hostname, x.Platform, x.Score, strings.Repeat("★", x.Stars))
	for _, sec := range x.Sections {
		fmt.Fprintf(w, "## %s — %d/10000\n\n%s\n\n", sec.Name, sec.Score, sec.Summary)
		for _, m := range sec.Metrics {
			value := m.Text
			if value == "" {
				value = strconv.FormatFloat(m.Value, 'f', 2, 64) + " " + m.Unit
			}
			fmt.Fprintf(w, "- %s: %s\n", m.Name, value)
		}
		if sec.ID == "ip-quality" {
			writeIPQualityMarkdown(w, sec.Details)
		}
		if sec.ID == "route" {
			if routes, ok := sec.Details["returnRoutes"].([]any); ok {
				fmt.Fprintln(w, "\n### 三地三网回程线路及 Ping 延迟")
				for _, raw := range routes {
					route, ok := raw.(map[string]any)
					if !ok {
						continue
					}
					fmt.Fprintf(w, "\n#### %v %v — %v — %v\n\n目标：`%v`\n", route["city"], route["carrier"], route["line"], route["pingText"], route["address"])
					if hops, ok := route["hops"].([]any); ok && len(hops) > 0 {
						path := make([]string, 0, len(hops)+1)
						path = append(path, "本机")
						for _, hop := range hops {
							path = append(path, fmt.Sprint(hop))
						}
						fmt.Fprintf(w, "\n线路图：`%s`\n", strings.Join(path, " → "))
					}
					fmt.Fprintf(w, "\n```text\n%v\n```\n", route["output"])
				}
			}
		}
		fmt.Fprintln(w)
	}
}

func writeIPQualityMarkdown(w http.ResponseWriter, details map[string]any) {
	base, _ := details["base"].(map[string]any)
	fmt.Fprintln(w, "\n### 一、基础信息")
	for _, row := range [][2]string{{"自治系统号", "asn"}, {"组织", "organization"}, {"ISP", "isp"}, {"坐标", "coordinates"}, {"城市", "city"}, {"使用地", "country"}, {"时区", "timezone"}, {"IP类型", "ipType"}} {
		fmt.Fprintf(w, "- %s：%v\n", row[0], base[row[1]])
	}
	if purity, ok := details["ipPurity"].(map[string]any); ok {
		fmt.Fprintf(w, "- IP纯净度：%v（越高越纯净）\n", purity["text"])
	}
	fmt.Fprintln(w, "\n### 二、IP 类型属性\n\n| 数据库 | 地区 | 城市 | ASN | 类型 |\n|---|---|---|---|---|")
	if sources, ok := details["sources"].([]any); ok {
		for _, raw := range sources {
			x, _ := raw.(map[string]any)
			fmt.Fprintf(w, "| %v | %v | %v | %v | %v |\n", x["name"], x["country"], x["city"], x["asn"], x["type"])
		}
	}
	fmt.Fprintln(w, "\n### 三、风险评分")
	if scores, ok := details["riskScores"].(map[string]any); ok {
		for name, value := range scores {
			fmt.Fprintf(w, "- %s：%v\n", name, value)
		}
	}
	fmt.Fprintln(w, "\n> 未配置商业风险库 API 密钥时显示无数据，不推测评分。")
	fmt.Fprintln(w, "\n### 四、风险因子")
	if factors, ok := details["riskFactors"].(map[string]any); ok {
		for name, value := range factors {
			fmt.Fprintf(w, "- %s：%v\n", name, value)
		}
	}
	fmt.Fprintln(w, "\n### 五、邮局连通性及黑名单")
	if mail, ok := details["mail"].(map[string]any); ok {
		fmt.Fprintf(w, "- 本地 25 端口出站：%v\n", mail["outbound25"])
	}
	if blacklist, ok := details["blacklist"].(map[string]any); ok {
		fmt.Fprintf(w, "- DNSBL：有效 %v，正常 %v，已标记 %v\n", blacklist["valid"], blacklist["normal"], blacklist["listed"])
	}
}
