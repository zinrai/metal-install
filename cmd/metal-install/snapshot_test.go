package main

import (
	"reflect"
	"testing"

	"github.com/zinrai/metal-install/internal/data"
)

// TestAllowedPairs covers the function that drives `metal-install
// snapshot`: which (machine, os) combinations does it expand?
//
// The git-diff-based workflow relies on this function producing the
// same set, in the same order, every run. A regression here would
// silently change snapshot output (extra files, missing files, or
// reordered output that makes diffs unreadable) -- the kind of bug
// that does not surface as an error message.
func TestAllowedPairs(t *testing.T) {
	tests := []struct {
		name string
		ds   *data.DataSet
		want []pair
	}{
		{
			name: "single compat entry expands the cross product",
			ds: &data.DataSet{
				Compat: []data.CompatEntry{
					{
						Machines: []string{"r660_10g", "dl380_gen10"},
						OS:       []string{"almalinux100", "debian12"},
					},
				},
			},
			want: []pair{
				{"dl380_gen10", "almalinux100"},
				{"dl380_gen10", "debian12"},
				{"r660_10g", "almalinux100"},
				{"r660_10g", "debian12"},
			},
		},
		{
			name: "multiple compat entries are unioned",
			ds: &data.DataSet{
				Compat: []data.CompatEntry{
					{Machines: []string{"r660_10g"}, OS: []string{"almalinux100"}},
					{Machines: []string{"r660_10g"}, OS: []string{"ubuntu2404"}},
				},
			},
			want: []pair{
				{"r660_10g", "almalinux100"},
				{"r660_10g", "ubuntu2404"},
			},
		},
		{
			name: "duplicates across compat entries are deduplicated",
			ds: &data.DataSet{
				Compat: []data.CompatEntry{
					{Machines: []string{"r660_10g"}, OS: []string{"almalinux100"}},
					{Machines: []string{"r660_10g"}, OS: []string{"almalinux100"}},
				},
			},
			want: []pair{
				{"r660_10g", "almalinux100"},
			},
		},
		{
			name: "input order does not affect output order",
			// Same set as the first case, but with reversed input
			// order. allowedPairs must still emit in sorted order.
			ds: &data.DataSet{
				Compat: []data.CompatEntry{
					{
						Machines: []string{"r660_10g", "dl380_gen10"},
						OS:       []string{"debian12", "almalinux100"},
					},
				},
			},
			want: []pair{
				{"dl380_gen10", "almalinux100"},
				{"dl380_gen10", "debian12"},
				{"r660_10g", "almalinux100"},
				{"r660_10g", "debian12"},
			},
		},
		{
			name: "empty compat yields empty result",
			ds:   &data.DataSet{Compat: nil},
			want: []pair{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := allowedPairs(tc.ds)
			if !pairsEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestAllowedPairs_Determinism runs allowedPairs many times on the
// same input and confirms the result is byte-identical. Go map
// iteration order is randomized, so a buggy implementation that
// forgets to sort would surface here.
func TestAllowedPairs_Determinism(t *testing.T) {
	ds := &data.DataSet{
		Compat: []data.CompatEntry{
			{
				Machines: []string{"a", "b", "c", "d", "e"},
				OS:       []string{"x", "y", "z"},
			},
		},
	}
	first := allowedPairs(ds)
	for i := 0; i < 50; i++ {
		next := allowedPairs(ds)
		if !reflect.DeepEqual(first, next) {
			t.Fatalf("non-deterministic output on iteration %d: first=%v next=%v",
				i, first, next)
		}
	}
}

func pairsEqual(a, b []pair) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
