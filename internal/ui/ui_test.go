package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/rivo/uniseg"
)

func TestFormatAge(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m"},
		{3 * time.Hour, "3h"},
		{50 * time.Hour, "2d"},
	}
	for _, c := range cases {
		if got := FormatAge(c.d); got != c.want {
			t.Errorf("FormatAge(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestRenderTable_NoRows(t *testing.T) {
	got := RenderTable(nil, true, false, 0)
	if !strings.Contains(got, "no pending confirmations") {
		t.Errorf("RenderTable(nil) = %q, want an empty-state message", got)
	}
}

func TestRenderTable_LinksSupported(t *testing.T) {
	rows := []Row{{Team: "ecommerce", Stack: "orders-api", Title: "apply", Age: "5m", URL: "https://example.test/stack/orders-api/run/r1"}}
	got := RenderTable(rows, true, false, 0)
	if !strings.Contains(got, "\x1b]8;;https://example.test/stack/orders-api/run/r1\x1b\\") {
		t.Errorf("RenderTable() with links supported missing OSC 8 escape sequence: %q", got)
	}
	if !strings.Contains(got, "ecommerce") || !strings.Contains(got, "orders-api") {
		t.Errorf("RenderTable() = %q, missing expected cell content", got)
	}
}

func TestRenderTable_LinksUnsupported(t *testing.T) {
	rows := []Row{{Team: "ecommerce", Stack: "orders-api", Title: "apply", Age: "5m", URL: "https://example.test/stack/orders-api/run/r1"}}
	got := RenderTable(rows, false, false, 0)
	if strings.Contains(got, "\x1b]8;;") {
		t.Errorf("RenderTable() with links unsupported should not contain OSC 8 escapes: %q", got)
	}
	if !strings.Contains(got, "https://example.test/stack/orders-api/run/r1") {
		t.Errorf("RenderTable() with links unsupported should print the raw URL: %q", got)
	}
}

func TestRenderTable_ColumnsAlign(t *testing.T) {
	rows := []Row{
		{Team: "e", Stack: "short", Title: "t", Age: "1m", URL: "u"},
		{Team: "ecommerce-long-name", Stack: "s", Title: "t", Age: "1m", URL: "u"},
	}
	got := RenderTable(rows, false, false, 0)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 { // header + 2 rows
		t.Fatalf("RenderTable() produced %d lines, want 3", len(lines))
	}
	// The STACK column should start at the same byte offset on every line.
	headerStackCol := strings.Index(lines[0], "STACK")
	row2StackCol := strings.Index(lines[2], "s")
	if headerStackCol != row2StackCol {
		t.Errorf("columns not aligned: header STACK at %d, row2's stack cell at %d", headerStackCol, row2StackCol)
	}
}

func TestRenderTable_ColumnsAlign_WithColorEnabled(t *testing.T) {
	// Color codes are zero-width visually but non-zero-length in bytes;
	// this guards against padding being computed after coloring (which
	// would misalign columns since %-*s would count escape bytes).
	rows := []Row{
		{Team: "e", Stack: "short", Title: "t", Age: "1m", URL: "u"},
		{Team: "ecommerce-long-name", Stack: "s", Title: "t", Age: "1m", URL: "u", Selected: true},
	}
	got := RenderTable(rows, false, true, 0)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("RenderTable() produced %d lines, want 3", len(lines))
	}
	// Strip color codes before checking alignment, since the STACK cell
	// content itself ("s") is plain but preceded by reset/color escapes.
	stripped := make([]string, len(lines))
	for i, l := range lines {
		stripped[i] = stripANSI(l)
	}
	headerStackCol := strings.Index(stripped[0], "STACK")
	row2StackCol := strings.Index(stripped[2], "s")
	if headerStackCol != row2StackCol {
		t.Errorf("columns not aligned with color enabled: header STACK at %d, row2's stack cell at %d\nraw: %q", headerStackCol, row2StackCol, lines[2])
	}
}

func TestRenderTable_ColumnsAlign_ByDisplayWidth(t *testing.T) {
	// A truncated title ends in a 3-byte, 1-cell "…", and the stack name
	// contains wide characters, so byte-length padding would misalign.
	rows := []Row{
		{Team: "e", Stack: "日本", Title: strings.Repeat("x", 80), Age: "1m", URL: "u"},
		{Team: "e", Stack: "s", Title: "t", Age: "1m", URL: "u"},
	}
	lines := strings.Split(strings.TrimRight(RenderTable(rows, false, false, 0), "\n"), "\n")
	want := uniseg.StringWidth(lines[0][:strings.Index(lines[0], "LINK")])
	for i, l := range lines[1:] {
		if got := uniseg.StringWidth(l[:strings.LastIndex(l, "u")]); got != want {
			t.Errorf("row %d: LINK column at cell %d, want %d\n%q", i, got, want, l)
		}
	}
}

func TestRenderTable_ColorEnabled(t *testing.T) {
	rows := []Row{{Team: "ecommerce", Stack: "orders-api", Title: "apply", Age: "5m", URL: "u", Selected: true}}
	got := RenderTable(rows, false, true, 0)
	if !strings.Contains(got, Bold) {
		t.Errorf("RenderTable() with color enabled missing bold header/selection: %q", got)
	}
	if !strings.Contains(got, Cyan) {
		t.Errorf("RenderTable() with color enabled missing cyan TEAM styling: %q", got)
	}
	if !strings.Contains(got, Dim) {
		t.Errorf("RenderTable() with color enabled missing dim AGE styling: %q", got)
	}
	if !strings.Contains(got, BoldYellow) {
		t.Errorf("RenderTable() with color enabled and a selected row missing marker highlight: %q", got)
	}
}

func TestRenderTable_ColorDisabled_NoEscapeCodes(t *testing.T) {
	rows := []Row{{Team: "ecommerce", Stack: "orders-api", Title: "apply", Age: "5m", URL: "u", Selected: true}}
	got := RenderTable(rows, false, false, 0)
	if strings.Contains(got, "\x1b[") {
		t.Errorf("RenderTable() with color disabled contains an escape code: %q", got)
	}
}

func TestStyle(t *testing.T) {
	if got := Style("text", Bold, false); got != "text" {
		t.Errorf("Style() with enabled=false = %q, want unchanged", got)
	}
	if got := Style("", Bold, true); got != "" {
		t.Errorf("Style() on empty string = %q, want empty", got)
	}
	want := Bold + "text" + Reset
	if got := Style("text", Bold, true); got != want {
		t.Errorf("Style() = %q, want %q", got, want)
	}
}

// stripANSI removes SGR escape sequences (\x1b[...m) for test assertions
// that need to reason about visible text layout.
func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate(short) = %q, want unchanged", got)
	}
	got := truncate("this is a long title that exceeds the limit", 10)
	if uniseg.StringWidth(got) != 10 {
		t.Errorf("truncate() = %q, want exactly 10 cells", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncate() = %q, want ellipsis suffix", got)
	}
	// A wide character that would straddle the limit is dropped whole.
	if got := truncate("ab日本語", 5); got != "ab日…" {
		t.Errorf("truncate(wide) = %q, want %q", got, "ab日…")
	}
}

