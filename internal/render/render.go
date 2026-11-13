// Package render produces installation artifacts for a single
// InstallSpec.
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

type Request struct {
	DataSet *data.DataSet
	Spec    *data.Spec
}

type Result struct {
	NodeID string
	Files  map[string][]byte
}

type templateContext struct {
	Machine *data.Machine
	OS      *data.OS
	Spec    *data.Spec
	Env     *data.Env
}

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

	bootOut, err := executeTemplate(ds.Templates, osDef.BootTemplate, ctx)
	if err != nil {
		return nil, fmt.Errorf("boot template: %w", err)
	}
	result.Files["boot.ipxe"] = bootOut

	for _, c := range osDef.Configs {
		cfgOut, err := executeTemplate(ds.Templates, c.Template, ctx)
		if err != nil {
			return nil, fmt.Errorf("config template %s: %w", c.Template, err)
		}
		result.Files[c.Filename] = cfgOut
	}

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
