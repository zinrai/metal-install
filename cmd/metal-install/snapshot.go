// metal-install snapshot: generate artifacts for every machine x OS
// combination declared in compatibility.yml, into a directory tree
// suitable for committing to a repository.
//
// The intended workflow is:
//
//  1. edit a template, an OS YAML, or a machine YAML
//  2. re-run `metal-install snapshot`
//  3. inspect `git diff` of the output directory to see the impact
//     of the change on every supported (machine, os) combination
//
// Output layout:
//
//	<output-dir>/<machine_id>/<os_id>/
//	    boot.ipxe
//	    <installer config files>
//	    post/<scripts>.sh
//
// All artifacts use a fixed sample InstallSpec (sample IP addresses,
// a placeholder password hash, a placeholder SSH key, and a fixed
// sample MAC as node_id). The values are dummies; the point of the
// output is to show *template rendering* differences across machine
// and OS combinations, not to produce installable artifacts.
package main

import (
	"flag"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/zinrai/metal-install/internal/data"
	"github.com/zinrai/metal-install/internal/datadir"
	"github.com/zinrai/metal-install/internal/render"
)

func snapshotCmd(args []string) error {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".", "path to data directory (containing compatibility.yml, etc.)")
	outputDir := fs.String("output-dir", "", "directory to write the snapshot tree into; required")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *outputDir == "" {
		return fmt.Errorf("-output-dir is required")
	}

	ds, err := datadir.Load(*dataDir)
	if err != nil {
		return fmt.Errorf("load data dir: %w", err)
	}

	pairs := allowedPairs(ds)

	for _, p := range pairs {
		spec := sampleSpec(p.machine, p.os)
		result, err := render.Render(render.Request{
			DataSet: ds,
			Spec:    spec,
		})
		if err != nil {
			return fmt.Errorf("render %s/%s: %w", p.machine, p.os, err)
		}
		target := filepath.Join(*outputDir, p.machine, p.os)
		if err := writeArtifacts(target, result.Files); err != nil {
			return fmt.Errorf("write %s/%s: %w", p.machine, p.os, err)
		}
	}
	return nil
}

// pair is one (machine, os) combination from compatibility.yml.
type pair struct {
	machine string
	os      string
}

// allowedPairs returns every (machine, os) combination declared in
// compatibility.yml, deduplicated and sorted for deterministic output.
func allowedPairs(ds *data.DataSet) []pair {
	seen := make(map[pair]bool)
	for _, c := range ds.Compat {
		for _, m := range c.Machines {
			for _, o := range c.OS {
				seen[pair{m, o}] = true
			}
		}
	}
	pairs := make([]pair, 0, len(seen))
	for p := range seen {
		pairs = append(pairs, p)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].machine != pairs[j].machine {
			return pairs[i].machine < pairs[j].machine
		}
		return pairs[i].os < pairs[j].os
	})
	return pairs
}

// sampleSpec returns a fixed-value InstallSpec used by snapshot. The
// values are not specific to any real deployment: IP addresses come
// from RFC 5737 (TEST-NET-1), the password hash is a placeholder, and
// the SSH key is obviously dummy. node_id is a fixed sample MAC so
// rendered files differ across combinations only because of machine
// and OS data, not because the node_id changes.
func sampleSpec(machine, os string) *data.Spec {
	return &data.Spec{
		Machine:          machine,
		OS:               os,
		NodeID:           "02-00-00-00-00-00",
		IPv4Addr:         "192.0.2.99",
		PrefixLength:     24,
		Gateway:          "192.0.2.1",
		DNS:              "192.0.2.53",
		RootPasswordHash: "$6$rounds=4096$snapshot$dummy",
		SSHKeys: []string{
			"ssh-ed25519 AAAAAA snapshot@example",
		},
	}
}
