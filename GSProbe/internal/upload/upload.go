package upload

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gsprobe/internal/cli"
	"gsprobe/internal/model"
)

const defaultUploadURL = "https://www.gsvps.com/api/reports/upload"

type uploadEnvelope struct {
	model.Report
	MachineHash string `json:"machineHash,omitempty"`
}

func UploadReport(ctx context.Context, report model.Report) (string, error) {
	endpoint := strings.TrimSpace(os.Getenv("GSVPS_UPLOAD_URL"))
	if endpoint == "" {
		endpoint = defaultUploadURL
	}

	payload := uploadEnvelope{
		Report:      report,
		MachineHash: machineHash(),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GSNode/"+report.Version)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed: %s", strings.TrimSpace(string(respBody)))
	}

	var out struct {
		URL string `json:"url"`
		ID  string `json:"id"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	if out.URL != "" {
		return out.URL, nil
	}
	site := strings.TrimRight(strings.TrimSpace(os.Getenv("GSVPS_SITE_URL")), "/")
	if site == "" {
		site = "https://www.gsvps.com"
	}
	if out.ID != "" {
		return site + "/report/" + out.ID, nil
	}
	return "", fmt.Errorf("upload response missing url")
}

func machineHash() string {
	raw, err := os.ReadFile("/etc/machine-id")
	if err != nil {
		raw, err = os.ReadFile("/var/lib/dbus/machine-id")
	}
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(string(raw))
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:])
}

func TryUpload(ctx context.Context, report model.Report) (string, error) {
	if strings.EqualFold(os.Getenv("GSVPS_UPLOAD"), "0") || strings.EqualFold(os.Getenv("GSVPS_UPLOAD"), "false") {
		return "", nil
	}
	url, err := UploadReport(ctx, report)
	if err != nil {
		if !cli.IsRunMode() {
			log.Printf("[%s] upload failed: %v", report.ID, err)
		}
		return "", err
	}
	if !cli.IsRunMode() {
		log.Printf("[%s] report uploaded: %s", report.ID, url)
	}
	return url, nil
}
