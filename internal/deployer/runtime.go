package deployer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

func run(ctx context.Context, out io.Writer, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}

const portMin, portMax = 20000, 29999

func AllocPort(used map[int]bool) (int, error) {
	for p := portMin; p <= portMax; p++ {
		if used[p] {
			continue
		}
		l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(p))
		if err != nil {
			continue
		}
		l.Close()
		return p, nil
	}
	return 0, errors.New("no available port")
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

func WriteEnvFile(path string, containerPort int, env map[string]string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "PORT=%d\n", containerPort)

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}

	return os.WriteFile(path, []byte(b.String()), 0o600)
}
