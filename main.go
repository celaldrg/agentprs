// Command agentprs shows how AI coding agents' pull requests fare in a GitHub
// repository: how many PRs they open, how often those get merged, and how
// fast, compared with human PRs.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/celaldrg/agentprs/internal/detect"
	"github.com/celaldrg/agentprs/internal/gh"
	"github.com/celaldrg/agentprs/internal/render"
	"github.com/celaldrg/agentprs/internal/report"
)

var version = "0.1.0"

const usage = `agentprs: how do AI agents' pull requests fare in this repo?

Usage:
  agentprs [owner/repo | github URL] [options]

With no repository, agentprs uses the "origin" remote of the git repo you are in.

Options:
  --limit <n>          look at the n most recent PRs (default 1000)
  --since <when>       only PRs opened after a date (2026-01-01) or age (90d, 6m, 1y)
  --json               print the full report as JSON
  --markdown           print a Markdown summary (PR comments, job summaries)
  --badge <file.svg>   write a shields-style badge
  --badge-label <s>    badge label (default "agent PRs")
  --card <file.svg>    write a shareable SVG report card
  --input <file.json>  read PRs from a saved GitHub API response instead of the network
  --list-agents        print the agents agentprs recognises and how
  --no-color           disable colors (also respects NO_COLOR)
  --version            print the version

Set GITHUB_TOKEN (or GH_TOKEN) to raise the API limit from 60 to 5,000 requests/hour.
A token with no scopes is enough for public repositories.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("agentprs", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	limit := fs.Int("limit", 1000, "")
	since := fs.String("since", "", "")
	asJSON := fs.Bool("json", false, "")
	asMD := fs.Bool("markdown", false, "")
	badge := fs.String("badge", "", "")
	badgeLabel := fs.String("badge-label", "agent PRs", "")
	card := fs.String("card", "", "")
	input := fs.String("input", "", "")
	listAgents := fs.Bool("list-agents", false, "")
	noColor := fs.Bool("no-color", false, "")
	showVersion := fs.Bool("version", false, "")
	help := fs.Bool("help", false, "")
	fs.BoolVar(help, "h", false, "")

	// allow flags after the positional argument
	var positional []string
	rest := args
	for len(rest) > 0 {
		if err := fs.Parse(rest); err != nil {
			fmt.Fprintf(stderr, "agentprs: %v\n\n%s", err, usage)
			return 2
		}
		rest = fs.Args()
		if len(rest) > 0 {
			positional = append(positional, rest[0])
			rest = rest[1:]
		}
	}
	switch {
	case *help:
		fmt.Fprint(stdout, usage)
		return 0
	case *showVersion:
		fmt.Fprintln(stdout, "agentprs", version)
		return 0
	case *listAgents:
		printAgents(stdout)
		return 0
	}
	if len(positional) > 1 {
		fmt.Fprintf(stderr, "agentprs: expected one repository, got %d\n", len(positional))
		return 2
	}
	if *limit < 1 {
		fmt.Fprintln(stderr, "agentprs: --limit must be at least 1")
		return 2
	}
	now := time.Now()
	sinceT, err := parseSince(*since, now)
	if err != nil {
		fmt.Fprintf(stderr, "agentprs: %v\n", err)
		return 2
	}

	var repoArg string
	if len(positional) == 1 {
		repoArg = positional[0]
	} else if *input == "" {
		repoArg, err = originRepo()
		if err != nil {
			fmt.Fprintf(stderr, "agentprs: %v\n\n%s", err, usage)
			return 2
		}
	}

	var prs []gh.PR
	name := repoArg
	if *input != "" {
		prs, err = readInput(*input, *limit, sinceT)
		if err != nil {
			fmt.Fprintf(stderr, "agentprs: %v\n", err)
			return 1
		}
		if name == "" {
			name = strings.TrimSuffix(*input, ".json")
		}
	} else {
		owner, repo, err := gh.ParseRepo(repoArg)
		if err != nil {
			fmt.Fprintf(stderr, "agentprs: %v\n", err)
			return 2
		}
		name = owner + "/" + repo
		c := gh.NewClient()
		interactive := isTerminal(stderr)
		if interactive {
			c.Progress = func(n int) { fmt.Fprintf(stderr, "\rfetching pull requests… %d", n) }
		}
		prs, err = c.Pulls(owner, repo, *limit, sinceT)
		if interactive {
			fmt.Fprint(stderr, "\r\x1b[K")
		}
		if err != nil {
			if errors.Is(err, gh.ErrRateLimited) && len(prs) > 0 {
				fmt.Fprintf(stderr, "agentprs: %v (showing the %d PRs fetched so far)\n", err, len(prs))
			} else {
				fmt.Fprintf(stderr, "agentprs: %v\n", err)
				return 1
			}
		}
	}

	r := report.Build(name, prs, now)
	if *badge != "" {
		if err := os.WriteFile(*badge, []byte(render.Badge(*badgeLabel, render.BadgeValue(r))), 0o644); err != nil {
			fmt.Fprintf(stderr, "agentprs: %v\n", err)
			return 1
		}
	}
	if *card != "" {
		if err := os.WriteFile(*card, []byte(render.Card(r)), 0o644); err != nil {
			fmt.Fprintf(stderr, "agentprs: %v\n", err)
			return 1
		}
	}
	switch {
	case *asJSON:
		if err := render.JSON(stdout, r); err != nil {
			fmt.Fprintf(stderr, "agentprs: %v\n", err)
			return 1
		}
	case *asMD:
		render.Markdown(stdout, r)
	default:
		force := os.Getenv("FORCE_COLOR")
		color := !*noColor && ((force != "" && force != "0" && force != "false") || (os.Getenv("NO_COLOR") == "" && isTerminal(stdout)))
		render.Text(stdout, r, color)
	}
	return 0
}

var ageRe = regexp.MustCompile(`^(\d+)([dwmy])$`)

func parseSince(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if m := ageRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		switch m[2] {
		case "d":
			return now.AddDate(0, 0, -n), nil
		case "w":
			return now.AddDate(0, 0, -7*n), nil
		case "m":
			return now.AddDate(0, -n, 0), nil
		default:
			return now.AddDate(-n, 0, 0), nil
		}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("--since: expected a date like 2026-01-01 or an age like 90d, 6m, 1y; got %q", s)
}

func originRepo() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", errors.New("no repository given and no git \"origin\" remote here")
	}
	u := strings.TrimSpace(string(out))
	u = strings.TrimPrefix(u, "git@github.com:")
	u = strings.TrimPrefix(u, "ssh://git@github.com/")
	if !strings.Contains(u, "github.com") {
		u = "https://github.com/" + u
	}
	if _, _, err := gh.ParseRepo(u); err != nil {
		return "", fmt.Errorf("origin remote %q is not a GitHub repository", strings.TrimSpace(string(out)))
	}
	return u, nil
}

func readInput(path string, limit int, since time.Time) ([]gh.PR, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var prs []gh.PR
	if err := json.Unmarshal(b, &prs); err != nil {
		return nil, fmt.Errorf("%s: expected a JSON array of pull requests: %v", path, err)
	}
	var out []gh.PR
	for _, p := range prs {
		if !since.IsZero() && p.CreatedAt.Before(since) {
			continue
		}
		out = append(out, p)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func printAgents(w io.Writer) {
	fmt.Fprintln(w, "agentprs recognises these agents. It only trusts signatures the tools leave on purpose.")
	fmt.Fprintln(w)
	for _, a := range detect.Agents {
		var how []string
		for _, l := range a.Logins {
			how = append(how, "author @"+l)
		}
		for _, m := range a.Markers {
			how = append(how, fmt.Sprintf("body contains %q", m))
		}
		fmt.Fprintf(w, "  %-12s %s\n", a.Name, strings.Join(how, ", "))
	}
	fmt.Fprintln(w, "\nDependency bots (reported separately):", strings.Join(detect.DepsBots, ", "))
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}
