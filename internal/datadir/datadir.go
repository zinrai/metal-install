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
//	  templates/*.tmpl
//
// Returns an error if any required file is missing, any YAML is
// invalid, or any cross-reference (compat -> machine/os, os ->
// template) is broken.
func Load(root string) (*data.DataSet, error) {
	ds := &data.DataSet{
		Machines:  make(map[string]*data.Machine),
		OS:        make(map[string]*data.OS),
		Templates: make(map[string]string),
	}

	if err := loadMachines(root, ds); err != nil {
		return nil, fmt.Errorf("load machines: %w", err)
	}
	if err := loadOSDefs(root, ds); err != nil {
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
	if err := validate(root, ds); err != nil {
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

func loadMachines(root string, ds *data.DataSet) error {
	dir := filepath.Join(root, "machines")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yml")
		var m data.Machine
		if err := readYAML(filepath.Join(dir, e.Name()), &m); err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		ds.Machines[id] = &m
	}
	return nil
}

func loadOSDefs(root string, ds *data.DataSet) error {
	dir := filepath.Join(root, "os")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".yml")
		var o data.OS
		if err := readYAML(filepath.Join(dir, e.Name()), &o); err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
		ds.OS[id] = &o
	}
	return nil
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
func validate(root string, ds *data.DataSet) error {
	// Compatibility refers to existing machines and OSes.
	for i, c := range ds.Compat {
		for _, mid := range c.Machines {
			if _, ok := ds.Machines[mid]; !ok {
				return fmt.Errorf("compat[%d]: unknown machine %q", i, mid)
			}
		}
		for _, oid := range c.OS {
			if _, ok := ds.OS[oid]; !ok {
				return fmt.Errorf("compat[%d]: unknown os %q", i, oid)
			}
		}
	}
	// Each OS must declare boot, configs, and setup templates, and
	// every referenced template must exist in templates/.
	for id, o := range ds.OS {
		if o.BootTemplate == "" {
			return fmt.Errorf("os %s: boot_template is required", id)
		}
		if _, ok := ds.Templates[o.BootTemplate]; !ok {
			return fmt.Errorf("os %s: boot_template %q not found", id, o.BootTemplate)
		}
		if len(o.Configs) == 0 {
			return fmt.Errorf("os %s: configs must list at least one entry", id)
		}
		for i, c := range o.Configs {
			if c.Template == "" {
				return fmt.Errorf("os %s: configs[%d].template is required", id, i)
			}
			if c.Filename == "" {
				return fmt.Errorf("os %s: configs[%d].filename is required", id, i)
			}
			if _, ok := ds.Templates[c.Template]; !ok {
				return fmt.Errorf("os %s: configs[%d] template %q not found", id, i, c.Template)
			}
		}
		if len(o.Setup.Post) == 0 {
			return fmt.Errorf("os %s: setup.post must list at least one script", id)
		}
		for i, t := range o.Setup.Post {
			if t == "" {
				return fmt.Errorf("os %s: setup.post[%d] is required", id, i)
			}
			if _, ok := ds.Templates[t]; !ok {
				return fmt.Errorf("os %s: setup.post[%d] template %q not found", id, i, t)
			}
			if !strings.HasSuffix(t, ".tmpl") {
				return fmt.Errorf("os %s: setup.post[%d] %q must end with .tmpl", id, i, t)
			}
		}
	}
	return nil
}

// IsCompatible reports whether the given machine and OS combination
// is declared as valid in compatibility.yml.
func IsCompatible(ds *data.DataSet, machineID, osID string) bool {
	for _, c := range ds.Compat {
		ml := contains(c.Machines, machineID)
		ol := contains(c.OS, osID)
		if ml && ol {
			return true
		}
	}
	return false
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
