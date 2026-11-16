package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

type overlay struct {
	row, col, cols, rows int
	data                 string
}

type spot struct {
	line, x, cols, rows int
	chat, id            string
	data                string
}

type hit struct {
	row, col, cols, rows int
	chat, id             string
}

type frame struct {
	hits      []hit
	lastWheel time.Time
	fragAt    time.Time
	escAt     time.Time
}

var mouseFragment = regexp.MustCompile(`^\[?<?[0-9;]*[Mm]?$`)

func (f *frame) leak(runes string, esc bool) bool {
	now := time.Now()
	if esc {
		f.fragAt = now
		return false
	}
	if runes == "" {
		return false
	}
	if strings.HasPrefix(runes, "[<") || (now.Sub(f.fragAt) < 150*time.Millisecond && mouseFragment.MatchString(runes)) {
		f.fragAt = now
		f.escAt = time.Time{}
		return true
	}
	return false
}

var itermImages = detectIterm()

func detectIterm() bool {
	switch os.Getenv("WA_NEO_IMAGES") {
	case "iterm":
		return true
	case "blocks":
		return false
	}
	return os.Getenv("TERM_PROGRAM") == "iTerm.app" || strings.HasPrefix(os.Getenv("LC_TERMINAL"), "iTerm")
}

func encodeImage(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func applyOverlays(screen string, overlays []overlay) string {
	if len(overlays) == 0 {
		return screen
	}
	sort.SliceStable(overlays, func(i, j int) bool { return overlays[i].col < overlays[j].col })

	lines := strings.Split(screen, "\n")
	for y, original := range lines {
		var out strings.Builder
		pos, touched := 0, false
		for _, o := range overlays {
			if y < o.row || y >= o.row+o.rows {
				continue
			}
			touched = true
			out.WriteString(ansi.Cut(original, pos, o.col))
			if y == o.row {
				fmt.Fprintf(&out, "\x1b7\x1b]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=0:%s\x07\x1b8", o.cols, o.rows, o.data)
			}
			fmt.Fprintf(&out, "\x1b[%dC", o.cols)
			pos = o.col + o.cols
		}
		if touched {
			out.WriteString(ansi.Cut(original, pos, 1<<16))
			lines[y] = out.String()
		}
	}
	return strings.Join(lines, "\n")
}
