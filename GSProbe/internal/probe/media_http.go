package probe

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

const mediaUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

func newMediaClient(timeout time.Duration, followRedirects bool) *http.Client {
	checkRedirect := func(_ *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		return nil
	}
	if !followRedirects {
		checkRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return &http.Client{Timeout: timeout, CheckRedirect: checkRedirect}
}

func mediaDomain(rawURL string) string {
	domain := strings.TrimPrefix(strings.TrimPrefix(rawURL, "https://"), "http://")
	return strings.Split(domain, "/")[0]
}

func mediaGET(ctx context.Context, client *http.Client, rawURL string) (status int, body string, headers http.Header, err error) {
	return mediaGETWithHeaders(ctx, client, rawURL, nil)
}

func mediaGETWithHeaders(ctx context.Context, client *http.Client, rawURL string, extraHeaders map[string]string) (status int, body string, headers http.Header, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", nil, err
	}
	req.Header.Set("User-Agent", mediaUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	return resp.StatusCode, string(b), resp.Header.Clone(), nil
}

func mediaPOST(ctx context.Context, client *http.Client, rawURL, contentType, payload string, extraHeaders map[string]string) (status int, body string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, strings.NewReader(payload))
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", mediaUserAgent)
	req.Header.Set("Content-Type", contentType)
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	return resp.StatusCode, string(b), nil
}

func traceCountry(ctx context.Context, client *http.Client) string {
	for _, u := range []string{
		"https://chatgpt.com/cdn-cgi/trace",
		"https://chat.openai.com/cdn-cgi/trace",
		"https://www.cloudflare.com/cdn-cgi/trace",
	} {
		_, body, _, err := mediaGET(ctx, client, u)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(body, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "loc=") {
				return strings.TrimPrefix(line, "loc=")
			}
		}
	}
	return ""
}
