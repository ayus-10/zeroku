package deployer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

func Clone(ctx context.Context, out io.Writer, repo, ref, dest string) (commit string, err error) {
	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, "--", repo, dest)
	if err := run(ctx, out, "", []string{"GIT_TERMINAL_PROMPT=0"}, "git", args...); err != nil {
		return "", fmt.Errorf("failed to clone: %w", err)
	}
	var buf bytes.Buffer
	if err := run(ctx, &buf, dest, nil, "git", "rev-parse", "HEAD"); err != nil {
		return "", fmt.Errorf("failed to get commit: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}
