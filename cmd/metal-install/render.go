// metal-install render: generate per-node artifacts for one spec.
//
// The output is identical to what `metal-install server` would
// generate for the same spec; both call the same render package.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/zinrai/metal-install/internal/data"
	"github.com/zinrai/metal-install/internal/datadir"
	"github.com/zinrai/metal-install/internal/render"
)

func renderCmd(args []string) error {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	specPath := fs.String("spec", "", "path to InstallSpec YAML")
	dataDir := fs.String("data-dir", ".", "path to data directory (containing machines/, os/, etc.)")
	outputDir := fs.String("output-dir", "", "directory to write artifacts into (created if missing); required")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *specPath == "" {
		return fmt.Errorf("-spec is required")
	}
	if *outputDir == "" {
		return fmt.Errorf("-output-dir is required")
	}

	specBytes, err := os.ReadFile(*specPath)
	if err != nil {
		return fmt.Errorf("read spec: %w", err)
	}
	var spec data.Spec
	if err := yaml.Unmarshal(specBytes, &spec); err != nil {
		return fmt.Errorf("parse spec: %w", err)
	}

	ds, err := datadir.Load(*dataDir)
	if err != nil {
		return fmt.Errorf("load data dir: %w", err)
	}

	result, err := render.Render(render.Request{
		DataSet: ds,
		Spec:    &spec,
	})
	if err != nil {
		return err
	}

	return writeArtifacts(filepath.Join(*outputDir, result.NodeID), result.Files)
}

// writeArtifacts writes a Files map (filename -> bytes) under root.
// Filenames may contain subdirectory components (e.g. "post/udev.sh");
// parent directories are created as needed. Files with a .sh suffix
// get the executable bit set.
func writeArtifacts(root string, files map[string][]byte) error {
	for name, content := range files {
		fullpath := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(fullpath), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(fullpath), err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(fullpath, content, mode); err != nil {
			return fmt.Errorf("write %s: %w", fullpath, err)
		}
		fmt.Printf("Wrote: %s\n", fullpath)
	}
	return nil
}
