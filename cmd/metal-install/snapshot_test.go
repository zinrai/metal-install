package main

import (
	"reflect"
	"testing"

	"github.com/zinrai/metal-install/internal/data"
)

// TestAllowedPairs verifies that allowedPairs returns the union of
// every compat entry's machine x os pairs, without duplicates, in
// sorted order.
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
						Machines: []string{"machine_b", "machine_a"},
						OS:       []string{"almalinux100", "debian12"},
					},
				},
			},
			want: []pair{
				{"machine_a", "almalinux100"},
				{"machine_a", "debian12"},
				{"machine_b", "almalinux100"},
				{"machine_b", "debian12"},
			},
		},
		{
			name: "multiple compat entries are unioned",
			ds: &data.DataSet{
				Compat: []data.CompatEntry{
					{Machines: []string{"machine_b"}, OS: []string{"almalinux100"}},
					{Machines: []string{"machine_b"}, OS: []string{"ubuntu2404"}},
				},
			},
			want: []pair{
				{"machine_b", "almalinux100"},
				{"machine_b", "ubuntu2404"},
			},
		},
		{
			name: "duplicates across compat entries are deduplicated",
			ds: &data.DataSet{
				Compat: []data.CompatEntry{
					{Machines: []string{"machine_b"}, OS: []string{"almalinux100"}},
					{Machines: []string{"machine_b"}, OS: []string{"almalinux100"}},
				},
			},
			want: []pair{
				{"machine_b", "almalinux100"},
			},
		},
		{
			name: "input order does not affect output order",
			ds: &data.DataSet{
				Compat: []data.CompatEntry{
					{
						Machines: []string{"machine_b", "machine_a"},
						OS:       []string{"debian12", "almalinux100"},
					},
				},
			},
			want: []pair{
				{"machine_a", "almalinux100"},
				{"machine_a", "debian12"},
				{"machine_b", "almalinux100"},
				{"machine_b", "debian12"},
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

// TestAllowedPairs_Determinism verifies that repeated calls on the
// same input return the pairs in the same order.
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
