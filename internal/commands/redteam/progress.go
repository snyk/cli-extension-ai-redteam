package redteam

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/snyk/cli-extension-ai-redteam/internal/services/controlserver"
)

// Attack states for the checklist UI.
const (
	statePending = iota
	stateRunning
	stateDone
)

// checklistItem tracks a single attack entry in the progress checklist.
type checklistItem struct {
	key        string // AttackType from server
	name       string // display name
	state      int
	successful int // findings count (for coloring)
	completed  int // probes completed so far
	totalChats int // total probes planned
	reported   int // last state printed in plain mode
}

const strategyBarWidth = 12

// progressUI renders a checklist of attack strategies that updates in-place.
//
// Color mode uses ANSI cursor movement to redraw the list:
//
//	✓ directly_asking     ████████████  1 finding / 10 probes
//	■ role_play           ██████░░░░░░  4/7
//	□ crescendo
//
// Plain mode prints state changes as append-only lines.
type progressUI struct {
	mu        sync.Mutex
	w         io.Writer
	color     bool
	startTime time.Time
	items     []checklistItem
	lineCount int  // lines rendered (for cursor-up rewrite)
	rendered  bool // true after the first render
}

// newProgressUI creates a progress UI that writes to w.
func newProgressUI(w io.Writer, color bool) *progressUI {
	return &progressUI{
		w:         w,
		color:     color,
		startTime: time.Now(),
	}
}

// Start is called before the scan loop begins.
func (p *progressUI) Start() {}

// Stop is called when the scan loop ends.
func (p *progressUI) Stop() {}

// Update processes a ScanStatus snapshot, updating the checklist state and
// redrawing it in-place (color mode) or printing changes (plain mode).
func (p *progressUI) Update(status *controlserver.ScanStatus) {
	if status == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, atk := range status.Attacks {
		idx := p.findOrAdd(atk.AttackType)
		item := &p.items[idx]

		item.completed = atk.Completed
		item.totalChats = atk.TotalChats

		if atk.TotalChats > 0 && atk.Completed >= atk.TotalChats {
			item.state = stateDone
			item.successful = atk.Successful
		} else if item.state < stateRunning {
			item.state = stateRunning
		}
	}

	if p.color {
		p.renderColor()
	} else {
		p.renderPlain()
	}
}

