// metal-install server: HTTP API for per-node artifact generation.
//
// Endpoints:
//
//	POST   /nodes                     create a node, render artifacts
//	GET    /nodes                     list active node IDs
//	GET    /nodes/{node_id}           one node's spec
//	DELETE /nodes/{node_id}           remove a node from the registry
//	GET    /configs/{node_id}/{file...} serve a generated artifact
//	GET    /health                    readiness check
//
// The data directory is loaded once at startup; restart the server to
// pick up changes (matches the deploy model where releases are
// deployed as a new directory and symlink-switched).
//
// The state directory holds two things:
//
//	state/nodes/<node_id>.json      one file per active node
//	state/configs/<node_id>/        generated artifacts per node
//
// On DELETE the node JSON is removed and the node's artifacts are no
// longer served; the configs/ directory is kept for audit and
// debugging. Operators can rm -rf old configs later.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/zinrai/metal-install/internal/data"
	"github.com/zinrai/metal-install/internal/datadir"
	"github.com/zinrai/metal-install/internal/render"
)

func serverCmd(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	dataDir := fs.String("data-dir", ".", "path to data directory")
	stateDir := fs.String("state-dir", "./state", "path to state directory (nodes/, configs/)")
	listen := fs.String("listen", ":8080", "listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ds, err := datadir.Load(*dataDir)
	if err != nil {
		return fmt.Errorf("load data dir: %w", err)
	}
	log.Printf("loaded: %d machines, %d os, %d templates",
		len(ds.Machines), len(ds.OS), len(ds.Templates))

	if err := os.MkdirAll(filepath.Join(*stateDir, "nodes"), 0o755); err != nil {
		return fmt.Errorf("mkdir state/nodes: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(*stateDir, "configs"), 0o755); err != nil {
		return fmt.Errorf("mkdir state/configs: %w", err)
	}

	srv := &server{ds: ds, stateDir: *stateDir}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.handleHealth)
	mux.HandleFunc("POST /nodes", srv.handleCreateNode)
	mux.HandleFunc("GET /nodes", srv.handleListNodes)
	mux.HandleFunc("GET /nodes/{node_id}", srv.handleGetNode)
	mux.HandleFunc("DELETE /nodes/{node_id}", srv.handleDeleteNode)
	mux.HandleFunc("GET /configs/{node_id}/{file...}", srv.handleGetConfig)

	log.Printf("listening on %s", *listen)
	return http.ListenAndServe(*listen, mux)
}

type server struct {
	ds       *data.DataSet
	stateDir string
	mu       sync.Mutex
}

func validNodeID(id string) bool {
	return id != "" && !strings.Contains(id, "/") && !strings.Contains(id, "..")
}

func (s *server) nodePath(id string) string {
	return filepath.Join(s.stateDir, "nodes", id+".json")
}

func (s *server) registered(id string) bool {
	_, err := os.Stat(s.nodePath(id))
	return err == nil
}

// register writes the node's artifacts under state/configs/<node_id>/
// and records its spec in state/nodes/<node_id>.json.
func (s *server) register(spec *data.Spec, files map[string][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfgDir := filepath.Join(s.stateDir, "configs", spec.NodeID)
	if err := writeArtifacts(cfgDir, files); err != nil {
		return fmt.Errorf("write artifacts: %w", err)
	}
	nodeBytes, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(s.nodePath(spec.NodeID), nodeBytes, 0o644); err != nil {
		return fmt.Errorf("write node: %w", err)
	}
	return nil
}

// Not an error when the node is already gone: a retried removal
// should succeed.
func (s *server) deregister(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := os.Remove(s.nodePath(id))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

// POST /nodes
//
// Body: an InstallSpec as JSON. Renders and writes artifacts under
// state/configs/<node_id>/ and records the spec under
// state/nodes/<node_id>.json.
func (s *server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	var spec data.Spec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, "decode spec: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !validNodeID(spec.NodeID) {
		http.Error(w, "invalid node_id", http.StatusBadRequest)
		return
	}

	result, err := render.Render(render.Request{
		DataSet: s.ds,
		Spec:    &spec,
	})
	if err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.register(&spec, result.Files); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	files := make([]string, 0, len(result.Files))
	for name := range result.Files {
		files = append(files, name)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"node_id": spec.NodeID,
		"files":   files,
	})
}

// GET /nodes
func (s *server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(filepath.Join(s.stateDir, "nodes"))
	if err != nil {
		http.Error(w, "read nodes dir: "+err.Error(), http.StatusInternalServerError)
		return
	}
	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			ids = append(ids, strings.TrimSuffix(name, ".json"))
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"nodes": ids})
}

// GET /nodes/{node_id}
func (s *server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("node_id")
	if !validNodeID(id) {
		http.Error(w, "invalid node_id", http.StatusBadRequest)
		return
	}
	b, err := os.ReadFile(s.nodePath(id))
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		http.Error(w, "read: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

// DELETE /nodes/{node_id}
//
// Removes the node from the registry. metal-install does not track why
// a node is removed (completion, cancellation, and cleanup are all the
// same to it): that is the caller's concern. The configs/ directory is
// left in place, unserved, and can be rm -rf'd at the operator's
// discretion.
func (s *server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("node_id")
	if !validNodeID(id) {
		http.Error(w, "invalid node_id", http.StatusBadRequest)
		return
	}
	if err := s.deregister(id); err != nil {
		http.Error(w, "remove: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /configs/{node_id}/{file...}
//
// Serve a single generated artifact of a registered node. The file
// portion may contain subdirectory components (e.g. "post/udev.sh").
// Responds 404 for a node that is not registered. Rejects any segment
// that is empty, ".", or "..", and verifies that the resolved path
// stays inside the configs directory for the node.
func (s *server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("node_id")
	file := r.PathValue("file")
	if !validNodeID(id) {
		http.Error(w, "invalid node_id", http.StatusBadRequest)
		return
	}
	if !s.registered(id) {
		http.NotFound(w, r)
		return
	}
	for _, seg := range strings.Split(file, "/") {
		if seg == "" || seg == "." || seg == ".." {
			http.Error(w, "invalid file path", http.StatusBadRequest)
			return
		}
	}
	configRoot := filepath.Join(s.stateDir, "configs", id)
	fullpath := filepath.Join(configRoot, filepath.FromSlash(file))
	// Defense in depth: confirm the cleaned path is still inside
	// configRoot before serving.
	if !strings.HasPrefix(fullpath+string(filepath.Separator), configRoot+string(filepath.Separator)) {
		http.Error(w, "invalid file path", http.StatusBadRequest)
		return
	}
	http.ServeFile(w, r, fullpath)
}
