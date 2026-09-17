package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckLatest(t *testing.T) {
	var gotPath, gotAccept, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     "v1.4.2",
			"html_url":     "https://example.com/releases/v1.4.2",
			"published_at": "2026-01-02T03:04:05Z",
		})
	}))
	defer srv.Close()

	old := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = old }()

	rel, err := CheckLatest(context.Background(), "chafficui", "jaqen-newgen-tool")
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "1.4.2" {
		t.Errorf("Version = %q, want 1.4.2", rel.Version)
	}
	if rel.URL != "https://example.com/releases/v1.4.2" {
		t.Errorf("URL = %q", rel.URL)
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if !rel.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", rel.PublishedAt, want)
	}

	if gotPath != "/repos/chafficui/jaqen-newgen-tool/releases/latest" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Errorf("Accept header = %q", gotAccept)
	}
	if gotUA != "fm-newgen-faces" {
		t.Errorf("User-Agent header = %q", gotUA)
	}
}

func TestCheckLatestNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	old := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = old }()

	if _, err := CheckLatest(context.Background(), "o", "r"); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestCheckLatestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block only until the client gives up (context canceled), never
		// forever, so the test server can still shut down cleanly.
		<-r.Context().Done()
	}))
	defer srv.Close()

	old := apiBaseURL
	apiBaseURL = srv.URL
	defer func() { apiBaseURL = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := CheckLatest(ctx, "o", "r"); err == nil {
		t.Fatal("expected a context-deadline error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("CheckLatest took too long to time out: %v", elapsed)
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.10.0", "1.9.0", true},
		{"1.9.0", "1.10.0", false},
		{"1.2.3", "1.2", true},
		{"1.2", "1.2.3", false},
		{"1.2.3", "1.2.3", false},
		{"v2.0.0", "1.9.9", true},
		{"1.0.0", "dev", false},
		{"1.0.0", "", false},
		{"2.0.0", "v1.0.0", true},
		{"1.0.0", "1.0.0-beta", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.latest, c.current); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.latest, c.current, got, c.want)
		}
	}
}
