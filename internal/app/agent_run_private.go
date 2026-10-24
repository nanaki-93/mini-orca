package app

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func (s *Service) readAgentPrivate(ctx context.Context, store *agentstate.Store, run agentstate.RunSnapshot) (*AgentPrivateText, error) {
	value, err := s.readAgentText(ctx, store, run)
	if err != nil {
		return nil, err
	}
	record, err := store.Session(ctx, run.Session)
	if err != nil {
		return nil, fmt.Errorf("read private invocation: %w", err)
	}
	tools, truncated, err := agentruntime.InspectToolEvents(record.Events)
	if err != nil {
		return nil, err
	}
	value.ToolsTruncated = truncated
	for _, tool := range tools {
		value.ToolRecords = append(value.ToolRecords, AgentToolRecord{EventID: tool.EventID, CallID: tool.CallID, Name: tool.Name, Kind: tool.Kind, Value: tool.Value, Truncated: tool.Truncated})
	}
	return value, nil
}
