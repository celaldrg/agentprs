package render

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/celaldrg/agentprs/internal/gh"
	"github.com/celaldrg/agentprs/internal/report"
)

func sample() *report.Report {
	t0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	var prs []gh.PR
	add := func(login, typ, body string, merged bool) {
		p := gh.PR{Body: body, CreatedAt: t0, State: "open"}
		p.User.Login, p.User.Type = login, typ
		if merged {
			m := t0.Add(5 * time.Hour)
			p.MergedAt, p.ClosedAt, p.State = &m, &m, "closed"
		} else {
			c := t0.Add(time.Hour)
			p.ClosedAt, p.State = &c, "closed"
		}
		prs = append(prs, p)
		t0 = t0.Add(24 * time.Hour)
	}
	for i := 0; i < 8; i++ {
		add("Copilot", "Bot", "", i%2 == 0)
		add("dev", "User", "", true)
	}
	add("dev", "User", "<script>&", false)
	return report.Build("o/<r>", prs, t0)
}

func TestPctDur(t *testing.T) {
	for in, want := range map[float64]string{-1: "–", 0: "0%", 0.0004: "<0.1%", 0.055: "5.5%", 0.5: "50%", 1: "100%"} {
		if got := Pct(in); got != want {
			t.Errorf("Pct(%v)=%q want %q", in, got, want)
		}
	}
	for in, want := range map[float64]string{-1: "–", 0.25: "15m", 5: "5.0h", 72: "3.0d"} {
		if got := Dur(in); got != want {
			t.Errorf("Dur(%v)=%q want %q", in, got, want)
		}
	}
}

func TestTextNoColor(t *testing.T) {
	var b bytes.Buffer
	Text(&b, sample(), false)
	out := b.String()
	if strings.Contains(out, "\x1b[") {
		t.Fatal("escape codes with color off")
	}
	for _, want := range []string{"agentprs", "Copilot", "All agents", "Humans", "47%", "50%", "merge rate"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestHeadline(t *testing.T) {
	h := Headline(sample())
	if !strings.Contains(h, "AI agents opened 8 of the last 17 PRs (47%)") || !strings.Contains(h, "Agent PRs get merged less often: 50% vs 89% for human PRs.") {
		t.Fatal(h)
	}
	if Headline(report.Build("o/r", nil, time.Now())) != "No pull requests in this window." {
		t.Fatal("empty headline")
	}
}

func TestSVGsAreValidXML(t *testing.T) {
	r := sample()
	for name, svg := range map[string]string{"badge": Badge("agent PRs", BadgeValue(r)), "card": Card(r)} {
		d := xml.NewDecoder(strings.NewReader(svg))
		for {
			_, err := d.Token()
			if err != nil {
				if err.Error() != "EOF" {
					t.Fatalf("%s: %v\n%s", name, err, svg)
				}
				break
			}
		}
	}
	if BadgeValue(r) != "47% · 50% merged" {
		t.Fatal(BadgeValue(r))
	}
}

func TestMarkdownAndJSON(t *testing.T) {
	var b bytes.Buffer
	Markdown(&b, sample())
	if !strings.Contains(b.String(), "| Copilot | 8 | 47% | 4 | 4 | 0 | 50% | 5.0h |") {
		t.Fatal(b.String())
	}
	b.Reset()
	if err := JSON(&b, sample()); err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(b.Bytes(), &v); err != nil || v["total_prs"].(float64) != 17 {
		t.Fatal(err, v["total_prs"])
	}
}

func TestSparkline(t *testing.T) {
	if s := Sparkline([]float64{0, 0.5, 1}); s != "▁▅█" {
		t.Fatal(s)
	}
}
