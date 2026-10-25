package agentstate

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"
)

const journalVersion = 2
const journalRuntimeFamily = "adk-go-v2"

func validateJournalFormat(db *database) error {
	if db.Version == 1 && db.RuntimeFamily == "" {
		return nil
	}
	if db.Version == journalVersion && db.RuntimeFamily == journalRuntimeFamily {
		return nil
	}
	return fmt.Errorf("%w: unsupported journal format or runtime family", ErrCorrupt)
}

// Migration participates in the caller's locked atomic mutation. Reads never
// rewrite history, and a failed mutation cannot commit only half a migration.
func migrateJournalForWrite(ctx context.Context, db *database) error {
	if db.Version == journalVersion {
		return nil
	}
	if err := validateJournalFormat(db); err != nil {
		return err
	}
	now := time.Now().UTC()
	var interrupted []string
	for _, id := range slices.Sorted(maps.Keys(db.Runs)) {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		run := db.Runs[id]
		if run.Status != "running" {
			continue
		}
		// Running SDK invocations cannot survive a process/runtime cutover.
		// Host-owned waiting-input receipts have independent fresh admission.
		run.Status = "interrupted"
		if now.After(run.Updated) {
			run.Updated = now
		}
		db.Runs[id] = run
		interrupted = append(interrupted, id)
	}
	// Retain the same bounded event window without repeatedly copying it for
	// every orphan in a full journal. Sequence still counts evicted events.
	omitted := reserveMigrationEvents(db, len(interrupted))
	for _, id := range interrupted[omitted:] {
		if err := context.Cause(ctx); err != nil {
			return err
		}
		run := db.Runs[id]
		if err := appendProgress(db, run, "finished", nil); err != nil {
			return err
		}
	}
	db.Version, db.RuntimeFamily = journalVersion, journalRuntimeFamily
	return nil
}

func reserveMigrationEvents(db *database, count int) int {
	if count == 0 {
		return 0
	}
	omitted := max(0, count-MaxProgressEvents)
	keep := min(len(db.Progress), max(0, MaxProgressEvents-count))
	db.Progress = slices.Clone(db.Progress[len(db.Progress)-keep:])
	db.ProgressSequence += uint64(omitted)
	return omitted
}
