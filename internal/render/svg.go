package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/celaldrg/agentprs/internal/report"
)

func esc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// textWidth approximates Verdana 11px width, good enough for badges.
func textWidth(s string) float64 {
	w := 0.0
	for _, r := range s {
		switch {
		case strings.ContainsRune("il.,:;|!'· ", r):
			w += 3.6
		case strings.ContainsRune("mwMW%", r):
			w += 10
		case r >= 'A' && r <= 'Z':
			w += 7.6
		default:
			w += 6.6
		}
	}
	return w
}

// BadgeValue is the right half of the badge, e.g. "12% · 61% merged".
func BadgeValue(r *report.Report) string {
	v := Pct(r.AllAgents.Share)
	if r.AllAgents.MergeRate >= 0 {
		v += " · " + Pct(r.AllAgents.MergeRate) + " merged"
	}
	return v
}

// Badge returns a shields-style SVG badge.
func Badge(label, value string) string {
	lw := math.Round(textWidth(label) + 12)
	vw := math.Round(textWidth(value) + 12)
	w := lw + vw
	color := "#8b5cf6"
	t := esc(label + ": " + value)
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%[1]g" height="20" role="img" aria-label="%[2]s">
<title>%[2]s</title>
<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>
<clipPath id="r"><rect width="%[1]g" height="20" rx="3" fill="#fff"/></clipPath>
<g clip-path="url(#r)"><rect width="%[3]g" height="20" fill="#555"/><rect x="%[3]g" width="%[4]g" height="20" fill="%[5]s"/><rect width="%[1]g" height="20" fill="url(#s)"/></g>
<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11">
<text x="%[6]g" y="15" fill="#010101" fill-opacity=".3">%[7]s</text><text x="%[6]g" y="14">%[7]s</text>
<text x="%[8]g" y="15" fill="#010101" fill-opacity=".3">%[9]s</text><text x="%[8]g" y="14">%[9]s</text>
</g></svg>
`, w, t, lw, vw, color, lw/2, esc(label), lw+vw/2, esc(value))
}

// Card returns a shareable SVG report card.
func Card(r *report.Report) string {
	const W = 600
	rows := r.Agents
	if len(rows) > 6 {
		rows = rows[:6]
	}
	H := 250 + len(rows)*26
	var b strings.Builder
	font := `font-family="-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif"`
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s">`+"\n", W, H, W, H, esc(Headline(r)))
	fmt.Fprintf(&b, `<rect width="%d" height="%d" rx="14" fill="#0d1117" stroke="#30363d"/>`+"\n", W, H)
	fmt.Fprintf(&b, `<g %s>`+"\n", font)
	fmt.Fprintf(&b, `<text x="28" y="40" font-size="13" fill="#a78bfa" font-weight="600">agentprs</text><text x="96" y="40" font-size="13" fill="#7d8590">%s · last %s PRs</text>`+"\n", esc(r.Repo), thousands(r.Total))
	fmt.Fprintf(&b, `<text x="28" y="98" font-size="52" font-weight="700" fill="#e6edf3">%s</text>`+"\n", Pct(r.AllAgents.Share))
	fmt.Fprintf(&b, `<text x="28" y="124" font-size="13" fill="#7d8590">of pull requests were opened by AI agents</text>`+"\n")
	// merge-rate comparison
	cmp := func(x int, label string, rate float64, fill string) {
		fmt.Fprintf(&b, `<text x="%d" y="142" font-size="12" fill="#7d8590">%s</text><text x="%d" y="168" font-size="22" font-weight="600" fill="%s">%s</text>`+"\n", x, label, x, fill, Pct(rate))
	}
	cmp(340, "agent PRs merged", r.AllAgents.MergeRate, "#a78bfa")
	cmp(470, "human PRs merged", r.Humans.MergeRate, "#e6edf3")
	fmt.Fprintf(&b, `<text x="340" y="80" font-size="12" fill="#7d8590">median time to merge</text><text x="340" y="104" font-size="14" fill="#e6edf3">agents %s · humans %s</text>`+"\n", Dur(r.AllAgents.MedianMergeH), Dur(r.Humans.MedianMergeH))
	y := 220
	for _, g := range rows {
		bw := 0.0
		if r.AllAgents.PRs > 0 {
			bw = float64(g.PRs) / float64(r.AllAgents.PRs) * 240
		}
		fmt.Fprintf(&b, `<text x="28" y="%d" font-size="13" fill="#e6edf3">%s</text>`, y, esc(g.Name))
		fmt.Fprintf(&b, `<rect x="140" y="%d" width="%.1f" height="12" rx="3" fill="#8b5cf6"/>`, y-11, math.Max(bw, 2))
		fmt.Fprintf(&b, `<text x="%.1f" y="%d" font-size="12" fill="#7d8590">%s · %s merged</text>`+"\n", 148+math.Max(bw, 2), y, plural(g.PRs, "PR"), Pct(g.MergeRate))
		y += 26
	}
	fmt.Fprintf(&b, `<text x="28" y="%d" font-size="11" fill="#7d8590">%s → %s · github.com/celaldrg/agentprs</text>`+"\n", H-20, r.From, r.To)
	b.WriteString("</g></svg>\n")
	return b.String()
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return thousands(n) + " " + word + "s"
}
