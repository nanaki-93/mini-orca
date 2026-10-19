package app

import (
	"context"
	"io"

	"github.com/nanaki-93/mini-orca/v2/internal/app/gitops"
)

func gitCommand(ctx context.Context, root, index string, input io.Reader, args ...string) (string, error) {
	return gitops.Command(ctx, root, index, input, args...)
}

func gitRead(ctx context.Context, root string, args ...string) (string, error) {
	return gitops.Read(ctx, root, args...)
}
