package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type agentHTTPServer struct {
	server     *http.Server
	closeAgent func(context.Context) error
	done       chan error
}

func newAgentHTTPServer(application *app.Service) (*agentHTTPServer, error) {
	cfg := application.RemoteServiceConfig()
	if !cfg.Enabled {
		return nil, nil
	}
	agent, err := application.NewRemoteService("http://" + cfg.Bind + "/a2a")
	if err != nil {
		return nil, err
	}
	server := &http.Server{Addr: cfg.Bind, Handler: api.LocalOnly(agent, false), ReadTimeout: 10 * time.Second,
		WriteTimeout: time.Duration(cfg.TimeoutSeconds+10) * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8 << 10}
	return &agentHTTPServer{server: server, closeAgent: agent.Close, done: make(chan error, 1)}, nil
}

func startAgentHTTPServer(application *app.Service) (*agentHTTPServer, error) {
	server, err := newAgentHTTPServer(application)
	if err != nil || server == nil {
		return server, err
	}
	listener, err := net.Listen("tcp", server.server.Addr)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("start local A2A listener: %w", err), server.closeAgent(context.Background()))
	}
	go func() {
		defer close(server.done)
		if err := server.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			server.done <- err
		}
	}()
	return server, nil
}

func (s *agentHTTPServer) failures() <-chan error {
	if s == nil {
		return nil
	}
	return s.done
}

func (s *agentHTTPServer) shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	err := s.closeAgent(ctx)
	if shutdownErr := s.server.Shutdown(ctx); shutdownErr != nil {
		err = errors.Join(err, shutdownErr, s.server.Close())
	}
	select {
	case serveErr := <-s.done:
		return errors.Join(err, serveErr)
	case <-ctx.Done():
		return errors.Join(err, context.Cause(ctx))
	}
}
