package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ayus-10/zeroku/internal/config"
	"github.com/ayus-10/zeroku/internal/proxy"
	"github.com/ayus-10/zeroku/internal/state"
	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	cfg     config.Config
	store   *state.Store
	router  Router
	proxyMu sync.Mutex
}

func newRouter(cfg config.Config) (Router, error) {
	if !cfg.Production() {
		return NoopRouter{}, nil
	}
	opts := proxy.Options{
		BaseDomain:  cfg.BaseDomain,
		ControlAddr: cfg.Addr,
		ACMEStaging: cfg.ACMEStaging,
	}
	return proxy.NewCaddy(opts), nil
}

func NewServer() *http.Server {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
		log.Fatalf("failed to create state directory: %v", err)
	}

	store, err := state.NewStore(filepath.Join(cfg.StateDir, "state.json"), cfg.Token)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}

	router, err := newRouter(cfg)
	if err != nil {
		log.Fatalf("failed to configure proxy: %v", err)
	}

	s := &Server{
		cfg:    cfg,
		store:  store,
		router: router,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := router.Ensure(ctx, Routes(store.List())); err != nil {
		log.Fatalf("failed to start proxy: %v", err)
	}

	server := &http.Server{
		Addr:        cfg.ListenAddr(),
		Handler:     s.RegisterRoutes(),
		IdleTimeout: time.Minute,
		ReadTimeout: 10 * time.Second,
		// no WriteTimeout to allow Streaming responses (builds, logs)
	}

	return server
}
