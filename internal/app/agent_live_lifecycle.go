package app

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type liveRegistry struct {
	mu       sync.Mutex
	sessions map[string]*liveSessionOwner
}
type liveSessionOwner struct {
	opMu       sync.Mutex // serializes explicit controls; never acquired by provider callbacks
	mu         sync.Mutex // owns record, base, connection and failure
	root       string
	ctx        context.Context
	cancel     context.CancelCauseFunc
	finish     func()
	done       chan struct{}
	workers    sync.WaitGroup
	store      *agentstate.Store
	runtime    modelRuntime
	binding    LiveBinding
	policy     string
	record     agentstate.LiveRecord
	base       agentstate.LiveRecord
	connection *agentruntime.LiveConnection
	failure    error
	closing    bool
}

func (r *liveRegistry) owner(root, id string) *liveSessionOwner {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	owner := r.sessions[id]
	if owner != nil && owner.root == root {
		return owner
	}
	return nil
}

func (r *liveRegistry) register(owner *liveSessionOwner) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	active := 0
	for id, previous := range r.sessions {
		select {
		case <-previous.done:
			delete(r.sessions, id)
		default:
			active++
		}
	}
	if active >= 4 {
		return errors.New("at most four live sessions may be owned by this checkout")
	}
	r.sessions[owner.record.ID] = owner
	return nil
}

func (s *Service) cancelLiveSessions() {
	if s.live == nil {
		return
	}
	s.live.mu.Lock()
	defer s.live.mu.Unlock()
	for _, owner := range s.live.sessions {
		owner.cancel(agentruntime.ErrPolicyDenied)
	}
}

func (s *Service) stopLiveSessions() error {
	if s.live == nil {
		return nil
	}
	s.live.mu.Lock()
	var pending []<-chan struct{}
	for _, owner := range s.live.sessions {
		owner.cancel(agentruntime.ErrPolicyDenied)
		pending = append(pending, owner.done)
	}
	s.live.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return waitIntegrationOperations(ctx, pending)
}

func (owner *liveSessionOwner) persistLocked(ctx context.Context) error {
	record, err := owner.store.SaveLiveSession(ctx, owner.record, owner.record.Version)
	if err != nil {
		owner.failure = err
		owner.record.State = "failed"
		owner.cancel(err)
		return err
	}
	owner.record = record
	return nil
}

func (owner *liveSessionOwner) currentLocked() agentstate.LiveRecord {
	record := owner.record
	if owner.connection != nil {
		mergeLiveSnapshot(&record, owner.base, owner.connection.Snapshot())
	}
	record.Artifacts = slices.Clone(record.Artifacts)
	record.RunIDs = slices.Clone(record.RunIDs)
	record.Usage = record.Usage.Clone()
	return record
}

func mergeLiveSnapshot(record *agentstate.LiveRecord, base agentstate.LiveRecord, snapshot agentruntime.LiveSnapshot) {
	record.Turns = base.Turns + snapshot.Turns
	record.CompletedTurns = base.CompletedTurns + snapshot.CompletedTurns
	record.SentBytes = base.SentBytes + snapshot.SentBytes
	record.ReceivedBytes = base.ReceivedBytes + snapshot.ReceivedBytes
	record.ReportedInput = base.ReportedInput + snapshot.ReportedInput
	record.ReportedOutput = base.ReportedOutput + snapshot.ReportedOutput
	record.Reported = base.Reported || snapshot.Reported
	record.Usage = snapshot.Usage
	record.Artifacts = append(slices.Clone(base.Artifacts), snapshot.Artifacts...)
	record.RunIDs = slices.Clone(base.RunIDs)
	if snapshot.RunID != "" {
		record.RunIDs = append(record.RunIDs, snapshot.RunID)
	}
	record.State = snapshot.State
}

func (s *Service) watchLiveConnection(owner *liveSessionOwner, connection *agentruntime.LiveConnection) {
	defer owner.workers.Done()
	_ = connection.Wait() // typed final state and retention failures are retained below
	owner.mu.Lock()
	if owner.connection == connection {
		mergeLiveSnapshot(&owner.record, owner.base, connection.Snapshot())
		owner.connection = nil
		owner.record.ControlRevision++
		ctx, stop := context.WithTimeout(context.WithoutCancel(owner.ctx), 5*time.Second)
		_ = owner.persistLocked(ctx) // persistLocked records the failure and cancels the owner
		stop()
	}
	owner.mu.Unlock()
	s.notifyEvent("agent_live")
}

func (s *Service) ownLiveLifetime(owner *liveSessionOwner) {
	defer close(owner.done)
	defer owner.finish()
	<-owner.ctx.Done()
	owner.opMu.Lock()
	owner.mu.Lock()
	connection := owner.connection
	owner.mu.Unlock()
	if connection != nil {
		connection.Close()
	}
	owner.opMu.Unlock()
	owner.workers.Wait()
	owner.mu.Lock()
	owner.record.State = "interrupted"
	if owner.closing {
		owner.record.State = "closed"
	}
	if errors.Is(context.Cause(owner.ctx), agentruntime.ErrBudgetExceeded) {
		owner.record.State = "limit_reached"
	}
	if owner.failure != nil {
		owner.record.State = "failed"
	}
	owner.record.ControlRevision++
	owner.record.Usage = agentruntime.BudgetFromContext(owner.ctx).Snapshot()
	ctx, stop := context.WithTimeout(context.WithoutCancel(owner.ctx), 5*time.Second)
	_ = owner.persistLocked(ctx)
	stop()
	owner.mu.Unlock()
	s.notifyEvent("agent_live")
}
