package server

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/ayus-10/zeroku/internal/api"
)

var (
	appNamePattern  = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
	repoPathPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)+$`)
	gitRefPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	envKeyPattern   = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

const (
	maxEnvKeyLength   = 63
	maxEnvValueLength = 255
)

var allowedRepoHosts = map[string]bool{
	"github.com": true,
	"gitlab.com": true,
}

func validateAppName(name string) error {
	if name == "" {
		return errors.New("app name is required")
	}
	if len(name) > 63 {
		return fmt.Errorf("app name must be 63 characters or fewer, got %d", len(name))
	}
	if !appNamePattern.MatchString(name) {
		return errors.New("app name may contain letters, digits, and hyphens, but must not start or end with a hyphen")
	}
	return nil
}

func validateRepoURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return errors.New("repo must be an https git URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("repo URL must not contain credentials, query, or fragment")
	}
	if !allowedRepoHosts[strings.ToLower(u.Hostname())] {
		return errors.New("repo host is not allowed")
	}

	path := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
	if !repoPathPattern.MatchString(path) || strings.Contains(path, "..") {
		return errors.New("repo must point to a repository")
	}
	return nil
}

func validateDeployRequestPayload(req api.DeployRequest) (api.DeployRequest, error) {
	req.Repo = strings.TrimSpace(req.Repo)
	if req.Repo == "" {
		return req, errors.New("repo is required")
	}
	if err := validateRepoURL(req.Repo); err != nil {
		return req, err
	}

	req.GitRef = strings.TrimSpace(req.GitRef)
	if req.GitRef == "" {
		req.GitRef = "main"
	}
	if !gitRefPattern.MatchString(req.GitRef) ||
		strings.Contains(req.GitRef, "..") ||
		strings.HasSuffix(req.GitRef, "/") ||
		strings.HasSuffix(req.GitRef, ".lock") {
		return req, errors.New("git_ref contains invalid characters")
	}

	if req.ContainerPort < 1 || req.ContainerPort > 65535 {
		return req, errors.New("container_port must be between 1 and 65535")
	}

	if req.Env != nil {
		if err := validateEnv(req.Env); err != nil {
			return req, errors.New("env contains invalid characters")
		}
	}

	return req, nil
}

func validateEnv(env map[string]string) error {
	for key, value := range env {
		if key == "" {
			return errors.New("env key cannot be empty")
		}

		if len(key) > maxEnvKeyLength {
			return fmt.Errorf("env key %q exceeds %d characters",
				key, maxEnvKeyLength)
		}

		if !envKeyPattern.MatchString(key) {
			return fmt.Errorf("env key %q contains invalid characters", key)
		}

		if value == "" {
			return fmt.Errorf("env value for key %q cannot be empty", key)
		}

		if len(value) > maxEnvValueLength {
			return fmt.Errorf("env value for key %q exceeds %d bytes",
				key, maxEnvValueLength)
		}

		if strings.TrimSpace(value) != value {
			return fmt.Errorf("env value for key %q has leading or trailing whitespace", key)
		}

		for _, r := range value {
			if unicode.IsControl(r) {
				return fmt.Errorf("env value for key %q contains control characters", key)
			}
		}
	}

	return nil
}
