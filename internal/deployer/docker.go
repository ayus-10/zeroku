package deployer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

func Build(ctx context.Context, out io.Writer, dir, image string) error {
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		return errors.New("no Dockerfile found at " + dir)
	}
	return run(ctx, out, dir, nil, "docker", "build", "-t", image, ".")
}

func RunContainer(ctx context.Context, out io.Writer, name, image, envFile string, hostPort, containerPort int) error {
	args := []string{
		"run", "-d",
		"--name", name,
		"--restart", "unless-stopped",
		"--env-file", envFile,
		"--cpus", "0.5",
		"--memory", "256m",
		"-p", strconv.Itoa(hostPort) + ":" + strconv.Itoa(containerPort),
		image,
	}
	return run(ctx, out, "", nil, "docker", args...)
}

func StopContainer(ctx context.Context, out io.Writer, image string) error {
	args := []string{
		"stop", image,
	}
	return run(ctx, out, "", nil, "docker", args...)
}

func StreamLogs(ctx context.Context, out io.Writer, container string, lines int) {
	args := []string{"logs", "--tail", fmt.Sprint(lines), container}
	_ = run(ctx, out, "", nil, "docker", args...)
}
