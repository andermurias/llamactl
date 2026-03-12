// Package updater checks for new llamactl releases on GitHub.
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	// GitHubRepo is the owner/repo for GitHub API calls.
	GitHubRepo = "andermurias/llamactl"
	releaseURL = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
)

// Release represents a minimal GitHub release object.
type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Name    string `json:"name"`
}

var httpClient = &http.Client{Timeout: 5 * time.Second}

// CheckLatest fetches the latest GitHub release and compares it to currentVersion.
// Returns (latestRelease, hasUpdate, error).
// A version "dev" is never considered outdated.
func CheckLatest(currentVersion string) (Release, bool, error) {
	if currentVersion == "dev" {
		return Release{}, false, nil
	}
	return checkLatestFromURL(currentVersion, releaseURL)
}

// checkLatestFromURL is the testable core of CheckLatest.
func checkLatestFromURL(currentVersion, url string) (Release, bool, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Release{}, false, err
	}
	req.Header.Set("User-Agent", "llamactl/"+currentVersion)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return Release{}, false, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return Release{}, false, fmt.Errorf("GitHub API status %d", resp.StatusCode)
	}

	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, false, err
	}

	hasUpdate := rel.TagName != "" &&
		normalize(rel.TagName) != normalize(currentVersion)

	return rel, hasUpdate, nil
}

// normalize strips a leading "v" so "v1.2.3" == "1.2.3".
func normalize(v string) string {
	return strings.TrimPrefix(v, "v")
}