// Finish marks the scan as complete and prints a results summary.
func (p *progressUI) Finish(status *controlserver.ScanStatus) {
	if status != nil {
		p.Update(status)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Since(p.startTime).Round(time.Second)
	findings := 0
	strategies := 0
	totalProbes := 0
	if status != nil {
		findings = status.Successful
		strategies = len(status.Attacks)
		totalProbes = status.TotalChats
	}

	if p.color {
		fmt.Fprintf(p.w, "\n\n  %s\u2500\u2500\u2500 results \u2500\u2500\u2500%s\n\n", ansiWhite, ansiReset)
		fmt.Fprintf(p.w, "  %s%d findings  %s\u00B7  %d probes  \u00B7  %d strategies  \u00B7  %s elapsed%s\n",
			ansiRed, findings, ansiWhite, totalProbes, strategies, elapsed, ansiReset)
		fmt.Fprintf(p.w, "\n\n  %sView full report in evo \u2192 %shttps://app.snyk.io%s\n\n",
			ansiWhite, ansiPurple, ansiReset)
	} else {
		fmt.Fprintf(p.w, "done — %d findings, %d probes, %d strategies, %s elapsed\n", findings, totalProbes, strategies, elapsed)
		fmt.Fprintf(p.w, "View full report in evo -> https://app.snyk.io\n")
	}
}

// findOrAdd returns the index of the item with the given key, creating it if needed.
func (p *progressUI) findOrAdd(attackType string) int {
	for i, item := range p.items {
		if item.key == attackType {
			return i
		}
	}
	p.items = append(p.items, checklistItem{
		key:   attackType,
		name:  attackDisplayName(attackType),
		state: statePending,
	})
	return len(p.items) - 1
}

// renderColor redraws the entire checklist in-place using ANSI cursor movement.
func (p *progressUI) renderColor() {
	// Move cursor up to overwrite the previous render.
	if p.rendered && p.lineCount > 0 {
		fmt.Fprintf(p.w, "\033[%dA", p.lineCount)
	}

	lines := 0

	// Header + blank line margin.
	fmt.Fprintf(p.w, "\r  %s\u2500\u2500\u2500 attack strategies \u2500\u2500\u2500%s%s\n", ansiWhite, ansiReset, ansiClearLn)
	lines++
	fmt.Fprintf(p.w, "\r%s\n", ansiClearLn)
	lines++

	// Checklist items.
	for _, item := range p.items {
		padded := fmt.Sprintf("%-20s", item.name)
		switch item.state {
		case stateDone:
			bar := renderBar(item.completed, item.totalChats, ansiGreen, true)
			finding := "finding"
			if item.successful != 1 {
				finding = "findings"
			}
			findingsColor := ansiGreen
			if item.successful > 0 {
				findingsColor = ansiRed
			}
			detail := fmt.Sprintf("%s%d %s%s / %d probes",
				findingsColor, item.successful, finding, ansiGreen, item.totalChats)
			fmt.Fprintf(p.w, "\r  %s\u2713%s %s%s  %s%s%s\n",
				ansiGreen, ansiReset,
				ansiWhite, padded,
				bar, detail, ansiClearLn)
		case stateRunning:
			bar := renderBar(item.completed, item.totalChats, ansiPurple, false)
			counter := fmt.Sprintf("%s%d/%d%s", ansiDimGray, item.completed, item.totalChats, ansiReset)
			fmt.Fprintf(p.w, "\r  %s\u25A0%s %s%s  %s%s%s\n",
				ansiWhite, ansiReset,
				ansiWhite, padded,
				bar, counter, ansiClearLn)
		default:
			fmt.Fprintf(p.w, "\r  %s\u25A1 %s%s%s\n",
				ansiDimGray, padded, ansiReset,
				ansiClearLn)
		}
		lines++
	}

	p.lineCount = lines
	p.rendered = true
}

// renderBar builds a progress bar string: [████████░░░░]
func renderBar(completed, total int, fillColor string, full bool) string {
	filled := 0
	if full || total <= 0 {
		filled = strategyBarWidth
	} else {
		filled = completed * strategyBarWidth / total
	}
	empty := strategyBarWidth - filled

	return fmt.Sprintf("%s%s%s%s%s ",
		fillColor, strings.Repeat("\u2588", filled), ansiReset,
		ansiVeryDark, strings.Repeat("\u2591", empty))
}

// renderPlain prints state changes as append-only lines (non-TTY fallback).
func (p *progressUI) renderPlain() {
	for i := range p.items {
		item := &p.items[i]
		if item.state > item.reported {
			switch item.state {
			case stateRunning:
				fmt.Fprintf(p.w, "  running %s (%d/%d)...\n", item.name, item.completed, item.totalChats)
			case stateDone:
				finding := "finding"
				if item.successful != 1 {
					finding = "findings"
				}
				fmt.Fprintf(p.w, "  done    %-20s %d %s / %d probes\n", item.name, item.successful, finding, item.totalChats)
			}
			item.reported = item.state
		}
	}
}

// attackDisplayName extracts a human-friendly name from an AttackType string.
// AttackType format is typically "goal/strategy/index"; returns the strategy
// component if present, otherwise the goal.
func attackDisplayName(attackType string) string {
	parts := strings.SplitN(attackType, "/", 3)
	if len(parts) >= 2 && parts[1] != "" {
		return parts[1]
	}
	if len(parts) >= 1 {
		return parts[0]
	}
	return attackType
}
