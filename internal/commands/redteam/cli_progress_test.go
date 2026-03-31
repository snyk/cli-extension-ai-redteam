//nolint:testpackage // Tests internal helpers (fingerprint, renderers, truncate) alongside exported config helpers.
package redteam

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

func TestReportFindingsCount(t *testing.T) {
	raw := `{"id":"abc","results":[{"id":"1"},{"id":"2"}],"passed_types":[]}`
	if n := reportFindingsCount([]byte(raw)); n != 2 {
		t.Fatalf("findings count: got %d want 2", n)
	}
	if n := reportFindingsCount([]byte(`{"results":[]}`)); n != 0 {
		t.Fatalf("empty: got %d", n)
	}
	if n := reportFindingsCount([]byte(`not json`)); n != 0 {
		t.Fatalf("invalid json: got %d", n)
	}
}

func TestStatusFingerprint(t *testing.T) {
	a := controlserver.AttackStatus{
		AttackType: "g/s",
		TotalChats: 3,
		Completed:  1,
		Failed:     0,
	}
	s := &controlserver.ScanStatus{
		Completed:  2,
		TotalChats: 5,
		Attacks:    []controlserver.AttackStatus{a},
	}
	fp := statusFingerprint(s)
	if fp == "" {
		t.Fatal("empty fingerprint")
	}
	if fp2 := statusFingerprint(s); fp != fp2 {
		t.Fatalf("stable fingerprint: %q vs %q", fp, fp2)
	}
	a.Completed = 2
	s.Attacks = []controlserver.AttackStatus{a}
	if fp3 := statusFingerprint(s); fp3 == fp {
		t.Fatalf("expected change when attack progress changes: %q", fp3)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("abc", 10); got != "abc" {
		t.Fatalf("got %q", got)
	}
	if got := truncateRunes("abcdef", 4); got != "abc…" {
		t.Fatalf("got %q", got)
	}
}

func TestHorizontalRule_containsLabel(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	theme := newCLITheme(&buf)
	theme.r.SetColorProfile(termenv.Ascii)
	line := horizontalRule(theme, "scan configuration", 72)
	if !strings.Contains(line, "scan configuration") {
		t.Fatalf("missing label: %q", line)
	}
}

func TestRenderAttackStrategiesSection_ascii(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	theme := newCLITheme(&buf)
	theme.r.SetColorProfile(termenv.Ascii)
	st := &controlserver.ScanStatus{
		Tags: []string{"directly_asking"},
		Attacks: []controlserver.AttackStatus{
			{AttackType: "system_prompt_extraction/foo", TotalChats: 4, Completed: 2, Failed: 0},
		},
	}
	out := renderAttackStrategiesSection(theme, st, 80)
	if !strings.Contains(out, "attacks") {
		t.Fatalf("missing header: %q", out)
	}
	if !strings.Contains(out, "system prompt extraction / foo") {
		t.Fatalf("missing attack label: %q", out)
	}
}

func TestDeriveScanModeLabel(t *testing.T) {
	if deriveScanModeLabel("") != "custom" {
		t.Fatal()
	}
	if deriveScanModeLabel("Fast") != "Fast" {
		t.Fatal()
	}
}

func TestLiveProgress_renderBlock_ascii(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	theme := newCLITheme(&buf)
	theme.r.SetColorProfile(termenv.Ascii)
	lp := &liveProgress{theme: theme, width: 80, isTTY: false}
	st := &controlserver.ScanStatus{
		TotalChats: 10,
		Completed:  4,
		Tags:       []string{"directly_asking"},
		Attacks: []controlserver.AttackStatus{
			{AttackType: "goal_a/start_a", TotalChats: 5, Completed: 3, Failed: 0},
			{AttackType: "goal_b/start_b", TotalChats: 5, Completed: 1, Failed: 1},
		},
	}
	block := lp.renderBlock(st, true)
	if !strings.Contains(block, "attacks") {
		t.Fatalf("missing header: %q", block)
	}
	if !strings.Contains(block, "goal a / start a") {
		t.Fatalf("missing first attack: %q", block)
	}
	if !strings.Contains(block, "goal b / start b") {
		t.Fatalf("missing second attack: %q", block)
	}
	if !strings.Contains(block, "40%") {
		t.Fatalf("missing progress pct: %q", block)
	}

	finalBlock := lp.renderBlock(st, false)
	if strings.Contains(finalBlock, "Scanning") {
		t.Fatalf("final block should not have scanning indicator: %q", finalBlock)
	}
}

func TestLiveProgress_update_skips_duplicate(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	theme := newCLITheme(&buf)
	theme.r.SetColorProfile(termenv.Ascii)
	lp := &liveProgress{theme: theme, width: 80, isTTY: false}
	st := &controlserver.ScanStatus{
		TotalChats: 5,
		Completed:  2,
		Attacks:    []controlserver.AttackStatus{{AttackType: "a/b", TotalChats: 5, Completed: 2}},
	}
	lp.update(st)
	fp1 := lp.lastFP
	lp.update(st)
	if lp.lastFP != fp1 {
		t.Fatal("fingerprint changed on identical status")
	}
}
