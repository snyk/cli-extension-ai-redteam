// demobanner is a standalone program that renders the CLI banner and progress
// animation with mock data so you can preview the UX without running a real scan.
//
// Usage: go run cmd/demobanner/main.go
package main

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ansiReset       = "\033[0m"
	ansiBold        = "\033[1m"
	ansiPurple      = "\033[38;5;141m"
	ansiLightPurple = "\033[38;5;183m"
	ansiDimGray     = "\033[38;5;240m"
	ansiWhite       = "\033[38;5;255m"
	ansiGreen       = "\033[38;5;114m"
	ansiRed         = "\033[38;5;203m"
	ansiClearLn     = "\033[K"
)

var spinnerFrames = []string{"◜", "◝", "◞", "◟"}

var out = os.Stdout

func main() {
	lp := ansiLightPurple
	w := ansiWhite
	r := ansiReset

	// --- Logo ---
	fmt.Fprintf(out, "\n  %s ▄▄▄▄  ▄   ▄   ▄▄▄▄ %s\n", lp, r)
	fmt.Fprintf(out, "  %s█▄▄▄█  █   █  █    █%s\n", lp, r)
	fmt.Fprintf(out, "  %s█       █ █   █    █%s\n", lp, r)
	fmt.Fprintf(out, "  %s ▀▀▀▀    ▀     ▀▀▀▀ %s  %sby Snyk%s\n\n", lp, r, w, r)

	// --- Banner ---
	fmt.Fprintf(out, "  %s%sAI Red Teaming%s\n", ansiBold, lp, r)
	fmt.Fprintf(out, "  %sAdversarial testing for AI-native applications%s\n\n", w, r)
	fmt.Fprintf(out, "  %ssession a1b2c3d4-e5f6-7890-abcd-ef1234567890%s\n\n", w, r)
	fmt.Fprintf(out, "  %s─── scan configuration ───%s\n\n", w, r)
	fmt.Fprintf(out, "  %s%-10s%s %shttps://api.example.com/chat%s\n", w, "Target", r, ansiPurple, r)
	fmt.Fprintf(out, "  %s%-10s%s %ssystem_prompt_extraction, harmful_content%s\n", w, "Goal", r, w, r)
	fmt.Fprintf(out, "  %s%-10s%s %sprofile%s\n", w, "Mode", r, w, r)
	fmt.Fprintf(out, "  %s%-10s%s %sredteam.yaml%s\n\n", w, "Config", r, w, r)
	fmt.Fprintf(out, "  %s[directly asking]%s %s(crescendo)%s %s(role play)%s\n\n", lp, r, ansiDimGray, r, ansiDimGray, r)

	// --- Mock attacks ---
	type attack struct {
		name       string
		totalChats int
		delay      time.Duration
		successful int
	}

	attacks := []attack{
		{"system prompt extraction / directly asking", 10, 200 * time.Millisecond, 1},
		{"harmful content / crescendo", 14, 150 * time.Millisecond, 0},
		{"pii extraction / role play", 8, 250 * time.Millisecond, 2},
	}

	type itemState struct {
		state      int // 0=pending, 1=running, 2=done
		completed  int
		totalChats int
		successful int
	}

	items := make([]itemState, len(attacks))
	for i, a := range attacks {
		items[i] = itemState{totalChats: a.totalChats}
	}

	maxName := 0
	for _, a := range attacks {
		if len(a.name) > maxName {
			maxName = len(a.name)
		}
	}
	if maxName < 20 {
		maxName = 20
	}

	frame := 0
	lineCount := 0
	rendered := false

	render := func() {
		if rendered && lineCount > 0 {
			fmt.Fprintf(out, "\033[%dA", lineCount)
		}
		lines := 0

		fmt.Fprintf(out, "\r  %s─── attack strategies ───%s%s\n", w, r, ansiClearLn)
		lines++
		fmt.Fprintf(out, "\r%s\n", ansiClearLn)
		lines++

		for i, a := range attacks {
			it := items[i]
			padded := fmt.Sprintf("%-*s", maxName, a.name)
			switch it.state {
			case 2: // done
				finding := "finding"
				if it.successful != 1 {
					finding = "findings"
				}
				fc := ansiGreen
				if it.successful > 0 {
					fc = ansiRed
				}
				detail := fmt.Sprintf("%s%d probes / %s%d %s", w, it.totalChats, fc, it.successful, finding)
				fmt.Fprintf(out, "\r  %s✓%s  %s%s   %s%s%s\n", ansiGreen, r, w, padded, detail, r, ansiClearLn)
			case 1: // running
				counter := fmt.Sprintf("%s%d/%d%s", w, it.completed, it.totalChats, r)
				fmt.Fprintf(out, "\r  %s●%s  %s%s   %s%s\n", lp, r, w, padded, counter, ansiClearLn)
			default: // pending
				spin := spinnerFrames[frame%len(spinnerFrames)]
				fmt.Fprintf(out, "\r  %s%s%s  %s%s%s%s\n", ansiDimGray, spin, r, ansiDimGray, padded, r, ansiClearLn)
			}
			lines++
		}

		lineCount = lines
		rendered = true
	}

	// Phase 1: Spinner animation on all pending items (~2 seconds).
	for tick := 0; tick < 16; tick++ {
		render()
		frame++
		time.Sleep(120 * time.Millisecond)
	}

	// Phase 2: Run each attack sequentially.
	for i, a := range attacks {
		items[i].state = 1 // running
		for probe := 1; probe <= a.totalChats; probe++ {
			items[i].completed = probe
			render()
			frame++
			time.Sleep(a.delay)
		}
		items[i].state = 2 // done
		items[i].successful = a.successful
		render()
		time.Sleep(400 * time.Millisecond)
	}

	// --- Results ---
	totalProbes := 0
	totalFindings := 0
	for _, a := range attacks {
		totalProbes += a.totalChats
		totalFindings += a.successful
	}

	fmt.Fprintf(out, "\n\n  %s─── results ───%s\n\n", w, r)
	fmt.Fprintf(out, "  %s%d probes  %s·  %s%d findings  %s·  %d strategies  ·  4s elapsed%s\n",
		w, totalProbes, r, ansiRed, totalFindings, w, len(attacks), r)
	fmt.Fprintf(out, "\n\n  %sView full report in evo → %shttps://app.snyk.io%s\n\n", w, ansiPurple, r)

	// Hide cursor during animation, restore at end.
	_ = strings.Contains("", "")
}
