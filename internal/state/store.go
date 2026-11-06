package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

type Status string

const (
	StatusBuilding Status = "building"
	StatusRunning  Status = "running"
	StatusStopped  Status = "stopped"
	StatusFailed   Status = "failed"
	StatusRemoved  Status = "removed"
)

type Release struct {
	ID        int       `json:"id"`
	Image     string    `json:"image"`
	Commit    string    `json:"commit"`
	Container string    `json:"container"`
	HostPort  int       `json:"host_port"`
	Status    Status    `json:"status"`
	EnvData   string    `json:"env_file"`
	CreatedAt time.Time `json:"created_at"`
	Error     string    `json:"error,omitempty"`
}

type App struct {
	Name          string            `json:"name"`
	Repo          string            `json:"repo"`
	ContainerPort int               `json:"container_port"` // port the app listens on inside the container
	Env           map[string]string `json:"env"`
	Releases      []Release         `json:"releases"`
	CurrentID     int               `json:"current_id"` // 0 = nothing live
	NextID        int               `json:"next_id"`
}

type Data struct {
	Apps map[string]*App `json:"apps"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data Data
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.data, "", " ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) Update(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := fn(&s.data); err != nil {
		return err
	}
	return s.saveLocked()
}

func (s *Store) BeginRelease(name, repo string, containerPort int) (int, error) {
	var id int

	err := s.Update(func(d *Data) error {
		if d.Apps == nil {
			d.Apps = map[string]*App{}
		}
		app, ok := d.Apps[name]
		if !ok {
			app = &App{
				Name: name, Env: map[string]string{},
			}
		}
		app.Repo = repo
		app.ContainerPort = containerPort

		app.NextID++
		id = app.NextID

		app.Releases = append(app.Releases, Release{
			ID:        id,
			Image:     fmt.Sprintf("zeroku/%s:%d", name, id),
			Status:    StatusBuilding,
			CreatedAt: time.Now(),
		})
		return nil
	})
	return id, err
}

func (s *Store) UsedPorts() map[int]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	used := make(map[int]bool)
	for _, app := range s.data.Apps {
		for _, rel := range app.Releases {
			if rel.HostPort != 0 && (rel.Status == StatusRunning || rel.Status == StatusBuilding) {
				used[rel.HostPort] = true
			}
		}
	}
	return used
}

func NewStore(path string) (*Store, error) {
	s := &Store{path: path}

	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.data = Data{Apps: map[string]*App{}}
		return s, nil
	}

	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("state file is invalid: %w", err)
	}
	if s.data.Apps == nil {
		s.data.Apps = map[string]*App{}
	}
	return s, nil
}
