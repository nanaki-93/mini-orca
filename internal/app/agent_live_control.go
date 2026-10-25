package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func (s *Service) OpenLiveSession(ctx context.Context, request LiveOpenRequest) (*LiveSession, error) {
	if !request.ConfirmConnect || !llm.SupportsLiveSessions(s.liveRuntime().profile) {
		return nil, fmt.Errorf("configure a supported live model and explicitly confirm connection")
	}
	if err := s.RequireRemoteConfirmation(config.CodeModelScope, request.ConfirmRemoteProvider); err != nil {
		return nil, err
	}
	owner, err := s.admitLiveOwner(ctx, request.LiveBinding)
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { owner.cancel(context.Cause(ctx)) })
	owner.opMu.Lock()
	err = s.connectOwnedLive(owner)
	owner.opMu.Unlock()
	stop()
	if ctx.Err() != nil {
		owner.cancel(context.Cause(ctx))
		<-owner.done
		return nil, context.Cause(ctx)
	}
	if err != nil {
		return nil, liveControlError(err)
	}
	owner.mu.Lock()
	id := owner.record.ID
	owner.mu.Unlock()
	return s.LiveSessionDetail(ctx, AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}, id)
}

func (s *Service) admitLiveOwner(ctx context.Context, binding LiveBinding) (*liveSessionOwner, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	root := s.manager.Root()
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	if err := s.validateLiveBinding(ctx, root, policy.Version(), binding); err != nil {
		return nil, err
	}
	timed, stop := context.WithTimeoutCause(context.WithoutCancel(ctx), agentruntime.LiveLifetime, &agentruntime.BudgetError{Limit: "live lifetime"})
	base, cancel := context.WithCancelCause(timed)
	registered, finish, err := s.integrations.begin(base, "@live")
	if err != nil {
		stop()
		cancel(err)
		return nil, err
	}
	runtime := s.liveRuntime()
	admitted, release, err := s.agentRunContextAt(registered, root, runtime)
	if err != nil {
		finish()
		stop()
		cancel(err)
		return nil, err
	}
	cleanup := func() { release(); finish(); stop(); cancel(context.Canceled) }
	store, err := agentstate.Open(root)
	if err != nil {
		cleanup()
		return nil, artifactLibraryError(err)
	}
	now := time.Now().UTC()
	expires, _ := admitted.Deadline()
	owner := &liveSessionOwner{root: root, ctx: admitted, cancel: cancel, finish: cleanup, done: make(chan struct{}), store: store, runtime: runtime, binding: binding, policy: policy.Version()}
	owner.record = agentstate.LiveRecord{ID: rand.Text(), ControlRevision: 1, ProjectID: binding.ProjectID, ProjectRevision: binding.ProjectRevision, Policy: owner.policy, Configuration: binding.Configuration, SettingsVersion: binding.SettingsVersion, Model: binding.Model, ProviderOrigin: binding.ProviderOrigin, State: "connecting", Connections: 1, CreatedAt: now, ExpiresAt: expires.UTC(), RunIDs: []string{}, Artifacts: []agentstate.ArtifactReference{}, Usage: agentruntime.BudgetFromContext(admitted).Snapshot()}
	if err := owner.persistLocked(ctx); err != nil {
		cleanup()
		return nil, artifactLibraryError(err)
	}
	if err := s.live.register(owner); err != nil {
		owner.record.State = "failed"
		owner.cancel(err)
		_ = owner.persistLocked(ctx)
		cleanup()
		return nil, err
	}
	go s.ownLiveLifetime(owner)
	return owner, nil
}

func (s *Service) connectOwnedLive(owner *liveSessionOwner) error {
	if err := context.Cause(owner.ctx); err != nil {
		return err
	}
	authorize := func(ctx context.Context) error {
		return s.validateLiveBinding(ctx, owner.root, owner.policy, owner.binding)
	}
	if err := authorize(owner.ctx); err != nil {
		owner.cancel(err)
		return err
	}
	owner.mu.Lock()
	owner.base = owner.record
	number, id := owner.record.Connections, owner.record.ID
	owner.mu.Unlock()
	connection, err := agentruntime.OpenLive(owner.ctx, owner.runtime.client, agentruntime.LiveOptions{ID: id, Connection: number, Authorize: authorize})
	owner.mu.Lock()
	if err != nil {
		owner.record.State = "failed"
		owner.record.Usage = agentruntime.BudgetFromContext(owner.ctx).Snapshot()
		owner.record.ControlRevision++
		cleanup, stop := context.WithTimeout(context.WithoutCancel(owner.ctx), 5*time.Second)
		saveErr := owner.persistLocked(cleanup)
		stop()
		owner.mu.Unlock()
		return errors.Join(err, saveErr)
	}
	owner.connection = connection
	mergeLiveSnapshot(&owner.record, owner.base, connection.Snapshot())
	owner.workers.Add(1)
	err = owner.persistLocked(owner.ctx)
	owner.mu.Unlock()
	go s.watchLiveConnection(owner, connection)
	s.notifyEvent("agent_live")
	return err
}

func (s *Service) ControlLiveSession(ctx context.Context, id string, request LiveControl) (*LiveSession, error) {
	if request.Attachment != nil {
		copy := *request.Attachment
		request.Attachment = &copy
	}
	root := s.manager.Root()
	query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
	if err := s.artifactLibraryScope(ctx, query, root); err != nil {
		return nil, err
	}
	owner := s.live.owner(root, id)
	if owner == nil {
		return nil, agentstate.ErrLiveConflict
	}
	owner.opMu.Lock()
	err := s.controlLiveOwner(ctx, owner, request)
	owner.opMu.Unlock()
	if request.Action == "reconnect" && ctx.Err() != nil && owner.ctx.Err() != nil {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer stop()
		return nil, errors.Join(context.Cause(ctx), waitIntegrationOperations(cleanup, []<-chan struct{}{owner.done}))
	}
	if request.Action == "close" && err == nil {
		err = waitIntegrationOperations(ctx, []<-chan struct{}{owner.done})
	}
	if err != nil {
		return nil, liveControlError(err)
	}
	return s.LiveSessionDetail(ctx, query, id)
}

