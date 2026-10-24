package agentruntime

import (
	"context"
	"errors"
	"iter"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

type serverExecution struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type serverRequest struct {
	cancel context.CancelFunc
	done   chan struct{}
}

func (s *AgentServer) beginRequest(ctx context.Context) (context.Context, func(), error) {
	select {
	case s.permits <- struct{}{}:
	default:
		return nil, nil, errors.New("agent request limit reached")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		<-s.permits
		return nil, nil, errors.New("agent server closed")
	}
	scoped, cancel := context.WithTimeout(ctx, s.config.Timeout+5*time.Second)
	request := &serverRequest{cancel: cancel, done: make(chan struct{})}
	s.requests[request] = true
	return scoped, func() {
		cancel()
		s.mu.Lock()
		delete(s.requests, request)
		close(request.done)
		s.mu.Unlock()
		<-s.permits
	}, nil
}

func (s *AgentServer) Execute(ctx context.Context, request *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	return func(yield func(a2a.Event, error) bool) {
		scoped, finish, err := s.beginExecution(ctx, request.TaskID)
		if err != nil {
			yield(nil, errors.New("agent execution unavailable"))
			return
		}
		defer finish()
		for event, err := range s.executor.Execute(scoped, request) {
			if !yield(event, err) {
				return
			}
		}
	}
}

func (s *AgentServer) beginExecution(ctx context.Context, id a2a.TaskID) (context.Context, func(), error) {
	scope, ok := ctx.Value(serverScopeKey{}).(serverScope)
	if !ok || scope.admission.Validate(ctx) != nil || context.Cause(scope.request) != nil {
		return nil, nil, a2a.ErrUnauthenticated
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || len(s.active) >= s.config.MaxConcurrent {
		return nil, nil, errors.New("agent execution limit reached")
	}
	deadline := minTime(time.Now().Add(s.config.Timeout), scope.admission.ExpiresAt)
	scoped, cancel := context.WithDeadline(ctx, deadline)
	stop := context.AfterFunc(scope.request, cancel)
	s.active[id] = &serverExecution{cancel: cancel, done: make(chan struct{})}
	return scoped, func() { stop(); cancel() }, nil
}

func minTime(first, second time.Time) time.Time {
	if first.Before(second) {
		return first
	}
	return second
}

func (s *AgentServer) Cancel(ctx context.Context, request *a2asrv.ExecutorContext) iter.Seq2[a2a.Event, error] {
	// Native cancellation writes the terminal event before canceling and joining
	// the producer, so a canceled task is not overwritten by a provider error.
	return s.executor.Cancel(ctx, request)
}

func (s *AgentServer) Cleanup(_ context.Context, request *a2asrv.ExecutorContext, _ a2a.SendMessageResult, _ error) {
	if request.Message == nil {
		return // Cancellation cleanup must not release the execution's owner.
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if execution, exists := s.active[request.TaskID]; exists {
		execution.cancel()
		delete(s.active, request.TaskID)
		close(execution.done)
	}
}

func (s *AgentServer) Close(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	var pending []<-chan struct{}
	for execution := range s.requests {
		execution.cancel()
		pending = append(pending, execution.done)
	}
	for _, execution := range s.active {
		execution.cancel()
		pending = append(pending, execution.done)
	}
	s.mu.Unlock()
	for _, done := range pending {
		select {
		case <-done:
		case <-ctx.Done():
			return context.Cause(ctx)
		}
	}
	return nil
}
