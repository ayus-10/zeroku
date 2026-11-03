package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"zeroku/internal/deployer"
)

type DeployRequest struct {
	Repo          string `json:"repo"`
	GitRef        string `json:"git_ref"`
	ContainerPort int    `json:"container_port"`
}

type flushWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func (s *Server) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	// Register routes
	mux.HandleFunc("/", s.HelloWorldHandler)
	mux.HandleFunc("POST /v1/apps/{name}/deploy", s.DeployHandler)

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

func parseDeployRequest(w http.ResponseWriter, r *http.Request) (DeployRequest, bool) {
	var req DeployRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return req, false
	}

	if req.Repo == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return req, false
	}
	if req.GitRef == "" {
		req.GitRef = "main"
	}
	if req.ContainerPort < 1 || req.ContainerPort > 65535 {
		http.Error(w, "container_port must be between 1 and 65535", http.StatusBadRequest)
		return req, false
	}

	return req, true
}

func newFlushWriter(w http.ResponseWriter) *flushWriter {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
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

func (s *Server) DeployHandler(w http.ResponseWriter, r *http.Request) {
	req, ok := parseDeployRequest(w, r)
	if !ok {
		return
	}

	ctx := r.Context()
	flushWriter := newFlushWriter(w)

	tempDir, err := os.MkdirTemp("", "zeroku-")
	if err != nil {
		http.Error(w, "Failed to create temp dir: "+err.Error(), http.StatusInternalServerError)
		return
	}

	commitID, err := deployer.Clone(ctx, flushWriter, req.Repo, req.GitRef, tempDir)
	if err != nil {
		http.Error(w, "Failed to clone: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	appName := r.PathValue("name")

	image := fmt.Sprintf("zeroku/%s:%d-%s", appName, 1, commitID) // TODO: replace with releaseID

	if err := deployer.Build(ctx, flushWriter, tempDir, image); err != nil {
		http.Error(w, "Failed to build: "+err.Error(), http.StatusInternalServerError)
		return
	}

}

func (s *Server) HelloWorldHandler(w http.ResponseWriter, r *http.Request) {
	resp := map[string]string{"message": "Hello World"}
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "Failed to marshal response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonResp); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
