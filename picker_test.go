package main

import (
	"bufio"
	"slices"
	"strings"
	"testing"

	"github.com/georgearnall/spacewatch/internal/teamdiscovery"
)

var testCandidates = []teamdiscovery.Candidate{
	{Name: "DataPlatform", Slugs: []string{"dataplatform", "data-platform"}, Stacks: 115},
	{Name: "Search & Discovery", Slugs: []string{"search-discovery"}, Stacks: 43},
}

func TestNewPickerState_PreselectsCurrentTeams(t *testing.T) {
	s := newPickerState(testCandidates, []string{"data-platform", "dataplatform", "payments"}, "")

	if got, want := s.selectedSlugs(), []string{"dataplatform", "data-platform", "payments"}; !slices.Equal(got, want) {
		t.Errorf("selectedSlugs() = %v, want %v", got, want)
	}
	if !s.items[0].selected {
		t.Error("DataPlatform row not pre-selected, want it selected since all its slugs are current")
	}
	if last := s.items[len(s.items)-1]; !last.manual || last.name != "payments" {
		t.Errorf("last item = %+v, want manual row for payments", last)
	}
}

func TestNewPickerState_PartialMatchKeepsExactSlugs(t *testing.T) {
	s := newPickerState(testCandidates, []string{"data-platform"}, "")

	if got, want := s.selectedSlugs(), []string{"data-platform"}; !slices.Equal(got, want) {
		t.Errorf("selectedSlugs() = %v, want %v (confirming must not add dataplatform)", got, want)
	}
	if s.items[0].selected {
		t.Error("DataPlatform row pre-selected, want only a manual data-platform row")
	}
}

func TestPickerApply_ToggleAndConfirm(t *testing.T) {
	s := newPickerState(testCandidates, nil, "")

	s.apply(keyDown)
	s.apply(keySpace)
	if got := s.apply(keyEnter); got != pickerDone {
		t.Fatalf("apply(enter) = %v, want pickerDone", got)
	}
	if got, want := s.selectedSlugs(), []string{"search-discovery"}; !slices.Equal(got, want) {
		t.Errorf("selectedSlugs() = %v, want %v", got, want)
	}
}

func TestPickerApply_EnterWithNothingSelectedIsRejected(t *testing.T) {
	s := newPickerState(testCandidates, nil, "")

	if got := s.apply(keyEnter); got != pickerContinue {
		t.Fatalf("apply(enter) = %v, want pickerContinue", got)
	}
	if s.hint == "" {
		t.Error("hint is empty, want a nudge to select something")
	}
	s.apply(keySpace)
	if s.hint != "" {
		t.Errorf("hint = %q, want it cleared by the next key", s.hint)
	}
}

func TestPickerApply_CursorClampsAndReachesManualRow(t *testing.T) {
	s := newPickerState(testCandidates, nil, "")

	s.apply(keyUp)
	if s.cursor != 0 {
		t.Errorf("cursor after up at top = %d, want 0", s.cursor)
	}
	for range 5 {
		s.apply(keyDown)
	}
	if !s.manualRow() {
		t.Fatalf("cursor = %d, want the manual row (%d)", s.cursor, len(s.items))
	}
	if got := s.apply(keyEnter); got != pickerManual {
		t.Errorf("apply(enter) on manual row = %v, want pickerManual", got)
	}
	if got := s.apply(keySpace); got != pickerManual {
		t.Errorf("apply(space) on manual row = %v, want pickerManual", got)
	}
}

func TestPickerApply_QuitAborts(t *testing.T) {
	s := newPickerState(testCandidates, nil, "")
	if got := s.apply(keyQuit); got != pickerAbort {
		t.Errorf("apply(quit) = %v, want pickerAbort", got)
	}
}

func TestPickerAddManual(t *testing.T) {
	s := newPickerState(testCandidates, nil, "")
	s.cursor = len(s.items)

	s.addManual([]string{"data-platform", "search-discovery", "payments", "payments"})

	if got, want := s.selectedSlugs(), []string{"search-discovery", "data-platform", "payments"}; !slices.Equal(got, want) {
		t.Errorf("selectedSlugs() = %v, want %v", got, want)
	}
	if s.items[0].selected {
		t.Error("DataPlatform row selected, want typing data-platform to select only that slug")
	}
	if len(s.items) != 4 {
		t.Errorf("items = %+v, want manual rows added for data-platform and payments only", s.items)
	}
}

func TestReadKey_LeavesFollowingBytesBuffered(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\x1b[B\rpayments\n"))

	for _, want := range []key{keyDown, keyEnter} {
		got, err := readKey(r)
		if err != nil || got != want {
			t.Fatalf("readKey() = %v, %v; want %v", got, err, want)
		}
	}
	line, err := r.ReadString('\n')
	if err != nil || line != "payments\n" {
		t.Errorf("rest = %q, %v; want the pasted slug still buffered", line, err)
	}
}

func TestReadLine(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("payments\rnext\r\nlast\n"))
	for _, want := range []string{"payments", "next", "last"} {
		got, err := readLine(r)
		if err != nil || got != want {
			t.Fatalf("readLine() = %q, %v; want %q", got, err, want)
		}
	}
}

func TestReadKey_LoneEscDoesNotBlock(t *testing.T) {
	// An io.Reader that fails the test if read again after the Esc.
	r := bufio.NewReader(&oneShotReader{data: []byte{0x1b}, t: t})
	if got, err := readKey(r); err != nil || got != keyNone {
		t.Errorf("readKey() = %v, %v; want keyNone", got, err)
	}
}

type oneShotReader struct {
	data []byte
	t    *testing.T
	done bool
}

func (o *oneShotReader) Read(p []byte) (int, error) {
	if o.done {
		o.t.Fatal("readKey read past a lone Esc, which would block on a terminal")
	}
	o.done = true
	return copy(p, o.data), nil
}

func TestPhysicalRows(t *testing.T) {
	cases := []struct {
		line string
		cols int
		want int
	}{
		{"", 80, 1},
		{"abc", 3, 1},
		{"abcd", 3, 2},
		{"abcdefg", 3, 3},
		{"abcd", 0, 1},
		{"·→↓", 2, 2},
	}
	for _, c := range cases {
		if got := physicalRows(c.line, c.cols); got != c.want {
			t.Errorf("physicalRows(%q, %d) = %d, want %d", c.line, c.cols, got, c.want)
		}
	}
}

func TestRenderPicker(t *testing.T) {
	s := newPickerState(testCandidates, []string{"payments"}, "Couldn't discover your teams.")
	s.apply(keySpace)

	got := renderPicker(s)
	for _, want := range []string{
		"Couldn't discover your teams.\r\n",
		"> [x] DataPlatform  (dataplatform, data-platform · 115 stacks)\r\n",
		"  [ ] Search & Discovery  (search-discovery · 43 stacks)\r\n",
		"  [x] payments  (added manually)\r\n",
		"  + Type team slugs manually...\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("renderPicker() missing %q in:\n%s", want, got)
		}
	}
}

func TestParseTeamList(t *testing.T) {
	got := parseTeamList(" payments, ,checkout,payments ")
	if want := []string{"payments", "checkout"}; !slices.Equal(got, want) {
		t.Errorf("parseTeamList() = %v, want %v", got, want)
	}
}
