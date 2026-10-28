package labels

import "testing"

func TestMatch(t *testing.T) {
	cfg := Config{Labels: []string{"folder:owning-team/checkout", "folder:owning-team/payments"}}

	cases := []struct {
		name      string
		labels    []string
		wantMatch bool
		wantOn    string
	}{
		{
			name:      "exact match on first configured label",
			labels:    []string{"folder:owning-team/checkout", "disable_autorun"},
			wantMatch: true,
			wantOn:    "folder:owning-team/checkout",
		},
		{
			name:      "exact match on second configured label",
			labels:    []string{"folder:owning-team/payments"},
			wantMatch: true,
			wantOn:    "folder:owning-team/payments",
		},
		{
			name:      "no match for unrelated team",
			labels:    []string{"folder:owning-team/platform"},
			wantMatch: false,
		},
		{
			name:      "substring is not a match - exact only",
			labels:    []string{"folder:owning-team/checkout-extra"},
			wantMatch: false,
		},
		{
			name:      "substring is not a match - prefix only",
			labels:    []string{"checkout"},
			wantMatch: false,
		},
		{
			name:      "empty label list never matches",
			labels:    nil,
			wantMatch: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotMatch, gotOn := Match(c.labels, cfg)
			if gotMatch != c.wantMatch {
				t.Errorf("Match(%v) matched = %v, want %v", c.labels, gotMatch, c.wantMatch)
			}
			if gotMatch && gotOn != c.wantOn {
				t.Errorf("Match(%v) matchedOn = %q, want %q", c.labels, gotOn, c.wantOn)
			}
		})
	}
}

func TestMatch_EmptyConfig(t *testing.T) {
	got, _ := Match([]string{"folder:owning-team/checkout"}, Config{})
	if got {
		t.Errorf("Match with empty config should never match, got true")
	}
}
