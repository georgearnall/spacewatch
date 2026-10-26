// Package ui renders the pending-confirmations table to the terminal,
// including OSC 8 clickable hyperlinks where the terminal is known to
// support them.
package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/rivo/uniseg"
	"golang.org/x/term"
)

// Row is one already-formatted table row. Selected marks the row as the
// keyboard-navigation cursor's current target, shown as a leading marker.
type Row struct {
	Team     string
	Stack    string
	Title    string
	Plan     *Plan // nil if unknown
	Age      string
	URL      string
	Selected bool
}

// Plan is a run's planned resource changes.
type Plan struct {
	Add, Change, Delete int
}

// String formats p as e.g. "+2 ~3 -1".
func (p Plan) String() string {
	return fmt.Sprintf("+%d ~%d -%d", p.Add, p.Change, p.Delete)
}

// styled formats p like String, coloring each non-zero count: additions
// green, changes amber and deletions red.
func (p Plan) styled() string {
	part := func(sign string, n int, code string) string {
		s := sign + strconv.Itoa(n)
		if n == 0 {
			return s
		}
		return Style(s, code, true)
	}
	return part("+", p.Add, Green) + " " + part("~", p.Change, Yellow) + " " + part("-", p.Delete, Red)
}

func (r Row) planText() string {
	if r.Plan == nil {
		return ""
	}
	return r.Plan.String()
}

const linkText = "open →"

// SGR color/style codes used to highlight the table. Combined codes
// (e.g. BoldYellow) are single escape sequences rather than two Style()
// calls, so nesting them inside an already-styled cell can't produce a
// stray extra Reset in the middle.
const (
	Reset      = "\x1b[0m"
	Bold       = "\x1b[1m"
	Dim        = "\x1b[2m"
	Cyan       = "\x1b[36m"
	Red        = "\x1b[31m"
	Green      = "\x1b[32m"
	Yellow     = "\x1b[33m"
	BoldRed    = "\x1b[1;31m"
	BoldYellow = "\x1b[1;33m"
)

// Style wraps s in the given SGR code if enabled is true and s is
// non-empty; otherwise it returns s unchanged.
func Style(s, code string, enabled bool) string {
	if !enabled || s == "" {
		return s
	}
	return code + s + Reset
}

// Title widths for the table layout: titles are capped at maxTitle cells,
// and may shrink to minTitle to fit a narrow terminal before RenderTable
// gives up on the table layout.
const (
	maxTitle = 60
	minTitle = 20
)

// RenderTable renders rows as an aligned table. When linksSupported is
// true, the LINK column is an OSC 8 hyperlink; otherwise it falls back to
// printing the raw URL so it can still be copied. When colorEnabled is
// true, the header is bold, the TEAM column is cyan, the PLAN column's
// additions are green, changes amber and deletions red, the AGE column is dim, and the
// selected row's marker is highlighted.
//
// cols is the terminal width (0 if unknown). When the table is wider,
// titles are truncated further to fit, and if that isn't enough each row
// is stacked over several lines instead.
func RenderTable(rows []Row, linksSupported, colorEnabled bool, cols int) string {
	if len(rows) == 0 {
		return "no pending confirmations for your team\n"
	}

	titleMax := maxTitle
	if cols > 0 {
		_, widths := tableCells(rows, linksSupported, maxTitle)
		if over := tableWidth(widths) - cols; over > 0 {
			titleMax = widths[3] - over
			if titleMax < minTitle {
				return renderStacked(rows, linksSupported, colorEnabled, cols)
			}
		}
	}
	return renderTable(rows, linksSupported, colorEnabled, titleMax)
}

var headers = []string{"", "TEAM", "STACK", "TITLE", "PLAN", "AGE", "LINK"}

// tableCells lays out rows as table cells, with titles truncated to
// titleMax cells, and returns each column's width in terminal cells. The
// LINK column's width is that of its visible text.
func tableCells(rows []Row, linksSupported bool, titleMax int) ([][]string, []int) {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	cells := make([][]string, len(rows))
	for i, r := range rows {
		link := r.URL
		if linksSupported {
			link = Hyperlink(r.URL, linkText)
		}
		marker := " "
		if r.Selected {
			marker = ">"
		}
		cells[i] = []string{marker, r.Team, r.Stack, truncate(r.Title, titleMax), r.planText(), r.Age, link}
		for j, c := range cells[i] {
			if l := uniseg.StringWidth(stripEscapes(c)); l > widths[j] {
				widths[j] = l
			}
		}
	}
	return cells, widths
}

