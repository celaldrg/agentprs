package gh

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseRepo(t *testing.T) {
	for in, want := range map[string]string{
		"vercel/next.js":                      "vercel/next.js",
		"https://github.com/astral-sh/uv":     "astral-sh/uv",
		"https://github.com/astral-sh/uv.git": "astral-sh/uv",
		"https://github.com/a/b/":             "a/b",
	} {
		o, r, err := ParseRepo(in)
		if err != nil || o+"/"+r != want {
			t.Errorf("ParseRepo(%q) = %s/%s, %v; want %s", in, o, r, err, want)
		}
	}
	if _, _, err := ParseRepo("not a repo"); err == nil {
		t.Error("expected error")
	}
}

func TestPullsPaginatesAndStopsAtSince(t *testing.T) {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if r.Header.Get("Authorization") != "Bearer t0k" {
			t.Errorf("missing auth header")
		}
		w.Write([]byte("["))
		for i := 0; i < 100; i++ {
			n := (page-1)*100 + i
			if i > 0 {
				w.Write([]byte(","))
			}
			created := base.Add(-time.Duration(n) * time.Hour).Format(time.RFC3339)
			fmt.Fprintf(w, `{"number":%d,"created_at":%q,"user":{"login":"u","type":"User"}}`, 1000-n, created)
		}
		w.Write([]byte("]"))
	}))
	defer srv.Close()
	c := &Client{Base: srv.URL, Token: "t0k", HTTP: srv.Client()}

	prs, err := c.Pulls("o", "r", 250, time.Time{})
	if err != nil || len(prs) != 250 {
		t.Fatalf("limit: got %d, %v", len(prs), err)
	}
	prs, err = c.Pulls("o", "r", 1000, base.Add(-150*time.Hour))
	if err != nil || len(prs) != 151 {
		t.Fatalf("since: got %d, %v", len(prs), err)
	}
}

func TestRateLimitMessage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(403)
	}))
	defer srv.Close()
	c := &Client{Base: srv.URL, HTTP: srv.Client()}
	_, err := c.Pulls("o", "r", 10, time.Time{})
	if err == nil || !strings.Contains(err.Error(), "GITHUB_TOKEN") {
		t.Fatalf("got %v", err)
	}
}