func (s *Service) controlLiveOwner(ctx context.Context, owner *liveSessionOwner, request LiveControl) error {
	connection, err := s.admitLiveControl(ctx, owner, request)
	if err != nil {
		return err
	}
	input, authorize, err := s.liveControlInput(ctx, owner, request)
	if err != nil {
		return err
	}
	owner.mu.Lock()
	owner.record.ControlRevision++
	if request.Action == "reconnect" {
		owner.record.Connections++
		owner.record.State = "connecting"
	}
	err = owner.persistLocked(ctx)
	owner.mu.Unlock()
	if err != nil {
		return err
	}
	switch request.Action {
	case "close":
		owner.mu.Lock()
		owner.closing = true
		owner.mu.Unlock()
		owner.cancel(context.Canceled)
		return nil
	case "reconnect":
		return s.reconnectOwnedLive(ctx, owner)
	case "send":
		err = connection.Send(ctx, input, authorize)
	case "interrupt":
		err = connection.Interrupt(ctx, authorize)
	}
	if err != nil {
		err = errors.Join(err, connection.Wait())
	}
	owner.mu.Lock()
	owner.record = owner.currentLocked()
	cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	saveErr := owner.persistLocked(cleanup)
	stop()
	owner.mu.Unlock()
	return errors.Join(err, saveErr)
}

func (s *Service) reconnectOwnedLive(ctx context.Context, owner *liveSessionOwner) error {
	stop := context.AfterFunc(ctx, func() { owner.cancel(context.Cause(ctx)) })
	err := s.connectOwnedLive(owner)
	stop()
	if ctx.Err() != nil {
		owner.cancel(context.Cause(ctx))
		return errors.Join(err, context.Cause(ctx))
	}
	return err
}

func (s *Service) admitLiveControl(ctx context.Context, owner *liveSessionOwner, request LiveControl) (*agentruntime.LiveConnection, error) {
	if err := context.Cause(owner.ctx); err != nil {
		return nil, agentstate.ErrLiveConflict
	}
	owner.mu.Lock()
	valid := owner.binding == request.LiveBinding && owner.record.ControlRevision == request.ControlRevision && owner.failure == nil
	state, connections, connection := owner.record.State, owner.record.Connections, owner.connection
	owner.mu.Unlock()
	if !valid {
		return nil, agentstate.ErrLiveConflict
	}
	if err := validateLiveControlShape(request, state, connections, connection != nil); err != nil {
		return nil, err
	}
	if request.Action != "close" {
		if err := s.validateLiveBinding(ctx, owner.root, owner.policy, owner.binding); err != nil {
			return nil, err
		}
	}
	return connection, nil
}

func validateLiveControlShape(request LiveControl, state string, connections int, connected bool) error {
	if request.Action != "send" && (request.Text != "" || request.Attachment != nil) {
		return fmt.Errorf("only send accepts text or attachment input")
	}
	switch request.Action {
	case "close":
		return nil
	case "reconnect":
		if connected || connections >= 2 || !slices.Contains([]string{"failed", "disconnected", "go_away"}, state) || !request.ConfirmConnect {
			return fmt.Errorf("reconnect requires a disconnected owned session, remaining connection budget and fresh confirmation")
		}
	case "send", "interrupt":
		if !connected || state != "connected" {
			return agentstate.ErrLiveConflict
		}
	default:
		return fmt.Errorf("live action must be send, interrupt, reconnect or close")
	}
	return nil
}

func (s *Service) liveControlInput(ctx context.Context, owner *liveSessionOwner, request LiveControl) (agentruntime.LiveInput, func(context.Context) error, error) {
	input := agentruntime.LiveInput{Text: request.Text}
	authorize := func(ctx context.Context) error {
		return s.validateLiveBinding(ctx, owner.root, owner.policy, owner.binding)
	}
	if request.Action == "send" || request.Action == "reconnect" {
		if err := s.RequireRemoteConfirmation(config.CodeModelScope, request.ConfirmRemoteProvider); err != nil {
			return input, nil, err
		}
	}
	if request.Action != "send" {
		return input, authorize, nil
	}
	if !request.ConfirmSourceSharing {
		return input, nil, fmt.Errorf("live input requires explicit sharing confirmation")
	}
	if request.Attachment != nil {
		query := AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}
		refs := []AttachmentReference{*request.Attachment}
		media, err := s.loadAttachments(ctx, query, owner.root, owner.policy, refs)
		if err != nil {
			return input, nil, err
		}
		input.Media = &media[0]
		authorize = func(ctx context.Context) error {
			if err := s.validateLiveBinding(ctx, owner.root, owner.policy, owner.binding); err != nil {
				return err
			}
			_, err := s.loadAttachments(ctx, query, owner.root, owner.policy, refs)
			return err
		}
	}
	return input, authorize, agentruntime.ValidateLiveInput(input)
}

func liveControlError(err error) error {
	if errors.Is(err, agentruntime.ErrLiveTransport) || errors.Is(err, agentruntime.ErrLiveProtocol) {
		return fmt.Errorf("%w: %w", ErrLiveUnavailable, err)
	}
	if errors.Is(err, agentstate.ErrPersistence) || errors.Is(err, agentstate.ErrCorrupt) {
		return artifactLibraryError(err)
	}
	return err
}