// tableWidth is the width of a table line with the given column widths.
func tableWidth(widths []int) int {
	w := 2 * (len(widths) - 1)
	for _, cw := range widths {
		w += cw
	}
	return w
}

func renderTable(rows []Row, linksSupported, colorEnabled bool, titleMax int) string {
	cells, widths := tableCells(rows, linksSupported, titleMax)

	var b strings.Builder

	writeHeader := func() {
		for i, h := range headers {
			cell := h
			if i != len(headers)-1 {
				cell = pad(h, widths[i])
			}
			b.WriteString(Style(cell, Bold, colorEnabled))
			if i != len(headers)-1 {
				b.WriteString("  ")
			}
		}
		b.WriteByte('\n')
	}

	writeDataRow := func(row []string, r Row) {
		for i, c := range row {
			if i == len(row)-1 {
				b.WriteString(styleCell(i, c, r, colorEnabled))
				continue
			}
			padded := pad(c, widths[i])
			b.WriteString(styleCell(i, padded, r, colorEnabled))
			b.WriteString("  ")
		}
		b.WriteByte('\n')
	}

	writeHeader()
	for i, row := range cells {
		writeDataRow(row, rows[i])
	}
	return b.String()
}

// renderStacked renders each row as a block of lines, for terminals too
// narrow for the table:
//
//	> team
//	  stack  plan  age  open →
//	  title
//
// Each field is truncated to fit cols, except a raw URL fallback, which is
// left whole on its own line so it can still be copied. The stack shares
// a line with the plan, age and link when it fits, and gets its own line
// otherwise.
func renderStacked(rows []Row, linksSupported, colorEnabled bool, cols int) string {
	const indent = "  "
	inner := max(cols-len(indent), 1)
	var b strings.Builder
	line := func(parts ...string) {
		b.WriteString(indent)
		b.WriteString(strings.Join(parts, "  "))
		b.WriteByte('\n')
	}
	for i, r := range rows {
		if i > 0 {
			b.WriteByte('\n')
		}
		marker := " "
		if r.Selected {
			marker = ">"
		}
		b.WriteString(styleCell(0, marker, r, colorEnabled) + " ")
		b.WriteString(styleCell(1, truncate(r.Team, inner), r, colorEnabled))
		b.WriteByte('\n')

		var meta []string
		metaW := 0
		for _, f := range []struct {
			col  int
			text string
		}{{4, r.planText()}, {5, r.Age}} {
			if f.text != "" {
				meta = append(meta, styleCell(f.col, f.text, r, colorEnabled))
				metaW += uniseg.StringWidth(f.text) + 2
			}
		}
		if linksSupported {
			meta = append(meta, styleCell(6, Hyperlink(r.URL, linkText), r, colorEnabled))
			metaW += uniseg.StringWidth(linkText) + 2
		}
		stack := styleCell(2, truncate(r.Stack, inner), r, colorEnabled)
		if uniseg.StringWidth(r.Stack)+metaW <= inner {
			line(append([]string{stack}, meta...)...)
		} else {
			line(stack)
			if len(meta) > 0 {
				line(meta...)
			}
		}

		if r.Title != "" {
			line(styleCell(3, truncate(r.Title, inner), r, colorEnabled))
		}
		if !linksSupported {
			line(r.URL)
		}
	}
	return b.String()
}

// styleCell applies per-column coloring: TEAM is cyan, PLAN counts are
// green, amber and red for additions, changes and deletions, AGE is dim, the
// marker is highlighted when its row is selected, and the remaining
// columns are bolded on the selected row for a full-row highlight.
func styleCell(col int, s string, r Row, colorEnabled bool) string {
	if !colorEnabled {
		return s
	}
	switch col {
	case 0: // marker
		if r.Selected {
			return Style(s, BoldYellow, true)
		}
		return s
	case 1: // team
		return Style(s, Cyan, true)
	case 4: // plan
		if r.Plan == nil {
			return s
		}
		// s may carry trailing padding after the plan text.
		return r.Plan.styled() + strings.TrimPrefix(s, r.Plan.String())
	case 5: // age
		return Style(s, Dim, true)
	default:
		if r.Selected {
			return Style(s, Bold, true)
		}
		return s
	}
}

