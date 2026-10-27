// Package report turns a list of classified pull requests into numbers:
// share, merge rate and time to merge per agent, for all agents together, for
// humans and for bots.
package report

import (
	"sort"
	"time"

	"github.com/celaldrg/agentprs/internal/detect"
	"github.com/celaldrg/agentprs/internal/gh"
)

// MinDecided is the smallest number of merged+closed PRs a group needs before
// a merge rate is shown. Below it the rate is noise.
const MinDecided = 5

// Group holds the numbers for one row of the report.
type Group struct {
	Name          string  `json:"name"`
	PRs           int     `json:"prs"`
	Share         float64 `json:"share"` // of all PRs in the window, 0..1
	Merged        int     `json:"merged"`
	Closed        int     `json:"closed_unmerged"`
	Open          int     `json:"open"`
	MergeRate     float64 `json:"merge_rate"`            // merged / (merged+closed), -1 when too few
	MedianMergeH  float64 `json:"median_hours_to_merge"` // -1 when nothing merged
	MedianCloseH  float64 `json:"median_hours_to_close"` // closed without merge, -1 when none
	FirstSeen     string  `json:"first_seen,omitempty"`  // YYYY-MM-DD
	LastSeen      string  `json:"last_seen,omitempty"`   // YYYY-MM-DD
	ExampleURL    string  `json:"example_url,omitempty"` // most recent PR
	Evidence      string  `json:"evidence,omitempty"`    // how the most recent one was recognised
	mergeHours    []float64
	closeHours    []float64
	first, latest time.Time
}

// Month is the agent share of PRs opened in one calendar month.
type Month struct {
	Month  string  `json:"month"` // YYYY-MM
	PRs    int     `json:"prs"`
	Agents int     `json:"agent_prs"`
	Share  float64 `json:"agent_share"`
}

// Report is the full result for one repository.
type Report struct {
	Repo      string    `json:"repo"`
	Generated time.Time `json:"generated_at"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	Total     int       `json:"total_prs"`
	Agents    []*Group  `json:"agents"`     // one per agent seen, biggest first
	AllAgents *Group    `json:"all_agents"` // every agent PR together
	Humans    *Group    `json:"humans"`
	DepsBots  *Group    `json:"dependency_bots"`
	OtherBots *Group    `json:"other_bots"`
	Monthly   []Month   `json:"monthly"`
}

// Build classifies prs and computes the report. now is the reference time
// for open PRs (pass time.Now()).
func Build(repo string, prs []gh.PR, now time.Time) *Report {
	r := &Report{
		Repo: repo, Generated: now.UTC(), Total: len(prs),
		AllAgents: &Group{Name: "All agents"}, Humans: &Group{Name: "Humans"},
		DepsBots: &Group{Name: "Dependency bots"}, OtherBots: &Group{Name: "Other bots"},
	}
	r.Agents, r.Monthly = []*Group{}, []Month{}
	byAgent := map[string]*Group{}
	months := map[string]*Month{}
	var minT, maxT time.Time
	for _, p := range prs {
		res := detect.Classify(detect.PR{Login: p.User.Login, UserType: p.User.Type, HeadRef: p.Head.Ref, Body: p.Body})
		groups := []*Group{}
		switch res.Kind {
		case detect.Agent:
			g := byAgent[res.Agent]
			if g == nil {
				g = &Group{Name: res.Agent}
				byAgent[res.Agent] = g
			}
			groups = append(groups, g, r.AllAgents)
		case detect.Human:
			groups = append(groups, r.Humans)
		case detect.DepsBot:
			groups = append(groups, r.DepsBots)
		default:
			groups = append(groups, r.OtherBots)
		}
		for _, g := range groups {
			g.add(p, res.Evidence)
		}
		mk := p.CreatedAt.UTC().Format("2006-01")
		m := months[mk]
		if m == nil {
			m = &Month{Month: mk}
			months[mk] = m
		}
		m.PRs++
		if res.Kind == detect.Agent {
			m.Agents++
		}
		if minT.IsZero() || p.CreatedAt.Before(minT) {
			minT = p.CreatedAt
		}
		if p.CreatedAt.After(maxT) {
			maxT = p.CreatedAt
		}
	}
	if !minT.IsZero() {
		r.From, r.To = minT.UTC().Format("2006-01-02"), maxT.UTC().Format("2006-01-02")
	}
	for _, g := range byAgent {
		r.Agents = append(r.Agents, g)
	}
	sort.Slice(r.Agents, func(i, j int) bool {
		if r.Agents[i].PRs != r.Agents[j].PRs {
			return r.Agents[i].PRs > r.Agents[j].PRs
		}
		return r.Agents[i].Name < r.Agents[j].Name
	})
	for _, g := range append([]*Group{r.AllAgents, r.Humans, r.DepsBots, r.OtherBots}, r.Agents...) {
		g.finish(r.Total)
	}
	for _, m := range months {
		if m.PRs > 0 {
			m.Share = float64(m.Agents) / float64(m.PRs)
		}
		r.Monthly = append(r.Monthly, *m)
	}
	sort.Slice(r.Monthly, func(i, j int) bool { return r.Monthly[i].Month < r.Monthly[j].Month })
	return r
}

func (g *Group) add(p gh.PR, evidence string) {
	g.PRs++
	switch {
	case p.MergedAt != nil:
		g.Merged++
		g.mergeHours = append(g.mergeHours, p.MergedAt.Sub(p.CreatedAt).Hours())
	case p.ClosedAt != nil || p.State == "closed":
		g.Closed++
		if p.ClosedAt != nil {
			g.closeHours = append(g.closeHours, p.ClosedAt.Sub(p.CreatedAt).Hours())
		}
	default:
		g.Open++
	}
	if g.first.IsZero() || p.CreatedAt.Before(g.first) {
		g.first = p.CreatedAt
	}
	if p.CreatedAt.After(g.latest) || g.ExampleURL == "" {
		if p.CreatedAt.After(g.latest) {
			g.latest = p.CreatedAt
		}
		g.ExampleURL, g.Evidence = p.HTMLURL, evidence
	}
}

func (g *Group) finish(total int) {
	if total > 0 {
		g.Share = float64(g.PRs) / float64(total)
	}
	g.MergeRate = -1
	if d := g.Merged + g.Closed; d >= MinDecided {
		g.MergeRate = float64(g.Merged) / float64(d)
	}
	g.MedianMergeH = median(g.mergeHours)
	g.MedianCloseH = median(g.closeHours)
	if !g.first.IsZero() {
		g.FirstSeen = g.first.UTC().Format("2006-01-02")
		g.LastSeen = g.latest.UTC().Format("2006-01-02")
	}
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return -1
	}
	s := append([]float64(nil), xs...)
	sort.Float64s(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// Gap is the merge-rate difference in percentage points between agent PRs and
// human PRs, and whether both rates are meaningful.
func (r *Report) Gap() (points float64, ok bool) {
	if r.AllAgents.MergeRate < 0 || r.Humans.MergeRate < 0 {
		return 0, false
	}
	return (r.AllAgents.MergeRate - r.Humans.MergeRate) * 100, true
}
