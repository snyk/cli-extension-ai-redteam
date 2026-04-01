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

// progressUI renders a checklist of attack strategies that updates in-place.
//
// Color mode uses ANSI cursor movement to redraw the list:
//
//	✓ directly_asking       1 finding / 10 probes
//	● role_play             4/7
//	○ crescendo
//
// Plain mode prints state changes as append-only lines.
// Spinner frames for pending items (rotating circle animation).
var spinnerFrames = []string{"◜", "◝", "◞", "◟"}

type progressUI struct {
	mu        sync.Mutex
	w         io.Writer
	color     bool
	startTime time.Time
	items     []checklistItem
	lineCount int  // lines rendered (for cursor-up rewrite)
	rendered  bool // true after the first render
	frame     int  // current spinner animation frame

	// Client-side sent counter: incremented after each SendPrompt so
	// counters update smoothly between server-side status updates.
	sent       int
	totalChats int

	ticker *time.Ticker
	stopCh chan struct{}
}

// newProgressUI creates a progress UI that writes to w.
func newProgressUI(w io.Writer, color bool) *progressUI {
	return &progressUI{
		w:         w,
		color:     color,
		startTime: time.Now(),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the spinner animation ticker.
func (p *progressUI) Start() {
	if !p.color {
		return
	}
	p.ticker = time.NewTicker(120 * time.Millisecond)
	go func() {
		for {
			select {
			case <-p.stopCh:
				return
			case <-p.ticker.C:
				p.mu.Lock()
				p.frame++
				if p.rendered {
					p.renderColor()
				}
				p.mu.Unlock()
			}
		}
	}()
}

// Stop halts the spinner animation.
func (p *progressUI) Stop() {
	if p.ticker != nil {
		p.ticker.Stop()
	}
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
}

// IncrementSent records that one more prompt was sent to the target.
// Distributes progress proportionally across running strategies so
// counters update smoothly between server-side status updates.
func (p *progressUI) IncrementSent() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sent++
	p.distributeClientProgress()
	if p.color && p.rendered {
		p.renderColor()
	}
}

// distributeClientProgress spreads client-side sent count across running
// items proportionally based on their totalChats. Caller must hold p.mu.
func (p *progressUI) distributeClientProgress() {
	if p.totalChats == 0 || p.sent == 0 {
		return
	}
	for i := range p.items {
		item := &p.items[i]
		if item.state != stateRunning || item.totalChats == 0 {
			continue
		}
		estimated := p.sent * item.totalChats / p.totalChats
		if estimated > item.totalChats {
			estimated = item.totalChats
		}
		if estimated > item.completed {
			item.completed = estimated
		}
	}
}

// Update processes a ScanStatus snapshot, updating the checklist state and
// redrawing it in-place (color mode) or printing changes (plain mode).
func (p *progressUI) Update(status *controlserver.ScanStatus) {
	if status == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.totalChats = status.TotalChats

	for _, atk := range status.Attacks {
		idx := p.findOrAdd(atk.AttackType)
		item := &p.items[idx]

		item.completed = atk.Completed
		item.totalChats = atk.TotalChats

		if atk.TotalChats > 0 && atk.Completed >= atk.TotalChats {
			item.state = stateDone
			item.successful = atk.Successful
		} else if atk.Completed > 0 && item.state < stateRunning {
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
		fmt.Fprintf(p.w, "  %s%d probes  %s\u00B7  %s%d findings  %s\u00B7  %d strategies  \u00B7  %s elapsed%s\n",
			ansiWhite, totalProbes, ansiReset, ansiRed, findings, ansiWhite, strategies, elapsed, ansiReset)
		fmt.Fprintf(p.w, "\n\n  %sView full report in evo \u2192 %shttps://app.snyk.io%s\n\n",
			ansiWhite, ansiPurple, ansiReset)
	} else {
		fmt.Fprintf(p.w, "done — %d probes, %d findings, %d strategies, %s elapsed\n", totalProbes, findings, strategies, elapsed)
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
	if p.rendered && p.lineCount > 0 {
		fmt.Fprintf(p.w, "\033[%dA", p.lineCount)
	}

	lines := 0

	// Header.
	fmt.Fprintf(p.w, "\r  %s\u2500\u2500\u2500 attack strategies \u2500\u2500\u2500%s%s\n", ansiWhite, ansiReset, ansiClearLn)
	lines++
	fmt.Fprintf(p.w, "\r%s\n", ansiClearLn)
	lines++

	// Determine max name width for alignment.
	maxName := 0
	for _, item := range p.items {
		if len(item.name) > maxName {
			maxName = len(item.name)
		}
	}
	if maxName < 20 {
		maxName = 20
	}

	// Attack list.
	for _, item := range p.items {
		padded := fmt.Sprintf("%-*s", maxName, item.name)
		switch item.state {
		case stateDone:
			finding := "finding"
			if item.successful != 1 {
				finding = "findings"
			}
			findingsColor := ansiGreen
			if item.successful > 0 {
				findingsColor = ansiRed
			}
			detail := fmt.Sprintf("%s%d probes / %s%d %s",
				ansiWhite, item.totalChats, findingsColor, item.successful, finding)
			fmt.Fprintf(p.w, "\r  %s\u2713%s  %s%s   %s%s%s\n",
				ansiGreen, ansiReset,
				ansiWhite, padded,
				detail, ansiReset, ansiClearLn)
		case stateRunning:
			counter := fmt.Sprintf("%s%d/%d%s", ansiWhite, item.completed, item.totalChats, ansiReset)
			fmt.Fprintf(p.w, "\r  %s\u25CF%s  %s%s   %s%s\n",
				ansiLightPurple, ansiReset,
				ansiWhite, padded,
				counter, ansiClearLn)
		default:
			spin := spinnerFrames[p.frame%len(spinnerFrames)]
			fmt.Fprintf(p.w, "\r  %s%s%s  %s%s%s%s\n",
				ansiDimGray, spin, ansiReset,
				ansiDimGray, padded,
				ansiReset, ansiClearLn)
		}
		lines++
	}

	p.lineCount = lines
	p.rendered = true
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
// AttackType format is typically "goal/strategy/index"; returns "goal / strategy"
// with underscores replaced by spaces for readability.
func attackDisplayName(attackType string) string {
	parts := strings.SplitN(attackType, "/", 3)
	clean := func(s string) string { return strings.ReplaceAll(s, "_", " ") }
	if len(parts) >= 2 && parts[1] != "" {
		return clean(parts[0]) + " / " + clean(parts[1])
	}
	if len(parts) >= 1 {
		return clean(parts[0])
	}
	return clean(attackType)
}
