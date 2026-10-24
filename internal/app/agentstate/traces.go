package agentstate

import (
	"context"
	"encoding/hex"
	"fmt"
	"slices"
	"time"
)

const MaxTraceSpans = 4096

// TraceSpan accepts only host identities, categories and timestamps. Raw SDK
// attributes, errors, arguments and outputs cannot be represented here.
type TraceSpan struct {
	ID           string    `json:"id"`
	ParentID     string    `json:"parent_id,omitempty"`
	RunKey       string    `json:"run_key,omitempty"`
	OperationKey string    `json:"operation_key,omitempty"`
	Kind         string    `json:"kind"`
	Status       string    `json:"status"`
	Started      time.Time `json:"started"`
	Ended        time.Time `json:"ended,omitzero"`
	ElapsedNanos int64     `json:"elapsed_nanos"`
}

type TraceRecord struct {
	Version int         `json:"version"`
	Dropped uint64      `json:"dropped"`
	Spans   []TraceSpan `json:"spans"`
}

func validSpanID(value string) bool {
	data, err := hex.DecodeString(value)
	return err == nil && len(data) == 8 && value != "0000000000000000"
}

func validTraceSpan(span TraceSpan) bool {
	if !validSpanID(span.ID) || span.Started.IsZero() || span.ElapsedNanos < 0 {
		return false
	}
	if span.ParentID != "" && (!validSpanID(span.ParentID) || span.ParentID == span.ID) {
		return false
	}
	for _, key := range []string{span.RunKey, span.OperationKey} {
		if key != "" && !validHash(key) {
			return false
		}
	}
	if !slices.Contains([]string{"invocation", "node", "tool", "provider", "retry", "summary", "delegation", "check"}, span.Kind) {
		return false
	}
	if span.Status == "running" {
		return span.Ended.IsZero() && span.ElapsedNanos == 0
	}
	return slices.Contains([]string{"completed", "failed", "canceled", "timed_out"}, span.Status) && !span.Ended.Before(span.Started)
}

func validateTraces(db *database) error {
	count := 0
	for budget, record := range db.Traces {
		if _, ok := db.Budgets[budget]; !ok || record.Version != 1 || record.Dropped > 1<<50 {
			return ErrCorrupt
		}
		seen := map[string]bool{}
		for _, span := range record.Spans {
			if seen[span.ID] || !validTraceSpan(span) {
				return ErrCorrupt
			}
			seen[span.ID] = true
		}
		if !validTraceParents(record.Spans) {
			return ErrCorrupt
		}
		count += len(record.Spans)
	}
	if count > MaxTraceSpans {
		return ErrCorrupt
	}
	return nil
}

func validTraceParents(spans []TraceSpan) bool {
	parents := make(map[string]string, len(spans))
	for _, span := range spans {
		parents[span.ID] = span.ParentID
	}
	finished := map[string]bool{}
	for _, span := range spans {
		path := map[string]bool{}
		for id := span.ID; id != "" && !finished[id]; id = parents[id] {
			if path[id] {
				return false
			}
			path[id] = true
		}
		for id := range path {
			finished[id] = true
		}
	}
	return true
}

// BeginSpan evicts only completed spans. If every slot is active, it records
// the omission instead of stealing an unfinished span's eventual receipt.
func (s *Store) BeginSpan(ctx context.Context, budget string, span TraceSpan) (bool, error) {
	recorded := false
	err := s.mutate(ctx, func(db *database) error {
		var err error
		recorded, err = beginTraceRecord(db, budget, span)
		return err
	})
	return recorded && err == nil, err
}

func beginTraceRecord(db *database, budget string, span TraceSpan) (bool, error) {
	if span.Status != "running" || !validTraceSpan(span) {
		return false, fmt.Errorf("invalid source-free trace span")
	}
	if db.Traces == nil {
		db.Traces = map[string]TraceRecord{}
	}
	if slices.ContainsFunc(db.Traces[budget].Spans, func(item TraceSpan) bool { return item.ID == span.ID }) {
		return false, fmt.Errorf("trace span already exists")
	}
	room := makeTraceRoom(db)
	record := db.Traces[budget]
	record.Version = 1
	if room {
		record.Spans = append(record.Spans, span)
	} else {
		record.Dropped++
	}
	db.Traces[budget] = record
	return room, nil
}

// applyTraceRecord joins a trace transition to its existing durable owner.
// Completion cannot change the admitted identity, parent or operation.
func applyTraceRecord(db *database, budget string, span *TraceSpan) (bool, error) {
	if span == nil {
		return false, nil
	}
	if span.Status == "running" {
		return beginTraceRecord(db, budget, *span)
	}
	index := slices.IndexFunc(db.Traces[budget].Spans, func(item TraceSpan) bool { return item.ID == span.ID })
	if index < 0 {
		return false, ErrNotFound
	}
	previous := db.Traces[budget].Spans[index]
	identity := *span
	identity.Status, identity.Ended, identity.ElapsedNanos = previous.Status, previous.Ended, previous.ElapsedNanos
	if identity.Started.Equal(previous.Started) {
		identity.Started = previous.Started
	}
	if identity != previous {
		return false, fmt.Errorf("trace completion changed its admitted identity")
	}
	return true, finishTraceRecord(db, budget, span.ID, span.Status, span.Ended, span.ElapsedNanos)
}

func makeTraceRoom(db *database) bool {
	count, oldestIndex := 0, -1
	var oldestBudget string
	var oldest TraceSpan
	for budget, record := range db.Traces {
		count += len(record.Spans)
		for i, span := range record.Spans {
			if span.Status == "running" {
				continue
			}
			if oldestIndex == -1 || span.Ended.Before(oldest.Ended) || span.Ended.Equal(oldest.Ended) && span.ID < oldest.ID {
				oldestBudget, oldestIndex, oldest = budget, i, span
			}
		}
	}
	if count < MaxTraceSpans {
		return true
	}
	if oldestIndex == -1 {
		return false
	}
	record := db.Traces[oldestBudget]
	record.Spans = slices.Delete(record.Spans, oldestIndex, oldestIndex+1)
	record.Dropped++
	db.Traces[oldestBudget] = record
	return true
}

func (s *Store) FinishSpan(ctx context.Context, budget, id, status string, ended time.Time, elapsed int64) error {
	return s.mutate(ctx, func(db *database) error {
		return finishTraceRecord(db, budget, id, status, ended, elapsed)
	})
}

func finishTraceRecord(db *database, budget, id, status string, ended time.Time, elapsed int64) error {
	record := db.Traces[budget]
	index := slices.IndexFunc(record.Spans, func(item TraceSpan) bool { return item.ID == id })
	if index < 0 {
		return ErrNotFound
	}
	span := record.Spans[index]
	if span.Status != "running" || status == "running" {
		return fmt.Errorf("trace terminal status cannot be replaced")
	}
	span.Status, span.Ended, span.ElapsedNanos = status, ended, elapsed
	if !validTraceSpan(span) {
		return fmt.Errorf("invalid trace completion")
	}
	record.Spans[index] = span
	db.Traces[budget] = record
	return nil
}
