// Package render prints a report as colored text, Markdown, JSON, an SVG
// badge or an SVG report card.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/celaldrg/agentprs/internal/report"
)

type palette struct{ on bool }

func (p palette) rgb(r, g, b int, s string) string {
	if !p.on {
		return s
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[39m", r, g, b, s)
}
func (p palette) bold(s string) string {
	if !p.on {
		return s
	}
	return "\x1b[1m" + s + "\x1b[22m"
}
func (p palette) dim(s string) string {
	if !p.on {
		return s
	}
	return "\x1b[2m" + s + "\x1b[22m"
}
func (p palette) accent(s string) string { return p.rgb(167, 139, 250, s) }
func (p palette) good(s string) string   { return p.rgb(63, 185, 80, s) }
func (p palette) bad(s string) string    { return p.rgb(248, 81, 73, s) }

// Pct formats a 0..1 ratio as a percentage; negative means "not enough data".
func Pct(x float64) string {
	if x < 0 {
		return "–"
	}
	v := x * 100
	if v > 0 && v < 0.1 {
		return "<0.1%"
	}
	if v >= 10 || v == math.Trunc(v) {
		return fmt.Sprintf("%.0f%%", v)
	}
	return fmt.Sprintf("%.1f%%", v)
}

// Dur formats hours as a short human duration; negative means none.
func Dur(h float64) string {
	switch {
	case h < 0:
		return "–"
	case h < 1:
		return fmt.Sprintf("%.0fm", math.Max(1, h*60))
	case h < 48:
		return fmt.Sprintf("%.1fh", h)
	default:
		return fmt.Sprintf("%.1fd", h/24)
	}
}

func thousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func pad(s string, w int, right bool) string {
	n := utf8.RuneCountInString(s)
	if n >= w {
		return s
	}
	if right {
		return strings.Repeat(" ", w-n) + s
	}
	return s + strings.Repeat(" ", w-n)
}

var spark = []rune("▁▂▃▄▅▆▇█")

// Sparkline renders values in 0..1 relative to their max.
func Sparkline(vals []float64) string {
	max := 0.0
	for _, v := range vals {
		max = math.Max(max, v)
	}
	var b strings.Builder
	for _, v := range vals {
		i := 0
		if max > 0 {
			i = int(math.Round(v / max * float64(len(spark)-1)))
		}
		b.WriteRune(spark[i])
	}
	return b.String()
}

// Headline is the one-sentence summary used by every format.
func Headline(r *report.Report) string {
	if r.Total == 0 {
		return "No pull requests in this window."
	}
	if r.AllAgents.PRs == 0 {
		return fmt.Sprintf("No AI agent signed any of the last %s PRs.", thousands(r.Total))
	}
	s := fmt.Sprintf("AI agents opened %s of the last %s PRs (%s).", thousands(r.AllAgents.PRs), thousands(r.Total), Pct(r.AllAgents.Share))
	if gap, ok := r.Gap(); ok {
		s += " " + Verdict(gap, r)
	}
	return s
}

// Verdict compares agent and human merge rates in words.
func Verdict(gap float64, r *report.Report) string {
	a, h := Pct(r.AllAgents.MergeRate), Pct(r.Humans.MergeRate)
	switch {
	case math.Abs(gap) < 3:
		return fmt.Sprintf("Agent PRs get merged about as often as human PRs (%s vs %s).", a, h)
	case gap < 0:
		return fmt.Sprintf("Agent PRs get merged less often: %s vs %s for human PRs.", a, h)
	default:
		return fmt.Sprintf("Agent PRs get merged more often: %s vs %s for human PRs.", a, h)
	}
}

// Text writes the terminal report.
func Text(w io.Writer, r *report.Report, color bool) {
	p := palette{on: color}
	fmt.Fprintf(w, "%s %s %s\n", p.bold(p.accent("agentprs")), p.dim("·"), p.bold(r.Repo))
	if r.Total == 0 {
		fmt.Fprintln(w, "\n"+Headline(r))
		return
	}
	fmt.Fprintln(w, p.dim(fmt.Sprintf("last %s PRs · opened %s → %s", thousands(r.Total), r.From, r.To)))
	fmt.Fprintln(w)

	// headline bar
	barW := 40
	filled := int(math.Round(r.AllAgents.Share * float64(barW)))
	if r.AllAgents.PRs > 0 && filled == 0 {
		filled = 1
	}
	fmt.Fprintf(w, "  %s  %s%s  %s\n", p.bold(pad(Pct(r.AllAgents.Share), 5, true)),
		p.accent(strings.Repeat("█", filled)), p.dim(strings.Repeat("░", barW-filled)), "of PRs opened by AI agents")
	fmt.Fprintln(w)

	head := fmt.Sprintf("  %s %s %s %s %s %s %s %s", pad("", 16, false), pad("PRs", 6, true), pad("share", 6, true),
		pad("merged", 7, true), pad("closed", 7, true), pad("open", 5, true), pad("merge rate", 11, true), pad("time to merge", 14, true))
	fmt.Fprintln(w, p.dim(head))
	row := func(g *report.Group, name string, strong bool) {
		rate := pad(Pct(g.MergeRate), 11, true)
		line := fmt.Sprintf("  %s %s %s %s %s %s %s %s", pad(name, 16, false), pad(thousands(g.PRs), 6, true), pad(Pct(g.Share), 6, true),
			pad(fmt.Sprint(g.Merged), 7, true), pad(fmt.Sprint(g.Closed), 7, true), pad(fmt.Sprint(g.Open), 5, true), rate, pad(Dur(g.MedianMergeH), 14, true))
		if strong {
			line = p.bold(line)
		}
		fmt.Fprintln(w, line)
	}
	for _, g := range r.Agents {
		row(g, "  "+g.Name, false)
	}
	if len(r.Agents) > 0 {
		fmt.Fprintln(w, p.dim("  "+strings.Repeat("─", 79)))
	}
	row(r.AllAgents, "All agents", true)
	row(r.Humans, "Humans", true)
	if r.DepsBots.PRs > 0 {
		row(r.DepsBots, "Dependency bots", false)
	}
	if r.OtherBots.PRs > 0 {
		row(r.OtherBots, "Other bots", false)
	}
	fmt.Fprintln(w)

	if len(r.Monthly) > 1 {
		vals := make([]float64, len(r.Monthly))
		for i, m := range r.Monthly {
			vals[i] = m.Share
		}
		first, last := r.Monthly[0], r.Monthly[len(r.Monthly)-1]
		fmt.Fprintf(w, "  %s  %s %s %s  %s\n", p.dim("agent share by month"), p.dim(first.Month), p.accent(Sparkline(vals)), p.dim(last.Month),
			p.dim(fmt.Sprintf("(%s → %s)", Pct(first.Share), Pct(last.Share))))
	}
	if gap, ok := r.Gap(); ok {
		msg := Verdict(gap, r)
		switch {
		case gap <= -3:
			msg = p.bad("▼ ") + msg
		case gap >= 3:
			msg = p.good("▲ ") + msg
		default:
			msg = p.dim("= ") + msg
		}
		fmt.Fprintf(w, "  %s\n", msg)
	} else if r.AllAgents.PRs == 0 {
		fmt.Fprintf(w, "  %s\n", Headline(r))
	} else {
		fmt.Fprintf(w, "  %s\n", p.dim(fmt.Sprintf("Too few finished agent PRs to compare merge rates (need %d).", report.MinDecided)))
	}
	fmt.Fprintln(w, p.dim("  merge rate = merged / (merged + closed unmerged); open PRs are left out"))
}

// Markdown writes a summary for READMEs, PR comments and job summaries.
func Markdown(w io.Writer, r *report.Report) {
	fmt.Fprintf(w, "### agentprs: %s\n\n%s\n\n", r.Repo, Headline(r))
	if r.Total == 0 {
		return
	}
	fmt.Fprintf(w, "Last %s PRs, opened %s to %s.\n\n", thousands(r.Total), r.From, r.To)
	fmt.Fprintln(w, "| Opened by | PRs | Share | Merged | Closed | Open | Merge rate | Median time to merge |")
	fmt.Fprintln(w, "|---|---:|---:|---:|---:|---:|---:|---:|")
	row := func(g *report.Group, name string) {
		fmt.Fprintf(w, "| %s | %s | %s | %d | %d | %d | %s | %s |\n", name, thousands(g.PRs), Pct(g.Share), g.Merged, g.Closed, g.Open, Pct(g.MergeRate), Dur(g.MedianMergeH))
	}
	for _, g := range r.Agents {
		row(g, g.Name)
	}
	row(r.AllAgents, "**All agents**")
	row(r.Humans, "**Humans**")
	if r.DepsBots.PRs > 0 {
		row(r.DepsBots, "Dependency bots")
	}
	if r.OtherBots.PRs > 0 {
		row(r.OtherBots, "Other bots")
	}
	fmt.Fprintln(w, "\n<sub>Generated by [agentprs](https://github.com/celaldrg/agentprs). Merge rate = merged / (merged + closed unmerged).</sub>")
}

// JSON writes the report as indented JSON.
func JSON(w io.Writer, r *report.Report) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(r)
}
