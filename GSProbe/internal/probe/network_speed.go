package probe

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type speedTestResult struct {
	DownloadMbps float64 `json:"downloadMbps,omitempty"`
	UploadMbps   float64 `json:"uploadMbps,omitempty"`
	DownloadText string  `json:"downloadText"`
	UploadText   string  `json:"uploadText"`
}

func probeSpeedTest(ctx context.Context) speedTestResult {
	downloadBytes := int64(25_000_000)
	uploadBytes := int64(10_000_000)
	client := &http.Client{Timeout: 45 * time.Second}
	out := speedTestResult{}
	if mbps, ok := cloudflareDownloadMbps(ctx, client, downloadBytes); ok {
		out.DownloadMbps = mbps
		out.DownloadText = fmt.Sprintf("%.1f Mbps", mbps)
	} else {
		out.DownloadText = "失败"
	}
	if mbps, ok := cloudflareUploadMbps(ctx, client, uploadBytes); ok {
		out.UploadMbps = mbps
		out.UploadText = fmt.Sprintf("%.1f Mbps", mbps)
	} else {
		out.UploadText = "失败"
	}
	return out
}

func cloudflareDownloadMbps(ctx context.Context, client *http.Client, bytes int64) (float64, bool) {
	url := fmt.Sprintf("https://speed.cloudflare.com/__down?bytes=%d", bytes)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false
	}
	n, err := io.Copy(io.Discard, io.LimitReader(resp.Body, bytes))
	if err != nil || n == 0 {
		return 0, false
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0, false
	}
	return float64(n) * 8 / elapsed / 1_000_000, true
}

func cloudflareUploadMbps(ctx context.Context, client *http.Client, size int64) (float64, bool) {
	payload := make([]byte, size)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://speed.cloudflare.com/__up", bytes.NewReader(payload))
	if err != nil {
		return 0, false
	}
	req.ContentLength = size
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false
	}
	elapsed := time.Since(start).Seconds()
	if elapsed <= 0 {
		return 0, false
	}
	return float64(size) * 8 / elapsed / 1_000_000, true
}
