// Package datadir loads a data directory (machines/, os/,
// compatibility.yml, env.yml, templates/) into a DataSet.
//
// The loader does basic consistency checks: every Machine listed in
// compatibility.yml must exist as machines/<id>.yml, every OS must
// exist as os/<id>.yml, and every template referenced by an OS must
// exist in templates/.
//
// Loading is read-only and idempotent.
package datadir

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/zinrai/metal-install/internal/data"
)

// Load reads a data directory and returns a fully populated DataSet.
//
// The directory layout is:
//
//	root/
//	  machines/*.yml
//	  os/*.yml
//	  compatibility.yml
//	  env.yml
//	  templates/**/*.tmpl
//
// Returns an error if any required file is missing, any YAML is
// invalid, or any cross-reference (compat -> machine/os, os ->
// template) is broken.
func Load(root string) (*data.DataSet, error) {
	ds := &data.DataSet{
		Templates: make(map[string]string),
	}

	var err error
	if ds.Machines, err = loadDir[data.Machine](filepath.Join(root, "machines")); err != nil {
		return nil, fmt.Errorf("load machines: %w", err)
	}
	if ds.OS, err = loadDir[data.OS](filepath.Join(root, "os")); err != nil {
		return nil, fmt.Errorf("load os: %w", err)
	}
	if err := loadCompat(root, ds); err != nil {
		return nil, fmt.Errorf("load compatibility: %w", err)
	}
	if err := loadEnv(root, ds); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}
	if err := loadTemplates(root, ds); err != nil {
		return nil, fmt.Errorf("load templates: %w", err)
	}
	if err := validate(ds); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	return ds, nil
}

func readYAML(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, v)
}

// loadDir reads every <id>.yml in dir into a map keyed by <id>.
func loadDir[T any](dir string) (map[string]*T, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	loaded := make(map[string]*T)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		var v T
		if err := readYAML(filepath.Join(dir, e.Name()), &v); err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		loaded[strings.TrimSuffix(e.Name(), ".yml")] = &v
	}
	return loaded, nil
}

func loadCompat(root string, ds *data.DataSet) error {
	return readYAML(filepath.Join(root, "compatibility.yml"), &ds.Compat)
}

func loadEnv(root string, ds *data.DataSet) error {
	var env data.Env
	if err := readYAML(filepath.Join(root, "env.yml"), &env); err != nil {
		return err
	}
	ds.Env = &env
	return nil
}

// loadTemplates reads every regular file under templates/ recursively
// and stores it keyed by its path relative to root (e.g.
// "templates/rhel/boot.ipxe.tmpl"). Subdirectories are allowed; their
// names have no semantic meaning to the loader, but conventionally
// group templates by OS family.
func loadTemplates(root string, ds *data.DataSet) error {
	dir := filepath.Join(root, "templates")
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		ds.Templates[filepath.ToSlash(rel)] = string(b)
		return nil
	})
}

// validate checks cross-references after all files are loaded.
func validate(ds *data.DataSet) error {
	for i, c := range ds.Compat {
		if err := validateCompatEntry(ds, c); err != nil {
			return fmt.Errorf("compat[%d]: %w", i, err)
		}
	}
	for id, o := range ds.OS {
		if err := validateOS(ds.Templates, o); err != nil {
			return fmt.Errorf("os %s: %w", id, err)
		}
	}
	return nil
}

func validateCompatEntry(ds *data.DataSet, c data.CompatEntry) error {
	for _, mid := range c.Machines {
		if _, ok := ds.Machines[mid]; !ok {
			return fmt.Errorf("unknown machine %q", mid)
		}
	}
	for _, oid := range c.OS {
		if _, ok := ds.OS[oid]; !ok {
			return fmt.Errorf("unknown os %q", oid)
		}
	}
	return nil
}

func validateOS(templates map[string]string, o *data.OS) error {
	if o.BootTemplate == "" {
		return fmt.Errorf("boot_template is required")
	}
	if _, ok := templates[o.BootTemplate]; !ok {
		return fmt.Errorf("boot_template %q not found", o.BootTemplate)
	}
	if len(o.Configs) == 0 {
		return fmt.Errorf("configs must list at least one entry")
	}
	for i, c := range o.Configs {
		if err := validateConfig(templates, c); err != nil {
			return fmt.Errorf("configs[%d]: %w", i, err)
		}
	}
	if len(o.Setup.Post) == 0 {
		return fmt.Errorf("setup.post must list at least one script")
	}
	for i, t := range o.Setup.Post {
		if err := validatePost(templates, t); err != nil {
			return fmt.Errorf("setup.post[%d]: %w", i, err)
		}
	}
	return nil
}

func validateConfig(templates map[string]string, c data.ConfigOutput) error {
	if c.Template == "" {
		return fmt.Errorf("template is required")
	}
	if c.Filename == "" {
		return fmt.Errorf("filename is required")
	}
	if _, ok := templates[c.Template]; !ok {
		return fmt.Errorf("template %q not found", c.Template)
	}
	return nil
}

func validatePost(templates map[string]string, t string) error {
	if t == "" {
		return fmt.Errorf("template is required")
	}
	if _, ok := templates[t]; !ok {
		return fmt.Errorf("template %q not found", t)
	}
	if !strings.HasSuffix(t, ".tmpl") {
		return fmt.Errorf("%q must end with .tmpl", t)
	}
	return nil
}

// IsCompatible reports whether the given machine and OS combination
// is declared as valid in compatibility.yml.
func IsCompatible(ds *data.DataSet, machineID, osID string) bool {
	for _, c := range ds.Compat {
		if slices.Contains(c.Machines, machineID) && slices.Contains(c.OS, osID) {
			return true
		}
	}
	return false
}
