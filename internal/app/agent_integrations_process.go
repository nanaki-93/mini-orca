package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/sandbox"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func integrationCredential(reference string) (string, error) {
	if reference == "" {
		return "", nil
	}
	value, ok := os.LookupEnv(reference)
	if !ok || value == "" || len(value) > 8192 || strings.ContainsAny(value, "\x00\r\n") {
		return "", ErrIntegrationUnavailable
	}
	return value, nil
}

func (s *Service) integrationConnection(server config.MCPServer) (agentruntime.MCPConnection, string, error) {
	connection := agentruntime.MCPConnection{Endpoint: server.Endpoint, Timeout: time.Duration(server.TimeoutSeconds) * time.Second, Begin: func(ctx context.Context) (context.Context, func(), error) {
		return s.integrations.begin(ctx, server.ID)
	}}
	token, err := integrationCredential(server.BearerTokenEnv)
	if err != nil {
		return connection, "", err
	}
	connection.BearerToken = token
	credentials := map[string]string{"http": token}
	var environment []string
	for _, name := range slices.Sorted(maps.Keys(server.Environment)) {
		value, err := integrationCredential(server.Environment[name])
		if err != nil {
			return connection, "", err
		}
		credentials[name] = value
		environment = append(environment, name+"="+value)
	}
	if server.Transport == "stdio" {
		info, err := os.Stat(server.Command)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
			return connection, "", ErrIntegrationUnavailable
		}
		credentials["process_identity"] = fmt.Sprintf("%s/%d/%d/%d", server.Command, info.Size(), info.ModTime().UnixNano(), info.Mode())
		connection.Start = func(ctx context.Context) (io.ReadCloser, io.WriteCloser, func() error, error) {
			return startIntegrationProcess(ctx, server, environment)
		}
	}
	// A string map cannot fail JSON encoding. Only its digest leaves this scope.
	data, _ := json.Marshal(credentials)
	hash := sha256.Sum256(data)
	return connection, hex.EncodeToString(hash[:]), nil
}

func startIntegrationProcess(ctx context.Context, server config.MCPServer, environment []string) (io.ReadCloser, io.WriteCloser, func() error, error) {
	directory, err := os.MkdirTemp("", "mini-orca-mcp-")
	if err != nil {
		return nil, nil, nil, ErrIntegrationUnavailable
	}
	command := exec.CommandContext(ctx, server.Command, server.Args...)
	command.Dir, command.WaitDelay, command.Stderr = directory, 250*time.Millisecond, io.Discard
	command.Env = append([]string{"HOME=" + directory, "TMPDIR=" + directory, "PATH=/usr/bin:/bin"}, environment...)
	if err := sandbox.ConfigureCommand(command); err != nil {
		return nil, nil, nil, errors.Join(err, os.RemoveAll(directory))
	}
	reader, writer, err := integrationProcessPipes(command)
	if err != nil {
		return nil, nil, nil, errors.Join(err, os.RemoveAll(directory))
	}
	if err := command.Start(); err != nil {
		return nil, nil, nil, errors.Join(err, closeIntegrationPipe(reader), closeIntegrationPipe(writer), os.RemoveAll(directory))
	}
	cleanup := func() error {
		stopErr := sandbox.StopCommandGroup(command)
		pipeErr := errors.Join(closeIntegrationPipe(reader), closeIntegrationPipe(writer))
		waitErr := command.Wait()
		if stopErr != nil {
			// A fast normal exit can race the group signal (EPERM on Darwin).
			// Recover only after reaping and proving the entire group is absent.
			if proof := sandbox.VerifyCommandGroupStopped(command); proof == nil {
				stopErr = nil
			} else {
				stopErr = errors.Join(stopErr, proof)
			}
		}
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) && exit.ProcessState.ExitCode() == -1 && stopErr == nil {
			waitErr = nil // The owner deliberately terminated its persistent server.
		}
		return errors.Join(stopErr, pipeErr, waitErr, os.RemoveAll(directory))
	}
	return reader, writer, cleanup, nil
}

func integrationProcessPipes(command *exec.Cmd) (io.ReadCloser, io.WriteCloser, error) {
	reader, err := command.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	writer, err := command.StdinPipe()
	if err != nil {
		return nil, nil, errors.Join(err, reader.Close())
	}
	return reader, writer, nil
}

func closeIntegrationPipe(pipe io.Closer) error {
	err := pipe.Close()
	if errors.Is(err, os.ErrClosed) {
		return nil // The native IO transport may already own this close.
	}
	return err
}
