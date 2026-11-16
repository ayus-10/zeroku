package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ayus-10/zeroku/internal/config"
	"github.com/ayus-10/zeroku/internal/state"
	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	cfg     config.Config
	store   *state.Store
	router  Router
	proxyMu sync.Mutex
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

	NewServer := &Server{
		cfg:   cfg,
		store: store,
	}

	server := &http.Server{
		Addr:        fmt.Sprintf(":%d", NewServer.cfg.Port),
		Handler:     NewServer.RegisterRoutes(),
		IdleTimeout: time.Minute,
		ReadTimeout: 10 * time.Second,
		// no WriteTimeout to allow Streaming responses (builds, logs)
	}

	return server
}
