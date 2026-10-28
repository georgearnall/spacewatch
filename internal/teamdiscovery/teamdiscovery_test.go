package teamdiscovery

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestMatch(t *testing.T) {
	teams := []string{"DataPlatform", "Search & Discovery", "Engineering", "Mobile", "data-platform"}
	labels := []string{
		"folder:owning-team/search-discovery",
		"folder:collab-team/search-discovery",
		"folder:owning-team/data-platform",
		"folder:owning-team/dataplatform",
		"folder:collab-team/dataplatform",
		"folder:owning-team/mobile",
		"folder:owning-team/payments",
		"engineering", // not a team label, must not match the Engineering group
		"folder:owning-team/",
	}
	counts := []int{24, 19, 39, 14, 54, 2, 100, 500, 3}

	got := match(teams, labels, counts)
	want := []Candidate{
		{Name: "DataPlatform", Slugs: []string{"dataplatform", "data-platform"}, Stacks: 107},
		{Name: "Search & Discovery", Slugs: []string{"search-discovery"}, Stacks: 43},
		{Name: "Mobile", Slugs: []string{"mobile"}, Stacks: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("match() =\n%+v\nwant\n%+v", got, want)
	}
}

func TestMatch_NoTeams(t *testing.T) {
	if got := match(nil, []string{"folder:owning-team/payments"}, []int{1}); got != nil {
		t.Errorf("match() with no teams = %+v, want nil", got)
	}
}

type fakeClient struct {
	resp   discoverQuery
	counts map[string]int // first label of a count predicate -> filteredCount
	err    error
	inputs []SearchSuggestionsInput
}

func (f *fakeClient) Query(_ context.Context, out any, vars map[string]any) error {
	in := vars["input"].(SearchSuggestionsInput)
	f.inputs = append(f.inputs, in)
	if f.err != nil {
		return f.err
	}
	switch q := out.(type) {
	case *discoverQuery:
		*q = f.resp
	case *countQuery:
		q.Suggestions.FilteredCount = f.counts[in.Predicates[0].Constraint.StringEquals[0]]
	}
	return nil
}

func TestDiscover(t *testing.T) {
	var resp discoverQuery
	resp.Viewer.Teams = []string{"Mobile", "Identity"}
	resp.Suggestions.Fields = []suggestionField{
		{Name: "name"},
		{Name: "label", PossibleValuesString: &stringValues{
			Values: []string{"folder:owning-team/mobile", "folder:owning-team/identity", "folder:collab-team/identity"},
			Counts: []int{25, 16, 15},
		}},
	}
	// Identity' labels overlap: 31 label matches, 20 distinct stacks,
	// which drops it below Mobile.
	fc := &fakeClient{resp: resp, counts: map[string]int{
		"folder:owning-team/mobile":   25,
		"folder:owning-team/identity": 20,
	}}

	got, err := Discover(context.Background(), fc)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	want := []Candidate{
		{Name: "Mobile", Slugs: []string{"mobile"}, Stacks: 25},
		{Name: "Identity", Slugs: []string{"identity"}, Stacks: 20},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Discover() = %+v, want %+v", got, want)
	}

	if len(fc.inputs) != 3 || !reflect.DeepEqual(fc.inputs[0].Fields, []string{"label"}) {
		t.Fatalf("Discover() inputs = %+v, want a label suggestions query then one count per candidate", fc.inputs)
	}
	wantLabels := []string{"folder:owning-team/identity", "folder:collab-team/identity"}
	if got := fc.inputs[1].Predicates[0].Constraint.StringEquals; !reflect.DeepEqual(got, wantLabels) {
		t.Errorf("count predicate = %v, want %v", got, wantLabels)
	}
}

func TestDiscover_QueryError(t *testing.T) {
	if _, err := Discover(context.Background(), &fakeClient{err: errors.New("boom")}); err == nil {
		t.Fatal("Discover() error = nil, want one")
	}
}
