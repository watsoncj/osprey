// Package selfupdate checks GitHub Releases for a newer version of the
// running binary and replaces it in place.
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

const repo = "watsoncj/osprey"

// Component identifies which binary to update.
type Component string

const (
	Agent  Component = "agent"
	Server Component = "server"
)

// githubRelease is the subset of the GitHub API response we need.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// assetName returns the expected release asset filename.
func assetName(component Component) string {
	name := "osprey-" + string(component) + "-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// Check queries GitHub for the latest release and reports whether an update
// is available. Returns the download URL and new version, or empty strings
// if no update is needed.
func Check(ctx context.Context, currentVersion string, component Component, client *http.Client) (downloadURL, newVersion string, err error) {
	cur, err := parseSemver(currentVersion)
	if err != nil {
		return "", "", fmt.Errorf("current version %q is not valid semver: %w", currentVersion, err)
	}

	rel, err := fetchLatestRelease(ctx, client)
	if err != nil {
		return "", "", err
	}

	latest, err := parseSemver(rel.TagName)
	if err != nil {
		return "", "", fmt.Errorf("latest release tag %q is not valid semver: %w", rel.TagName, err)
	}

	if !latest.newerThan(cur) {
		return "", "", nil
	}

	want := assetName(component)
	for _, a := range rel.Assets {
		if a.Name == want {
			return a.BrowserDownloadURL, rel.TagName, nil
		}
	}
	return "", "", fmt.Errorf("release %s has no asset %q", rel.TagName, want)
}

// Apply downloads the new binary and replaces the running executable.
// On success, the caller should exit (or restart) so the new version runs.
func Apply(ctx context.Context, downloadURL string, client *http.Client) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable symlinks: %w", err)
	}

	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}

	return applyBinary(exe, resp.Body)
}

// CheckAndApply is a convenience that checks for an update and applies it
// if available. Returns the new version string if an update was applied,
// or empty string if already up to date.
func CheckAndApply(ctx context.Context, currentVersion string, component Component, client *http.Client) (string, error) {
	if currentVersion == "dev" {
		log.Printf("selfupdate: skipping update check (dev build)")
		return "", nil
	}

	url, newVersion, err := Check(ctx, currentVersion, component, client)
	if err != nil {
		return "", fmt.Errorf("check for update: %w", err)
	}
	if url == "" {
		return "", nil
	}

	log.Printf("selfupdate: updating from %s to %s", currentVersion, newVersion)
	if err := Apply(ctx, url, client); err != nil {
		return "", fmt.Errorf("apply update: %w", err)
	}
	return newVersion, nil
}

func fetchLatestRelease(ctx context.Context, client *http.Client) (*githubRelease, error) {
	if client == nil {
		client = http.DefaultClient
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "osprey-selfupdate")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API returned %s: %s", resp.Status, body)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &rel, nil
}
