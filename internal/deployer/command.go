package deployer

import (
	"context"
	"io"
	"os"
	"os/exec"
)

func run(ctx context.Context, out io.Writer, dir string, env []string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = append(os.Environ(), env...)
	return cmd.Run()
}
