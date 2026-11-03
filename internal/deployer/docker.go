package deployer

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

func Build(ctx context.Context, out io.Writer, dir, image string) error {
	if _, err := os.Stat(filepath.Join(dir, "Dockerfile")); err != nil {
		return errors.New("no Dockerfile found at " + dir)
	}
	return run(ctx, out, dir, nil, "docker", "build", "-t", image, ".")
}
