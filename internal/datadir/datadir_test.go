package datadir

import (
	"strings"
	"testing"

	"github.com/zinrai/metal-install/internal/data"
)

// TestIsCompatible covers the public predicate used by render to
// reject (machine, os) combinations that are not declared. A
// regression here either lets disallowed combinations through, or
// blocks valid combinations -- both fail silently from the user's
// point of view (no validation error, just unexpected render
// behaviour).
func TestIsCompatible(t *testing.T) {
	ds := &data.DataSet{
		Compat: []data.CompatEntry{
			{
				Machines: []string{"r660_10g", "dl380_gen10"},
				OS:       []string{"almalinux100", "debian12"},
			},
			{
				Machines: []string{"r660_10g"},
				OS:       []string{"ubuntu2404"},
			},
		},
	}

	tests := []struct {
		machine string
		os      string
		want    bool
	}{
		{"r660_10g", "almalinux100", true},
		{"r660_10g", "debian12", true},
		{"r660_10g", "ubuntu2404", true},
		{"dl380_gen10", "almalinux100", true},
		{"dl380_gen10", "debian12", true},
		// dl380_gen10 + ubuntu2404 is not in compat
		{"dl380_gen10", "ubuntu2404", false},
		// Unknown machine
		{"unknown_machine", "almalinux100", false},
		// Unknown OS
		{"r660_10g", "unknown_os", false},
		// Both unknown
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

// TestValidate_Errors confirms that validate refuses each broken-data
// case it is meant to catch. The test asserts the error message
// contains a fragment that identifies which validation path was
// taken; this is stricter than `err != nil` (which would pass for
// the wrong-error case) but more robust than full-string match
// (which would break on any wording change).
func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name    string
		ds      *data.DataSet
		errFrag string
	}{
		{
			name: "compat references unknown machine",
			ds: makeDS(func(ds *data.DataSet) {
				ds.Compat = []data.CompatEntry{
					{Machines: []string{"nosuch"}, OS: []string{"os1"}},
				}
			}),
			errFrag: "unknown machine",
		},
		{
			name: "compat references unknown os",
			ds: makeDS(func(ds *data.DataSet) {
				ds.Compat = []data.CompatEntry{
					{Machines: []string{"m1"}, OS: []string{"nosuch"}},
				}
			}),
			errFrag: "unknown os",
		},
		{
			name: "os missing boot_template",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].BootTemplate = ""
			}),
			errFrag: "boot_template is required",
		},
		{
			name: "os boot_template not loaded",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].BootTemplate = "templates/missing.tmpl"
			}),
			errFrag: "boot_template",
		},
		{
			name: "os has no configs",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].Configs = nil
			}),
			errFrag: "configs must list at least one",
		},
		{
			name: "configs entry missing template",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].Configs[0].Template = ""
			}),
			errFrag: "template is required",
		},
		{
			name: "configs entry missing filename",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].Configs[0].Filename = ""
			}),
			errFrag: "filename is required",
		},
		{
			name: "configs template not loaded",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].Configs[0].Template = "templates/nosuch.tmpl"
			}),
			errFrag: "not found",
		},
		{
			name: "setup.post is empty",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].Setup.Post = nil
			}),
			errFrag: "setup.post must list at least one",
		},
		{
			name: "setup.post entry not loaded",
			ds: makeDS(func(ds *data.DataSet) {
				ds.OS["os1"].Setup.Post = []string{"templates/missing.tmpl"}
			}),
			errFrag: "not found",
		},
		{
			name: "setup.post entry missing .tmpl suffix",
			ds: makeDS(func(ds *data.DataSet) {
				// Replace post entry with one that exists in
				// Templates but lacks the .tmpl suffix.
				ds.Templates["templates/setup/post/odd"] = "ok"
				ds.OS["os1"].Setup.Post = []string{"templates/setup/post/odd"}
			}),
			errFrag: "must end with .tmpl",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validate("", tc.ds)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errFrag)
			}
			if !strings.Contains(err.Error(), tc.errFrag) {
				t.Errorf("error %q does not contain %q",
					err.Error(), tc.errFrag)
			}
		})
	}
}

// TestValidate_GoodData confirms that a well-formed DataSet passes
// validation without error.
func TestValidate_GoodData(t *testing.T) {
	ds := makeDS(nil)
	if err := validate("", ds); err != nil {
		t.Fatalf("good data must validate: %v", err)
	}
}

// makeDS returns a minimal well-formed DataSet. The optional mutator
// is applied after construction so test cases can break one specific
// invariant without re-stating the whole structure.
func makeDS(mutate func(*data.DataSet)) *data.DataSet {
	ds := &data.DataSet{
		Machines: map[string]*data.Machine{
			"m1": {},
		},
		OS: map[string]*data.OS{
			"os1": {
				BootTemplate: "templates/boot.tmpl",
				Configs: []data.ConfigOutput{
					{Template: "templates/cfg.tmpl", Filename: "cfg"},
				},
				Setup: data.Setup{
					Post: []string{"templates/setup/post/a.sh.tmpl"},
				},
			},
		},
		Templates: map[string]string{
			"templates/boot.tmpl":            "ok",
			"templates/cfg.tmpl":             "ok",
			"templates/setup/post/a.sh.tmpl": "ok",
		},
		Compat: []data.CompatEntry{
			{Machines: []string{"m1"}, OS: []string{"os1"}},
		},
	}
	if mutate != nil {
		mutate(ds)
	}
	return ds
}
