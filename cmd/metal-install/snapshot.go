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
		printWritten(target, result.Files)
	}
	return nil
}

type pair struct {
	machine string
	os      string
}

func allowedPairs(ds *data.DataSet) []pair {
	seen := make(map[pair]bool)
	for _, c := range ds.Compat {
		for _, p := range pairsOf(c) {
			seen[p] = true
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

func pairsOf(c data.CompatEntry) []pair {
	var pairs []pair
	for _, m := range c.Machines {
		for _, o := range c.OS {
			pairs = append(pairs, pair{m, o})
		}
	}
	return pairs
}

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
