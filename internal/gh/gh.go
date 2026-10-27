// Package gh fetches pull requests from the GitHub REST API using only the
// standard library.
package gh

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// PR is one pull request as returned by GET /repos/{owner}/{repo}/pulls.
type PR struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	HTMLURL   string     `json:"html_url"`
	State     string     `json:"state"`
	Draft     bool       `json:"draft"`
	Body      string     `json:"body"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
	MergedAt  *time.Time `json:"merged_at"`
	User      struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"user"`
	Head struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

// Client talks to api.github.com (or GITHUB_API_URL for GitHub Enterprise).
type Client struct {
	Base  string
	Token string
	HTTP  *http.Client
	// Progress, when set, is called after every page.
	Progress func(fetched int)
}

// NewClient builds a client from the environment: GITHUB_TOKEN or GH_TOKEN,
// and GITHUB_API_URL.
func NewClient() *Client {
	base := strings.TrimRight(os.Getenv("GITHUB_API_URL"), "/")
	if base == "" {
		base = "https://api.github.com"
	}
	tok := os.Getenv("GITHUB_TOKEN")
	if tok == "" {
		tok = os.Getenv("GH_TOKEN")
	}
	return &Client{Base: base, Token: tok, HTTP: &http.Client{Timeout: 60 * time.Second}}
}

var repoRe = regexp.MustCompile(`^(?:https?://github\.com/)?([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?/?$`)

// ParseRepo accepts "owner/repo" or a github.com URL.
func ParseRepo(s string) (owner, repo string, err error) {
	m := repoRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", "", fmt.Errorf("expected owner/repo or a github.com URL, got %q", s)
	}
	return m[1], m[2], nil
}

// ErrRateLimited is returned when GitHub refuses with a rate-limit error.
var ErrRateLimited = errors.New("GitHub API rate limit reached")

// Pulls returns up to limit pull requests (newest first) created at or after since.
func (c *Client) Pulls(owner, repo string, limit int, since time.Time) ([]PR, error) {
	var all []PR
	for page := 1; len(all) < limit; page++ {
		q := url.Values{}
		q.Set("state", "all")
		q.Set("sort", "created")
		q.Set("direction", "desc")
		q.Set("per_page", "100")
		q.Set("page", fmt.Sprint(page))
		u := fmt.Sprintf("%s/repos/%s/%s/pulls?%s", c.Base, owner, repo, q.Encode())
		var batch []PR
		if err := c.get(u, &batch); err != nil {
			return all, err
		}
		stop := len(batch) < 100
		for _, p := range batch {
			if !since.IsZero() && p.CreatedAt.Before(since) {
				stop = true
				break
			}
			all = append(all, p)
			if len(all) == limit {
				break
			}
		}
		if c.Progress != nil {
			c.Progress(len(all))
		}
		if stop {
			break
		}
	}
	return all, nil
}

func (c *Client) get(u string, out any) error {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "agentprs")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	var resp *http.Response
	for attempt := 0; ; attempt++ {
		resp, err = c.HTTP.Do(req)
		if err == nil && resp.StatusCode < 500 {
			break
		}
		if attempt == 2 {
			if err != nil {
				return err
			}
			break
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(time.Duration(attempt+1) * time.Second)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 || resp.StatusCode == 429 {
		if resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.StatusCode == 429 {
			hint := "set GITHUB_TOKEN (any token, no scopes needed for public repos) to get 5,000 requests/hour"
			if c.Token != "" {
				hint = "try again after " + resetTime(resp.Header.Get("X-RateLimit-Reset"))
			}
			return fmt.Errorf("%w: %s", ErrRateLimited, hint)
		}
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("repository not found (private repos need GITHUB_TOKEN with access)")
	}
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("GitHub API %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func resetTime(unix string) string {
	var sec int64
	if _, err := fmt.Sscan(unix, &sec); err != nil || sec == 0 {
		return "the limit resets"
	}
	return time.Unix(sec, 0).Format("15:04 MST")
}
