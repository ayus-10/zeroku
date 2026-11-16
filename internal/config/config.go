package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Host        string
	Port        int
	Addr        string
	StateDir    string
	Token       string
	BaseDomain  string
	Env         string
	Scheme      string
	ACMEStaging bool
}

func (c Config) BaseURL() string {
	return (&url.URL{
		Scheme: c.Scheme,
		Host:   c.Addr,
	}).String()
}

func (c Config) ServiceURL(path string) string {
	u, _ := url.Parse(c.BaseURL())
	return u.JoinPath(path).String()
}

func LoadConfig() (Config, error) {
	host := getEnv("HOST", "127.0.0.1")

	port, err := parsePort(getEnv("PORT", "8080"))
	if err != nil {
		return Config{}, err
	}

	stateDir, err := validateStateDir(
		getEnv("STATE_DIR", "/var/lib/zeroku"),
	)
	if err != nil {
		return Config{}, err
	}

	env, err := validateEnv(getEnv("APP_ENV", "local"))
	if err != nil {
		return Config{}, err
	}

	scheme, err := validateScheme(getEnv("APP_SCHEME", "http"))
	if err != nil {
		return Config{}, err
	}

	baseDomain, err := validateDomain(
		getEnv("BASE_DOMAIN", "1-2-3-4.sslip.io"),
	)
	if err != nil {
		return Config{}, err
	}

	acmeStaging, err := strconv.ParseBool(
		getEnv("ACME_STAGING", "false"),
	)
	if err != nil {
		return Config{}, fmt.Errorf(
			"invalid ACME_STAGING: %w", err,
		)
	}

	if err := validateHost(host); err != nil {
		return Config{}, err
	}

	token := getEnv("AUTH_TOKEN", "")

	if env == "production" && strings.TrimSpace(token) == "" {
		return Config{}, fmt.Errorf(
			"AUTH_TOKEN is required in production",
		)
	}

	if strings.ContainsAny(token, "\r\n\x00") {
		return Config{}, fmt.Errorf(
			"AUTH_TOKEN must not contain newlines or null bytes",
		)
	}

	if env == "production" && !filepath.IsAbs(stateDir) {
		return Config{}, fmt.Errorf(
			"STATE_DIR must be an absolute path in production",
		)
	}

	return Config{
		Host:        host,
		Port:        port,
		Addr:        net.JoinHostPort(host, strconv.Itoa(port)),
		StateDir:    stateDir,
		Token:       token,
		BaseDomain:  baseDomain,
		Env:         env,
		Scheme:      scheme,
		ACMEStaging: acmeStaging,
	}, nil
}

func parsePort(value string) (int, error) {
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf(
			"invalid PORT %q: must be an integer", value,
		)
	}

	if port < 1 || port > 65535 {
		return 0, fmt.Errorf(
			"invalid PORT %d: must be between 1 and 65535", port,
		)
	}

	return port, nil
}

func validateHost(host string) error {
	if host == "" || strings.TrimSpace(host) != host {
		return fmt.Errorf("invalid HOST: must not be empty or contain surrounding whitespace")
	}

	if strings.ContainsAny(host, " /\\?#") ||
		strings.ContainsAny(host, "[]") {
		return fmt.Errorf("invalid HOST %q", host)
	}

	// Accept IPv4 and IPv6 addresses.
	if net.ParseIP(host) != nil {
		return nil
	}

	// Accept IPv6 addresses with zone identifiers.
	if i := strings.LastIndex(host, "%"); i > 0 {
		ip, zone := host[:i], host[i+1:]
		if net.ParseIP(ip) != nil && zone != "" &&
			!strings.ContainsAny(zone, " /\\?#[]%") {
			return nil
		}
	}

	// Otherwise, accept a hostname.
	if !validDNSName(host, false) {
		return fmt.Errorf("invalid HOST %q: must be an IP address or valid hostname", host)
	}

	return nil
}

func validateDomain(domain string) (string, error) {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))

	if !validDNSName(domain, true) {
		return "", fmt.Errorf(
			"invalid BASE_DOMAIN %q: must be a valid fully qualified domain name",
			domain,
		)
	}

	return domain, nil
}

func validDNSName(name string, requireFQDN bool) bool {
	if name == "" || len(name) > 253 {
		return false
	}

	labels := strings.Split(name, ".")

	if requireFQDN && len(labels) < 2 {
		return false
	}

	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return false
		}

		if label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, r := range label {
			if !((r >= 'a' && r <= 'z') ||
				(r >= '0' && r <= '9') ||
				r == '-') {
				return false
			}
		}
	}

	return true
}

func validateScheme(scheme string) (string, error) {
	scheme = strings.ToLower(scheme)

	switch scheme {
	case "http", "https":
		return scheme, nil
	default:
		return "", fmt.Errorf(
			"invalid APP_SCHEME %q: must be http or https", scheme,
		)
	}
}

func validateEnv(env string) (string, error) {
	env = strings.ToLower(env)

	switch env {
	case "local", "development", "test", "staging", "production":
		return env, nil
	default:
		return "", fmt.Errorf(
			"invalid APP_ENV %q: must be local, development, test, staging, or production",
			env,
		)
	}
}

func validateStateDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("STATE_DIR must not be empty")
	}

	if strings.ContainsRune(dir, '\x00') {
		return "", fmt.Errorf("STATE_DIR must not contain null bytes")
	}

	return filepath.Clean(dir), nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
