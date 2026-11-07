package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/ayus-10/zeroku/internal/api"
	"github.com/ayus-10/zeroku/internal/deployer"
	"github.com/ayus-10/zeroku/internal/state"
)

type flushWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("POST /v1/apps/{name}/deploy", s.DeployHandler)
	mux.HandleFunc("POST /v1/apps/{name}/stop", s.StopAppHandler)
	mux.HandleFunc("GET /v1/apps/{name}", s.GetAppHandler)
	mux.HandleFunc("GET /v1/apps", s.ListAppsHandler)
	// mux.HandleFunc("POST /v1/apps/{name}/stop", s.StopAppHandler)

	// Wrap the mux with CORS middleware
	return s.corsMiddleware(mux)
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // Replace "*" with specific origins if needed
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "false") // Set to "true" if credentials are required

		// Handle preflight OPTIONS requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Proceed with the next handler
		next.ServeHTTP(w, r)
	})
}

func parseDeployRequest(r *http.Request) (api.DeployRequest, error) {
	var req api.DeployRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return req, errors.New("invalid JSON body")
	}

	if req, err := validateDeployRequestPayload(req); err != nil {
		return req, err
	}

	return req, nil
}

func newFlushWriter(w http.ResponseWriter) *flushWriter {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	return &flushWriter{
		w:  w,
		rc: http.NewResponseController(w),
	}
}

func (f *flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	_ = f.rc.Flush()
	return n, err
}

func (f *flushWriter) Printf(format string, args ...any) {
	fmt.Fprintf(f, format, args...)
}

func (s *Server) DeployHandler(w http.ResponseWriter, r *http.Request) {
	req, err := parseDeployRequest(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	appName := r.PathValue("name")
	if err := validateAppName(appName); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fw := newFlushWriter(w)
	ctx := r.Context()

	fw.Printf("===> cloning %s\n", req.Repo)

	tempDir, err := os.MkdirTemp("", "zeroku-")
	if err != nil {
		fw.Printf("ERROR: create temp dir: %v\n", err)
		return
	}

	commitID, err := deployer.Clone(ctx, fw, req.Repo, req.GitRef, tempDir)
	if err != nil {
		fw.Printf("ERROR: clone: %v\n", err)
		return
	}
	defer os.RemoveAll(tempDir)

	fw.Printf("===> building (commit %s)\n", commitID)

	releaseID, err := s.store.BeginRelease(appName, req.Repo, req.ContainerPort)
	if err != nil {
		fw.Printf("ERROR: begin release: %v\n", err)
		return
	}

	image := fmt.Sprintf("zeroku/%s:%d-%s", appName, releaseID, commitID)

	if err := deployer.Build(ctx, fw, tempDir, image); err != nil {
		// TODO: save build error to store
		fw.Printf("ERROR: build: %v\n", err)
		return
	}

	fw.Printf("===> build complete: %s\n", image)

	hostPort, err := deployer.AllocPort(s.store.UsedPorts())
	if err != nil {
		fw.Printf("ERROR: alloc port: %v\n", err)
		return
	}

	containerName := fmt.Sprintf("%s-%d", appName, releaseID)

	envFile := fmt.Sprintf("%s/env/%s", os.Getenv("STATE_DIR"), containerName)
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		fw.Printf("ERROR: create env dir: %v\n", err)
		return
	}

	if err := deployer.WriteEnvFile(envFile, req.ContainerPort, req.Env); err != nil {
		fw.Printf("ERROR: write env file: %v\n", err)
		return
	}

	fw.Printf("===> deploying %s on port %d\n", containerName, hostPort)

	if err := deployer.RunContainer(ctx, fw, containerName, image, envFile, hostPort, req.ContainerPort); err != nil {
		fw.Printf("ERROR: run container: %v\n", err)
		return
	}

	fw.Printf("===> waiting for health check\n")

	if err := deployer.WaitHealthy(ctx, hostPort, "/", 30*time.Second); err != nil {
		deployer.StreamLogs(ctx, fw, containerName, 30)
		fw.Printf("ERROR: health check: %v\n", err)
		return
	}

	fw.Printf("URL: http://localhost:%d\n", hostPort)

}

func (s *Server) ListAppsHandler(w http.ResponseWriter, r *http.Request) {
	apps := s.store.ListApps()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(map[string][]state.AppSummary{
		"apps": apps,
	}); err != nil {
		log.Printf("encode apps response: %v", err)
	}
}

func (s *Server) GetAppHandler(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")
	app := s.store.GetApp(appName)
	if app == nil {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(app); err != nil {
		log.Printf("encode app response: %v", err)
	}
}

func (s *Server) StopAppHandler(w http.ResponseWriter, r *http.Request) {
	appName := r.PathValue("name")

	fw := newFlushWriter(w)
	ctx := context.Background()

	rel, err := s.store.CurrentRelease(appName)
	if err != nil {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}

	fw.Printf("===> stopping %s (release %d)\n", appName, rel.ID)

	if err := deployer.StopContainer(ctx, fw, appName); err != nil {
		fw.Printf("ERROR: stop container: %v\n", err)
	}

	if err := s.store.StopRelease(appName, rel.ID); err != nil {
		fw.Printf("ERROR: stop release: %v\n", err)
	}

	fw.Printf("===> stopped\n")
}
