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
	if !strings.Contains(out, "attack strategies") {
		t.Fatalf("missing header: %q", out)
	}
	if !strings.Contains(out, "system_prompt_extraction/foo") {
		t.Fatalf("missing attack label: %q", out)
	}
	if !strings.Contains(out, "directly_asking") {
		t.Fatalf("missing tag: %q", out)
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
