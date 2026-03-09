package upload

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/watsoncj/osprey/internal/model"
)

// InsecureClient returns an HTTP client that skips TLS certificate verification.
func InsecureClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// Upload POSTs a Submission as JSON to the server's /api/visits endpoint.
// The hostname is sent in the X-Hostname header. If client is nil,
// http.DefaultClient is used.
func Upload(ctx context.Context, serverURL, hostname string, sub model.Submission, apiKey string, client *http.Client) error {
	body, err := json.Marshal(sub)
	if err != nil {
		return fmt.Errorf("marshal submission: %w", err)
	}

	url := serverURL + "/api/visits"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hostname", hostname)
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload report: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	return nil
}
