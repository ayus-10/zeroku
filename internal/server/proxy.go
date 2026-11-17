package server

import (
	"context"

	"github.com/ayus-10/zeroku/internal/state"
)

type Router interface {
	Ensure(ctx context.Context, apps []state.App) error
	Sync(ctx context.Context, apps []state.App) error
}

type NoopRouter struct{}

func (NoopRouter) Ensure(ctx context.Context, apps []state.App) error {
	return nil
}

func (NoopRouter) Sync(ctx context.Context, apps []state.App) error {
	return nil
}

func Routes(apps []state.App) []state.App {
	out := make([]state.App, 0, len(apps))
	for _, a := range apps {
		out = append(out, a)
	}
	return out
}

func (s *Server) syncProxy(ctx context.Context) error {
	s.proxyMu.Lock()
	defer s.proxyMu.Unlock()
	return s.router.Sync(ctx, Routes(s.store.List()))
}
