package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSince(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	for in, want := range map[string]time.Time{
		"":           {},
		"90d":        now.AddDate(0, 0, -90),
		"2w":         now.AddDate(0, 0, -14),
		"6m":         now.AddDate(0, -6, 0),
		"1y":         now.AddDate(-1, 0, 0),
		"2026-01-01": time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		got, err := parseSince(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("parseSince(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := parseSince("yesterday", now); err == nil {
		t.Error("expected error")
	}
}

func TestRunWithFixture(t *testing.T) {
	var out, errb bytes.Buffer
	dir := t.TempDir()
	badge, card := filepath.Join(dir, "b.svg"), filepath.Join(dir, "c.svg")
	code := run([]string{"--input", "testdata/sample.json", "--no-color", "--badge", badge, "--card", card}, &out, &errb)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"Copilot", "Codex", "Claude Code", "Humans", "Dependency bots"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q\n%s", want, s)
		}
	}
	for _, f := range []string{badge, card} {
		if b, err := os.ReadFile(f); err != nil || !bytes.HasPrefix(b, []byte("<svg")) {
			t.Errorf("%s not written: %v", f, err)
		}
	}
}

func TestRunJSONAndMarkdown(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--input", "testdata/sample.json", "--json"}, &out, &errb); code != 0 || !strings.Contains(out.String(), `"total_prs"`) {
		t.Fatalf("json: %d %s %s", code, out.String(), errb.String())
	}
	out.Reset()
	if code := run([]string{"--input", "testdata/sample.json", "--markdown"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "| **All agents** |") {
		t.Fatalf("markdown: %d %s", code, out.String())
	}
}

func TestRunErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--limit", "0", "o/r"}, &out, &errb); code != 2 {
		t.Errorf("limit 0: exit %d", code)
	}
	if code := run([]string{"not a repo!"}, &out, &errb); code != 2 {
		t.Errorf("bad repo: exit %d", code)
	}
	if code := run([]string{"a/b", "c/d"}, &out, &errb); code != 2 {
		t.Errorf("two repos: exit %d", code)
	}
	if code := run([]string{"--version"}, &out, &errb); code != 0 || !strings.Contains(out.String(), version) {
		t.Errorf("version: exit %d", code)
	}
	out.Reset()
	if code := run([]string{"--list-agents"}, &out, &errb); code != 0 || !strings.Contains(out.String(), "Jules") {
		t.Errorf("list-agents: exit %d", code)
	}
}
