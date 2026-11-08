package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
)

func ServiceURL(path string) (string, error) {
	scheme := getEnv("SCHEME", "http")
	host := getEnv("HOST", "localhost")
	port := getEnv("PORT", "8080")

	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid PORT %q: %w", port, err)
	}

	u := url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(host, port),
	}
	return u.JoinPath(path).String(), nil
}

func BaseURL() string {
	scheme := getEnv("SCHEME", "http")
	host := getEnv("HOST", "localhost")
	port := getEnv("PORT", "8080")

	return fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(host, port))
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
