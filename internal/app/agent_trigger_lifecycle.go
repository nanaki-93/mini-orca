package app

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type triggerController struct {
	mu    sync.Mutex
	owner *triggerOwner
	ticks <-chan time.Time // Optional deterministic clock source, captured before startup.
}
type triggerOwner struct {
	root, projectID string
	controlRevision int64
	store           *agentstate.Store
	limits          config.AgentTriggers
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	fault           bool // triggerController.mu owns this field.
	ticks           <-chan time.Time
}

func (s *Service) startTriggerOwner(root string, store *agentstate.Store, record agentstate.TriggerRecord) {
	ctx, cancel := context.WithCancel(context.Background())
	owner := &triggerOwner{root: root, projectID: record.ProjectID, controlRevision: record.ControlRevision, store: store, limits: s.agentRuntime.Triggers, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	s.triggers.mu.Lock()
	owner.ticks = s.triggers.ticks
	s.triggers.owner = owner
	s.triggers.mu.Unlock()
	go s.ownTriggerWatch(owner)
}

func (s *Service) triggerOwnerState(root string, revision int64) (bool, bool) {
	if s.triggers == nil {
		return false, false
	}
	s.triggers.mu.Lock()
	defer s.triggers.mu.Unlock()
	owner := s.triggers.owner
	if owner == nil || owner.root != root || owner.controlRevision != revision {
		return false, false
	}
	select {
	case <-owner.done:
		return false, owner.fault
	default:
		return true, owner.fault
	}
}

func (s *Service) stopTriggers(ctx context.Context) error {
	if s.triggers == nil {
		return nil
	}
	s.triggers.mu.Lock()
	owner := s.triggers.owner
	s.triggers.mu.Unlock()
	if owner == nil {
		return nil
	}
	owner.cancel()
	select {
	case <-owner.done:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (s *Service) stopTriggersForLifecycle() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return s.stopTriggers(ctx)
}

func (s *Service) ownTriggerWatch(owner *triggerOwner) {
	defer close(owner.done)
	defer owner.cancel()
	ticks := owner.ticks
	if ticks == nil {
		ticker := time.NewTicker(time.Duration(owner.limits.PollSeconds) * time.Second)
		defer ticker.Stop()
		ticks = ticker.C
	}
	for {
		select {
		case <-owner.ctx.Done():
			return
		case now, open := <-ticks:
			if !open {
				return
			}
			if err := s.triggerTick(owner, now.UTC()); err != nil {
				if !errors.Is(err, context.Canceled) && !errors.Is(err, agentstate.ErrTriggerConflict) {
					s.triggers.mu.Lock()
					owner.fault = true
					s.triggers.mu.Unlock()
				}
				return
			}
		}
	}
}

func (s *Service) triggerTick(owner *triggerOwner, now time.Time) error {
	record, err := owner.store.Triggers(owner.ctx, owner.projectID)
	if err != nil {
		return err
	}
	if record.ControlRevision != owner.controlRevision || record.Options.Mode == "disabled" {
		return agentstate.ErrTriggerConflict
	}
	before := record
	observed, err := s.captureTriggerSources(owner.ctx, owner.root, record.Options.Paths, false)
	if err != nil {
		record.State, record.Reason = "unavailable", "Selected source is unavailable, excluded or changed while reading; no provider work was started."
		return saveTriggerObservation(owner, before, record)
	}
	if !maps.Equal(observed, record.Observed) {
		record.Observed, record.ChangedAt = observed, now
		record.State, record.Reason = "waiting", "Source changes detected; waiting for a quiet snapshot."
	}
	path := eligibleTriggerPath(&record, now, owner.limits)
	if path == "" {
		return saveTriggerObservation(owner, before, record)
	}
	if record.Grant == nil || !record.Grant.ExpiresAt.After(now) {
		record.State, record.Reason = "suggested", "Analysis suggested for changed source. Automatic provider work requires a current explicit grant."
		return saveTriggerObservation(owner, before, record)
	}
	return s.runTriggeredAnalysis(owner, before, record, path, now)
}

func eligibleTriggerPath(record *agentstate.TriggerRecord, now time.Time, limits config.AgentTriggers) string {
	if now.Before(record.NextEligibleAt) || now.Sub(record.ChangedAt) < time.Duration(limits.DebounceSeconds)*time.Second {
		return ""
	}
	for _, path := range record.Options.Paths {
		if record.Observed[path] != "" && record.Handled[path] != record.Observed[path] {
			return path
		}
	}
	if record.Options.Mode == "interval" {
		record.NextEligibleAt = now.Add(time.Duration(record.Options.IntervalSeconds) * time.Second)
		record.State, record.Reason = "unchanged", "Selected snapshots are unchanged; no provider work is needed."
	}
	return ""
}

func saveTriggerObservation(owner *triggerOwner, before, after agentstate.TriggerRecord) error {
	if reflect.DeepEqual(before, after) {
		return context.Cause(owner.ctx)
	}
	_, err := owner.store.SaveTriggers(owner.ctx, after, before.Version)
	return err
}

func (s *Service) captureTriggerSources(ctx context.Context, root string, paths []string, indexed bool) (map[string]string, error) {
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(paths))
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if indexed {
			file, err := s.manager.IndexedFile(path)
			if err != nil {
				return nil, err
			}
			if !isSemanticAnalysisCandidate(*file) {
				return nil, errors.New("watch an eligible indexed source file")
			}
		}
		hash, err := readTriggerSource(ctx, policy, root, path)
		if err != nil {
			return nil, err
		}
		result[path] = hash
	}
	return result, nil
}

func readTriggerSource(ctx context.Context, policy *project.ContextPolicy, root, path string) (string, error) {
	if !policy.Decide(path).Include {
		return "", project.ErrExcludedFile
	}
	if _, err := project.ResolveWritePath(root, path); err != nil {
		return "", err
	}
	info, err := project.GetFileInfo(root, path)
	if err != nil {
		return "", err
	}
	if info.Binary || len(info.Content) > 512<<10 {
		return "", errors.New("ambient sources must be UTF-8 text of at most 512 KiB")
	}
	if _, err := policy.ReadSource(ctx, path, info.ContentHash); err != nil {
		return "", err
	}
	return strings.TrimPrefix(info.ContentHash, "sha256:"), nil
}

func (s *Service) triggerCompetingWork() bool {
	s.changeWorkflow.mu.Lock()
	busy := false
	for _, job := range s.changeWorkflow.allJobs() {
		busy = busy || !job.finished
	}
	s.changeWorkflow.mu.Unlock()
	s.analysisRun.mu.Lock()
	busy = busy || s.analysisRun.done != nil || !canReplaceAnalysisRun(s.analysisRun.run, false)
	s.analysisRun.mu.Unlock()
	s.integrations.mu.Lock()
	busy = busy || len(s.integrations.active) != 0
	s.integrations.mu.Unlock()
	s.missions.mu.Lock()
	busy = busy || len(s.missions.effects) != 0
	for _, job := range s.missions.jobs {
		busy = busy || !job.finished
	}
	s.missions.mu.Unlock()
	s.goScan.mu.Lock()
	if s.goScan.done != nil {
		select {
		case <-s.goScan.done:
		default:
			busy = true
		}
	}
	s.goScan.mu.Unlock()
	return busy
}
