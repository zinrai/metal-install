package datadir

import (
	"testing"

	"github.com/zinrai/metal-install/internal/data"
)

// TestIsCompatible verifies that a machine and an OS are compatible
// only when one compat entry lists both.
func TestIsCompatible(t *testing.T) {
	ds := &data.DataSet{
		Compat: []data.CompatEntry{
			{
				Machines: []string{"machine_b", "machine_a"},
				OS:       []string{"almalinux100", "debian12"},
			},
			{
				Machines: []string{"machine_b"},
				OS:       []string{"ubuntu2404"},
			},
		},
	}

	tests := []struct {
		machine string
		os      string
		want    bool
	}{
		{"machine_b", "almalinux100", true},
		{"machine_b", "debian12", true},
		{"machine_b", "ubuntu2404", true},
		{"machine_a", "almalinux100", true},
		{"machine_a", "debian12", true},
		{"machine_a", "ubuntu2404", false},
		{"unknown_machine", "almalinux100", false},
		{"machine_b", "unknown_os", false},
		{"unknown", "unknown", false},
	}

	for _, tc := range tests {
		got := IsCompatible(ds, tc.machine, tc.os)
		if got != tc.want {
			t.Errorf("IsCompatible(%q, %q) = %v, want %v",
				tc.machine, tc.os, got, tc.want)
		}
	}
}

func TestIsCompatible_EmptyCompat(t *testing.T) {
	ds := &data.DataSet{Compat: nil}
	if IsCompatible(ds, "any", "any") {
		t.Error("empty compat must reject everything")
	}
}
