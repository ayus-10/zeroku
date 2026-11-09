package server

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ayus-10/zeroku/internal/config"
	"github.com/ayus-10/zeroku/internal/state"
	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port  int
	store *state.Store
}

func NewServer() *http.Server {
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		log.Fatalf("invalid port: %v", err)
	}

	stateDir := os.Getenv("STATE_DIR")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		log.Fatalf("failed to create state directory: %v", err)
	}

	authToken := config.AuthToken()
	if len(authToken) < 32 {
		log.Fatalf("AUTH_TOKEN must be at least 32 characters")
	}

	store, err := state.NewStore(filepath.Join(stateDir, "state.json"), authToken)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}

	NewServer := &Server{
		port:  port,
		store: store,
	}

	server := &http.Server{
		Addr:        fmt.Sprintf(":%d", NewServer.port),
		Handler:     NewServer.RegisterRoutes(),
		IdleTimeout: time.Minute,
		ReadTimeout: 10 * time.Second,
		// no WriteTimeout to allow Streaming responses (builds, logs)
	}

	return server
}
