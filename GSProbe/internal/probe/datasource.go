package probe

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	dataPrimaryBase  = "https://dl.gsvps.com"
	dataFallbackBase = "https://github.com/gsvps/GSNode/raw/main"
	dataCDNBase      = "https://cdn.jsdelivr.net/gh/gsvps/GSNode@main"
)

func dataFileURLs(name string, extras ...string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 3+len(extras))
	add := func(u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			return
		}
		if _, ok := seen[u]; ok {
			return
		}
		seen[u] = struct{}{}
		out = append(out, u)
	}
	add(dataPrimaryBase + "/" + name)
	add(dataFallbackBase + "/" + name)
	add(dataCDNBase + "/" + name)
	for _, u := range extras {
		add(u)
	}
	return out
}

func fetchBytesFirstOK(ctx context.Context, client *http.Client, urls []string, limit int64) ([]byte, string, error) {
	if client == nil {
		client = http.DefaultClient
	}
	var lastErr error
	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", ipQualityUA)
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("http %d", resp.StatusCode)
			continue
		}
		if len(body) == 0 {
			lastErr = fmt.Errorf("empty body")
			continue
		}
		return body, url, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no urls")
	}
	return nil, "", lastErr
}

func fetchTextFirstOK(ctx context.Context, client *http.Client, urls []string) (string, string) {
	raw, _, err := fetchBytesFirstOK(ctx, client, urls, 2<<20)
	if err != nil {
		return "", ""
	}
	return string(raw), ""
}

func pingTargetsURLs() []string {
	if u := strings.TrimSpace(os.Getenv("GSPROBE_PING_TARGETS_URL")); u != "" {
		return []string{u}
	}
	return dataFileURLs("ping_targets.json")
}

func dnsblListURLs() []string {
	if u := strings.TrimSpace(os.Getenv("GSPROBE_DNSBL_URL")); u != "" {
		return []string{u}
	}
	return dataFileURLs("dnsbl.list", "https://raw.githubusercontent.com/xykt/IPQuality/main/ref/dnsbl.list")
}
