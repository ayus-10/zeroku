package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	mux.HandleFunc("POST /v1/apps/{name}/deploy", s.DeployHandler)
	mux.HandleFunc("POST /v1/apps/{name}/stop", s.StopAppHandler)
	mux.HandleFunc("GET /v1/apps/{name}", s.GetAppHandler)
	mux.HandleFunc("GET /v1/apps", s.ListAppsHandler)

	return s.corsMiddleware(s.authMiddleware(mux))
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
		w.Header().Set("Access-Control-Allow-Credentials", "false")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(got), []byte(s.store.AuthToken())) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
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
	defer os.RemoveAll(tempDir)
	if err != nil {
		fw.Printf("ERROR: create temp dir: %v\n", err)
		return
	}

	commitID, err := deployer.Clone(ctx, fw, req.Repo, req.GitRef, tempDir)
	if err != nil {
		fw.Printf("ERROR: clone: %v\n", err)
		return
	}

	fw.Printf("===> building (commit %s)\n", commitID)

	if err := s.store.UpsertApp(appName, req.Repo, req.ContainerPort, req.Env); err != nil {
		fw.Printf("ERROR: save app: %v\n", err)
		return
	}

	rel, err := s.store.BeginRelease(appName, commitID)
	if err != nil {
		fw.Printf("ERROR: begin release: %v\n", err)
		return
	}

	var started bool
	fail := func(msg string, err error) {
		fw.Printf("ERROR: %s: %v\n", msg, err)
		cctx := context.WithoutCancel(ctx) // cleanup must survive a cancelled request
		if started {
			_ = deployer.StopContainer(cctx, fw, rel.Container)
		}
		_ = s.store.UpdateRelease(appName, rel.ID, func(r *state.Release) {
			r.Status = state.StatusFailed
			r.Error = err.Error()
		})
	}

	if err := deployer.Build(ctx, fw, tempDir, rel.Image); err != nil {
		fail("build", err)
		return
	}

	fw.Printf("===> build complete: %s\n", rel.Image)

	hostPort, err := deployer.AllocPort(s.store.UsedPorts())
	if err != nil {
		fail("allocate port", err)
		return
	}

	envFile := fmt.Sprintf("%s/env/%s", os.Getenv("STATE_DIR"), rel.Container)
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		fail("create env dir", err)
		return
	}

	if err := deployer.WriteEnvFile(envFile, req.ContainerPort, req.Env); err != nil {
		fail("write env file", err)
		return
	}

	fw.Printf("===> deploying %s on port %d\n", rel.Container, hostPort)

	started = true
	if err := deployer.RunContainer(ctx, fw, rel.Container, rel.Image, envFile, hostPort, req.ContainerPort); err != nil {
		fail("run container", err)
		return
	}

	if err := s.store.UpdateRelease(appName, rel.ID, func(r *state.Release) {
		r.HostPort = hostPort
		r.EnvFile = envFile
		r.Status = state.StatusRunning
	}); err != nil {
		fail("update release", err)
		return
	}

	fw.Printf("===> waiting for health check\n")

	if err := deployer.WaitHealthy(ctx, hostPort, "/", 30*time.Second); err != nil {
		deployer.StreamLogs(ctx, fw, rel.Container, 30)
		fail("health check", err)
		return
	}

	prev, err := s.store.SetCurrent(appName, rel.ID)
	if err != nil {
		fail("set current", err)
		return
	}
	if prev != nil {
		fw.Printf("===> stopping previous release %d\n", prev.ID)
		if err := deployer.StopContainer(ctx, fw, prev.Container); err != nil {
			fw.Printf("WARN: stop previous container: %v\n", err)
		}
		if err := s.store.StopRelease(appName, prev.ID); err != nil {
			fw.Printf("WARN: record stopped release: %v\n", err)
		}
	}

	fw.Printf("URL: http://localhost:%d\n", hostPort)
}

func (s *Server) ListAppsHandler(w http.ResponseWriter, r *http.Request) {
	apps := s.store.ListApps()

	w.Header().Set("Content-Type", "application/json")

	if err := json.NewEncoder(w).Encode(apps); err != nil {
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
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	fw.Printf("===> stopping %s (release %d)\n", appName, rel.ID)

	if err := deployer.StopContainer(ctx, fw, rel.Container); err != nil {
		fw.Printf("ERROR: stop container: %v\n", err)
	}

	if err := s.store.StopRelease(appName, rel.ID); err != nil {
		fw.Printf("ERROR: stop release: %v\n", err)
	}

	fw.Printf("===> stopped\n")
}
