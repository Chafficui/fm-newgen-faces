// Package update checks GitHub releases for a newer version.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Release is the latest published release.
type Release struct {
	Version     string // tag without leading "v"
	URL         string // html_url
	PublishedAt time.Time
}

// apiBaseURL is the GitHub API root. It is a variable so tests can point it
// at an httptest.Server.
var apiBaseURL = "https://api.github.com"

// CheckLatest calls https://api.github.com/repos/<owner>/<repo>/releases/latest
// with a short timeout. Pre-releases and drafts are not returned by that endpoint.
func CheckLatest(ctx context.Context, owner, repo string) (Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", apiBaseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "fm-newgen-faces")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Release{}, fmt.Errorf("update: unexpected status %s", resp.Status)
	}

	var body struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Release{}, err
	}

	return Release{
		Version:     strings.TrimPrefix(body.TagName, "v"),
		URL:         body.HTMLURL,
		PublishedAt: body.PublishedAt,
	}, nil
}

// IsNewer compares dotted versions ("1.2.3" > "1.2"); "dev" and empty current
// are never outdated; leading "v" is ignored.
func IsNewer(latest, current string) bool {
	trimmedCurrent := strings.TrimSpace(current)
	if trimmedCurrent == "" || trimmedCurrent == "dev" {
		return false
	}

	lp := parseVersion(latest)
	cp := parseVersion(current)

	n := len(lp)
	if len(cp) > n {
		n = len(cp)
	}
	for i := 0; i < n; i++ {
		var l, c int
		if i < len(lp) {
			l = lp[i]
		}
		if i < len(cp) {
			c = cp[i]
		}
		if l != c {
			return l > c
		}
	}
	return false
}

// parseVersion splits a dotted version string into numeric components,
// ignoring a leading "v"/"V" and any non-numeric suffix on a component
// (e.g. "3-beta" -> 3).
func parseVersion(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		end := 0
		for end < len(p) && p[end] >= '0' && p[end] <= '9' {
			end++
		}
		if end == 0 {
			out = append(out, 0)
			continue
		}
		n, _ := strconv.Atoi(p[:end])
		out = append(out, n)
	}
	return out
}