func TestHyperlink(t *testing.T) {
	got := Hyperlink("https://example.test", "click me")
	want := "\x1b]8;;https://example.test\x1b\\click me\x1b]8;;\x1b\\"
	if got != want {
		t.Errorf("Hyperlink() = %q, want %q", got, want)
	}
}

func TestEnvSupportsLinks(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"iTerm", map[string]string{"TERM_PROGRAM": "iTerm.app"}, true},
		{"kitty via TERM", map[string]string{"TERM": "xterm-kitty"}, true},
		{"VTE-based terminal", map[string]string{"VTE_VERSION": "6003"}, true},
		{"Windows Terminal", map[string]string{"WT_SESSION": "abc"}, true},
		{"unknown terminal", map[string]string{"TERM": "xterm"}, false},
		{"nothing set", map[string]string{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			getenv := func(k string) string { return c.env[k] }
			if got := envSupportsLinks(getenv); got != c.want {
				t.Errorf("envSupportsLinks() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRenderTable_PlanColumn(t *testing.T) {
	rows := []Row{
		{Team: "t", Stack: "safe", Title: "x", Plan: "+1 ~2 -0", Age: "1m", URL: "u"},
		{Team: "t", Stack: "risky", Title: "x", Plan: "+0 ~0 -3", Destroys: true, Age: "1m", URL: "u"},
	}
	got := RenderTable(rows, false, true, 0)
	if !strings.Contains(got, "PLAN") || !strings.Contains(got, "+1 ~2 -0") {
		t.Errorf("RenderTable() missing PLAN column: %q", got)
	}
	if !strings.Contains(got, BoldRed+"+0 ~0 -3") {
		t.Errorf("RenderTable() did not highlight a destroying plan in red: %q", got)
	}
	if strings.Contains(got, BoldRed+"+1 ~2 -0") {
		t.Errorf("RenderTable() highlighted a non-destroying plan: %q", got)
	}
}

func TestEndsAtMargin(t *testing.T) {
	cases := []struct {
		name string
		in   string
		cols int
		want bool
	}{
		{"short line", "abc", 10, false},
		{"exact width", "1234567890", 10, true},
		{"exact width, colored", Style("1234567890", Bold, true), 10, true},
		{"hyperlink text only counts", "12345" + Hyperlink("https://example.test/run", "open →"), 11, true},
		{"wraps to exactly two rows", strings.Repeat("x", 20), 10, true},
		{"wraps with a partial second row", strings.Repeat("x", 15), 10, false},
		{"wide char wraps early, fills two rows", "123456789日abcdefgh", 10, true},
		{"wide char exactly fits", "12345678日", 10, true},
		{"joined emoji is one two-cell glyph", "123456👩‍💻ab", 10, true},
		{"unknown width", "1234567890", 0, false},
	}
	for _, c := range cases {
		if got := EndsAtMargin(c.in, c.cols); got != c.want {
			t.Errorf("%s: EndsAtMargin(%q, %d) = %v, want %v", c.name, c.in, c.cols, got, c.want)
		}
	}
}

func TestRenderTable_ShrinksTitleToFit(t *testing.T) {
	rows := []Row{{Team: "team", Stack: "stack", Title: strings.Repeat("x", 80), Plan: "+1 ~0 -0", Age: "3m", URL: "u"}}
	wide := RenderTable(rows, true, true, 0)
	cols := uniseg.StringWidth(stripEscapes(strings.SplitN(wide, "\n", 3)[1])) - 15
	got := RenderTable(rows, true, true, cols)
	for _, l := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if w := uniseg.StringWidth(stripEscapes(l)); w > cols {
			t.Errorf("line is %d cells, want <= %d: %q", w, cols, l)
		}
	}
	if !strings.Contains(got, "TITLE") || !strings.Contains(got, "…") {
		t.Errorf("RenderTable() = %q, want a table with a truncated title", got)
	}
}

func TestRenderTable_FitsUnchanged(t *testing.T) {
	rows := []Row{{Team: "team", Stack: "stack", Title: "title", Age: "3m", URL: "u"}}
	if got, want := RenderTable(rows, true, true, 200), RenderTable(rows, true, true, 0); got != want {
		t.Errorf("RenderTable() at a wide terminal = %q, want the unconstrained table %q", got, want)
	}
}
