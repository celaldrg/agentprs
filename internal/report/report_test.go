package report

import (
	"math"
	"testing"
	"time"

	"github.com/celaldrg/agentprs/internal/gh"
)

var t0 = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

func pr(login, typ, body string, created time.Time, mergedAfter, closedAfter time.Duration) gh.PR {
	p := gh.PR{Body: body, CreatedAt: created, State: "open", HTMLURL: "https://github.com/o/r/pull/1"}
	p.User.Login, p.User.Type = login, typ
	if mergedAfter > 0 {
		m := created.Add(mergedAfter)
		p.MergedAt, p.ClosedAt, p.State = &m, &m, "closed"
	} else if closedAfter > 0 {
		c := created.Add(closedAfter)
		p.ClosedAt, p.State = &c, "closed"
	}
	return p
}

func TestBuild(t *testing.T) {
	var prs []gh.PR
	// 6 Copilot PRs: 3 merged after 2h, 3 closed after 10h
	for i := 0; i < 3; i++ {
		prs = append(prs, pr("Copilot", "Bot", "", t0, 2*time.Hour, 0))
		prs = append(prs, pr("Copilot", "Bot", "", t0, 0, 10*time.Hour))
	}
	// 2 Codex PRs (under a human account), both merged
	prs = append(prs, pr("alice", "User", "https://chatgpt.com/codex/tasks/task_e_1", t0.AddDate(0, 1, 0), time.Hour, 0))
	prs = append(prs, pr("alice", "User", "https://chatgpt.com/codex/tasks/task_e_2", t0.AddDate(0, 1, 0), 3*time.Hour, 0))
	// 10 human PRs: 9 merged after 24h, 1 open
	for i := 0; i < 9; i++ {
		prs = append(prs, pr("bob", "User", "", t0.AddDate(0, 1, 0), 24*time.Hour, 0))
	}
	prs = append(prs, pr("bob", "User", "", t0.AddDate(0, 1, 0), 0, 0))
	// 2 dependabot
	prs = append(prs, pr("dependabot[bot]", "Bot", "", t0, time.Hour, 0), pr("dependabot[bot]", "Bot", "", t0, time.Hour, 0))

	r := Build("o/r", prs, t0.AddDate(0, 2, 0))
	if r.Total != 20 {
		t.Fatalf("total %d", r.Total)
	}
	if len(r.Agents) != 2 || r.Agents[0].Name != "Copilot" || r.Agents[1].Name != "Codex" {
		t.Fatalf("agents %+v", r.Agents)
	}
	c := r.Agents[0]
	if c.PRs != 6 || c.Merged != 3 || c.Closed != 3 || c.MergeRate != 0.5 || c.MedianMergeH != 2 || c.MedianCloseH != 10 {
		t.Fatalf("copilot %+v", c)
	}
	if r.Agents[1].MergeRate != -1 { // only 2 decided PRs: below MinDecided
		t.Fatalf("codex rate should be hidden, got %v", r.Agents[1].MergeRate)
	}
	a := r.AllAgents
	if a.PRs != 8 || a.Merged != 5 || math.Abs(a.MergeRate-5.0/8) > 1e-9 || math.Abs(a.Share-0.4) > 1e-9 {
		t.Fatalf("all agents %+v", a)
	}
	h := r.Humans
	if h.PRs != 10 || h.Open != 1 || h.MergeRate != 1 || h.MedianMergeH != 24 {
		t.Fatalf("humans %+v", h)
	}
	if r.DepsBots.PRs != 2 {
		t.Fatalf("deps %+v", r.DepsBots)
	}
	gap, ok := r.Gap()
	if !ok || math.Abs(gap-(-37.5)) > 1e-9 {
		t.Fatalf("gap %v %v", gap, ok)
	}
	if len(r.Monthly) != 2 || r.Monthly[0].Month != "2026-09" || r.Monthly[0].Agents != 6 || r.Monthly[1].Agents != 2 {
		t.Fatalf("monthly %+v", r.Monthly)
	}
	if r.From != "2026-09-01" || r.To != "2026-10-01" {
		t.Fatalf("range %s..%s", r.From, r.To)
	}
}

func TestEmpty(t *testing.T) {
	r := Build("o/r", nil, t0)
	if r.Total != 0 || r.AllAgents.MergeRate != -1 || len(r.Monthly) != 0 {
		t.Fatalf("%+v", r)
	}
	if _, ok := r.Gap(); ok {
		t.Fatal("gap should be unavailable")
	}
}

func TestMedian(t *testing.T) {
	if median([]float64{3, 1, 2}) != 2 || median([]float64{4, 1, 3, 2}) != 2.5 || median(nil) != -1 {
		t.Fatal("median")
	}
}
