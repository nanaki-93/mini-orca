package app

import (
	"context"

	"github.com/nanaki-93/mini-orca/v2/internal/app/checkexec"
)

type workspaceIdentity struct {
	fingerprint string
}

func workspaceFingerprint(ctx context.Context, root string) (string, error) {
	return checkexec.WorkspaceFingerprint(ctx, root)
}

func fingerprintCheckWorkspace(ctx context.Context, root string) (string, error) {
	return checkexec.FingerprintWorkspace(ctx, root)
}

func (fixture workspaceIdentity) verifyCurrent(ctx context.Context, root string) error {
	return checkexec.VerifyWorkspaceFingerprint(ctx, root, fixture.fingerprint)
}
