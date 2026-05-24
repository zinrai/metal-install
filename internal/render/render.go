// Package render produces installation artifacts for a single
// InstallSpec.
//
// This package is the shared core called by both metal-install-server
// (over HTTP) and metal-install-render (over CLI). It does no I/O
// beyond reading the in-memory DataSet; the caller is responsible
// for writing the returned bytes to disk or to an HTTP response.
//
// The intent is that the same input (DataSet + Spec) always produces
// the same output regardless of the calling context. This is what
// makes "what the developer sees locally" identical to "what the
// server generates in production".
package render

import (
	"bytes"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/zinrai/metal-install/internal/data"
	"github.com/zinrai/metal-install/internal/datadir"
)

// Request is the input to Render.
type Request struct {
	DataSet *data.DataSet
	Spec    *data.Spec
}

// Result is the output of Render.
//
// Files is filename -> content. Filenames are bare (no directory
// component), e.g. "boot.ipxe", "kickstart.ks", "setup.sh". The
// caller decides where to put them (e.g. configs/<node_id>/...).
type Result struct {
	NodeID string
	Files  map[string][]byte
}

// templateContext is what gets passed to template.Execute.
// Templates reference fields as .Machine.NICs, .OS.Packages,
// .Spec.NodeID, .Env.InstallServer.HTTPBase, etc.
type templateContext struct {
	Machine *data.Machine
	OS      *data.OS
	Spec    *data.Spec
	Env     *data.Env
}

// Render produces the artifacts for one InstallSpec.
//
// Errors are returned for:
//   - unknown machine ID or OS ID
//   - machine+os not declared in compatibility.yml
//   - template parse or execute errors
func Render(req Request) (*Result, error) {
	if req.DataSet == nil {
		return nil, fmt.Errorf("render: nil DataSet")
	}
	if req.Spec == nil {
		return nil, fmt.Errorf("render: nil Spec")
	}
	ds := req.DataSet
	spec := req.Spec

	machine, ok := ds.Machines[spec.Machine]
	if !ok {
		return nil, fmt.Errorf("unknown machine: %s", spec.Machine)
	}
	osDef, ok := ds.OS[spec.OS]
	if !ok {
		return nil, fmt.Errorf("unknown os: %s", spec.OS)
	}
	if !datadir.IsCompatible(ds, spec.Machine, spec.OS) {
		return nil, fmt.Errorf("incompatible combination: machine=%s os=%s",
			spec.Machine, spec.OS)
	}

	ctx := templateContext{
		Machine: machine,
		OS:      osDef,
		Spec:    spec,
		Env:     ds.Env,
	}

	result := &Result{
		NodeID: spec.NodeID,
		Files:  make(map[string][]byte),
	}

	// 1. boot.ipxe (always produced)
	bootOut, err := executeTemplate(ds.Templates, osDef.BootTemplate, ctx)
	if err != nil {
		return nil, fmt.Errorf("boot template: %w", err)
	}
	result.Files["boot.ipxe"] = bootOut

	// 2. installer configuration files.
	// One OS may need multiple: RHEL has kickstart.ks, Debian has
	// preseed.cfg, Ubuntu autoinstall needs user-data and meta-data.
	for _, c := range osDef.Configs {
		cfgOut, err := executeTemplate(ds.Templates, c.Template, ctx)
		if err != nil {
			return nil, fmt.Errorf("config template %s: %w", c.Template, err)
		}
		result.Files[c.Filename] = cfgOut
	}

	// 3. post-install scripts.
	// Each script is rendered to `post/<basename>` (basename = the
	// template filename minus the ".tmpl" suffix). The installer's
	// post-install hook fetches and runs them individually.
	for _, t := range osDef.Setup.Post {
		out, err := executeTemplate(ds.Templates, t, ctx)
		if err != nil {
			return nil, fmt.Errorf("setup.post template %s: %w", t, err)
		}
		base := strings.TrimSuffix(path.Base(t), ".tmpl")
		result.Files[path.Join("post", base)] = out
	}

	return result, nil
}

func executeTemplate(templates map[string]string, name string, ctx templateContext) ([]byte, error) {
	src, ok := templates[name]
	if !ok {
		return nil, fmt.Errorf("template not loaded: %s", name)
	}
	t, err := template.New(name).Funcs(templateFuncs()).Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("execute %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// templateFuncs returns the function map exposed to all templates.
//
// indent prefixes every line of s with n spaces. Used to insert a
// block of YAML written at the top level of a machine yml into a
// position deeper in a generated autoinstall user-data.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"indent": func(n int, s string) string {
			if s == "" {
				return s
			}
			prefix := strings.Repeat(" ", n)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line == "" {
					continue
				}
				lines[i] = prefix + line
			}
			return strings.Join(lines, "\n")
		},
	}
}