// truncate shortens s to at most max terminal cells, ending in "…" when
// anything was cut.
func truncate(s string, max int) string {
	if uniseg.StringWidth(s) <= max {
		return s
	}
	if max <= 0 {
		return ""
	}
	var b strings.Builder
	w := 0
	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		gw := gr.Width()
		if w+gw > max-1 {
			break
		}
		b.WriteString(gr.Str())
		w += gw
	}
	return b.String() + "…"
}

// pad right-pads s with spaces to w terminal cells.
func pad(s string, w int) string {
	if n := w - uniseg.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

// FormatAge renders a duration as a short, human-scaled age like "3m",
// "2h", or "5d".
func FormatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// Hyperlink wraps text in an OSC 8 terminal hyperlink escape sequence.
func Hyperlink(url, text string) string {
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// SupportsLinks reports whether the current terminal is known to render
// OSC 8 hyperlinks, based on common terminal-identifying environment
// variables. Terminals not in this list still work fine - they just show
// the plain URL fallback instead of a clickable link.
func SupportsLinks() bool {
	return isTTY(os.Stdout) && envSupportsLinks(os.Getenv)
}

func isTTY(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// ColorEnabled reports whether ANSI color output should be used: it's
// suppressed when stdout isn't a terminal (e.g. output is piped/redirected)
// or when NO_COLOR is set, per the https://no-color.org convention.
func ColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isTTY(os.Stdout)
}

func envSupportsLinks(getenv func(string) string) bool {
	switch getenv("TERM_PROGRAM") {
	case "iTerm.app", "WezTerm", "kitty", "ghostty", "Apple_Terminal":
		return true
	}
	if getenv("VTE_VERSION") != "" {
		return true // GNOME Terminal, Tilix, and other VTE-based terminals
	}
	if getenv("WT_SESSION") != "" {
		return true // Windows Terminal
	}
	if getenv("TERM") == "xterm-kitty" {
		return true
	}
	return false
}

// EndsAtMargin reports whether writing s from column 0 of a terminal cols
// wide leaves the cursor in the deferred-wrap state: the last character
// landed in the final column, so the cursor still sits on that cell until
// the next character arrives. It simulates wrapping glyph by glyph rather
// than comparing the total width, because a double-width character that
// doesn't fit in a row's last cell wraps early and leaves that cell empty.
// Escape sequences (SGR colors, OSC 8 hyperlinks) take no cells.
func EndsAtMargin(s string, cols int) bool {
	if cols <= 0 {
		return false
	}
	col := 0
	gr := uniseg.NewGraphemes(stripEscapes(s))
	for gr.Next() {
		// Measured per grapheme cluster, not per rune, so a joined emoji
		// like 👩‍💻 counts as the one two-cell glyph terminals draw.
		w := gr.Width()
		if w == 0 {
			continue
		}
		if col+w > cols {
			col = 0 // wraps onto the next row (early, for a wide character)
		}
		col += w
	}
	return col == cols
}

// stripEscapes removes CSI sequences (e.g. SGR colors) and OSC sequences
// (e.g. OSC 8 hyperlinks) from s.
func stripEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != 0x1b || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		switch s[i+1] {
		case '[': // CSI: parameters, then a final byte in 0x40-0x7e
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			i = j
		case ']': // OSC: terminated by BEL or ST (ESC \)
			j := i + 2
			for j < len(s) && s[j] != 0x07 && !(s[j] == 0x1b && j+1 < len(s) && s[j+1] == '\\') {
				j++
			}
			if j < len(s) && s[j] == 0x1b {
				j++
			}
			i = j
		default:
			i++
		}
	}
	return b.String()
}

// TerminalWidth returns stdout's width in columns, or 0 if it isn't a
// terminal.
func TerminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0
	}
	return w
}
