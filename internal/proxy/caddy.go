package proxy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ayus-10/zeroku/internal/state"
)

type Proxy interface {
	Ensure(ctx context.Context, apps []state.App) error
	Sync(ctx context.Context, apps []state.App) error
}

type Caddy struct {
	opts      Options
	cfgPath   string
	container string
	mu        sync.Mutex
}

func NewCaddy(opts Options) *Caddy {
	return &Caddy{opts: opts, cfgPath: "/etc/caddy/Caddyfile", container: "caddy"}
}

func (p *Caddy) Sync(ctx context.Context, apps []state.App) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	newCfg := Render(p.opts, apps)
	old, readErr := os.ReadFile(p.cfgPath)

	if err := atomicWrite(p.cfgPath, []byte(newCfg)); err != nil {
		return err
	}
	out, err := exec.CommandContext(ctx, "docker", "exec", p.container,
		"caddy", "reload", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile").CombinedOutput()
	if err != nil {
		if readErr == nil {
			_ = atomicWrite(p.cfgPath, old)
		}
		return fmt.Errorf("caddy reload failed: %v: %s", err, out)
	}
	return nil
}

func (p *Caddy) Ensure(ctx context.Context, apps []state.App) error {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "-f", "{{.State.Running}}", p.container).Output()
	if err == nil {
		if strings.TrimSpace(string(out)) != "true" {
			if o, err := exec.CommandContext(ctx, "docker", "start", p.container).CombinedOutput(); err != nil {
				return fmt.Errorf("start caddy: %v: %s", err, o)
			}
		}
		return p.Sync(ctx, apps)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(p.cfgPath), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(p.cfgPath, []byte(Render(p.opts, apps))); err != nil {
		return err
	}
	o, err := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", p.container,
		"--restart", "unless-stopped",
		"--network", "host",
		"-v", "/etc/caddy:/etc/caddy",
		"-v", "caddy_data:/data",
		"-v", "caddy_config:/config",
		"caddy:2",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("run caddy: %v: %s", err, o)
	}
	return nil
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".Caddyfile.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
