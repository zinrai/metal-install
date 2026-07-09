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
// On DELETE the node JSON is removed; the configs/ directory is kept
// for audit and debugging. Operators can rm -rf old configs later.
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

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "ok")
}

// POST /nodes
//
// Body: an InstallSpec as JSON. Renders and writes artifacts under
// state/configs/<node_id>/ and records the spec under
// state/nodes/<node_id>.json.
func (s *server) handleCreateNode(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var spec data.Spec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		http.Error(w, "decode spec: "+err.Error(), http.StatusBadRequest)
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

	// Write generated artifacts to state/configs/<node_id>/. Files
	// may contain subdirectory components (e.g. "post/udev.sh"); the
	// parent directory is created as needed.
	cfgDir := filepath.Join(s.stateDir, "configs", spec.NodeID)
	for name, content := range result.Files {
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		fullpath := filepath.Join(cfgDir, name)
		if err := os.MkdirAll(filepath.Dir(fullpath), 0o755); err != nil {
			http.Error(w, "mkdir: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(fullpath, content, mode); err != nil {
			http.Error(w, "write artifact: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Record the node in state/nodes/<node_id>.json.
	nodePath := filepath.Join(s.stateDir, "nodes", spec.NodeID+".json")
	nodeBytes, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		http.Error(w, "marshal: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(nodePath, nodeBytes, 0o644); err != nil {
		http.Error(w, "write node: "+err.Error(), http.StatusInternalServerError)
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
	if id == "" {
		http.Error(w, "node_id required", http.StatusBadRequest)
		return
	}
	if strings.Contains(id, "/") || strings.Contains(id, "..") {
		http.Error(w, "invalid node_id", http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.stateDir, "nodes", id+".json")
	b, err := os.ReadFile(path)
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
// left in place and can be rm -rf'd at the operator's discretion.
func (s *server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := r.PathValue("node_id")
	if id == "" {
		http.Error(w, "node_id required", http.StatusBadRequest)
		return
	}
	if strings.Contains(id, "/") || strings.Contains(id, "..") {
		http.Error(w, "invalid node_id", http.StatusBadRequest)
		return
	}
	path := filepath.Join(s.stateDir, "nodes", id+".json")
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			// Tolerate repeat DELETEs: a retried removal should not
			// see an error.
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "remove: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /configs/{node_id}/{file...}
//
// Serve a single generated artifact. The file portion may contain
// subdirectory components (e.g. "post/udev.sh"). Rejects any segment
// that is empty, ".", or "..", and verifies that the resolved path
// stays inside the configs directory for the node.
func (s *server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("node_id")
	file := r.PathValue("file")
	if id == "" || file == "" {
		http.Error(w, "node_id and file required", http.StatusBadRequest)
		return
	}
	if strings.Contains(id, "/") || strings.Contains(id, "..") {
		http.Error(w, "invalid node_id", http.StatusBadRequest)
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
