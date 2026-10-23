package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const maxAgentFrame = 1 << 20

// StreamAgentEvents owns one connection. The caller reconnects with the returned
// cursor; a failed callback leaves that batch unacknowledged. Close/Cancel and the
// parent context stop the response body, including a blocked read.
func (c *Client) StreamAgentEvents(parent context.Context, id, target, cursor string, receive func(AgentEventBatch) error) (string, error) {
	if id == "" || len(id) > 128 || receive == nil {
		return cursor, fmt.Errorf("unsupported agent progress request")
	}
	query, err := agentStreamQuery(target, cursor)
	if err != nil {
		return cursor, err
	}
	ctx, cancel, err := c.registerRequest(parent, id)
	if err != nil {
		return cursor, err
	}
	defer c.unregisterRequest(id, cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+target, nil)
	if err != nil {
		return cursor, err
	}
	req.Header.Set("Accept", "text/event-stream")
	if cursor != "" {
		req.Header.Set("Last-Event-ID", cursor)
	}
	httpClient := *c.http
	httpClient.Timeout = 0
	response, err := httpClient.Do(req)
	if err != nil {
		return cursor, fmt.Errorf("connect agent progress: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		return cursor, fmt.Errorf("agent progress unavailable (HTTP %d)", response.StatusCode)
	}
	return readAgentEvents(response.Body, query.Get("project_id"), query.Get("project_revision"), cursor, receive)
}

func agentStreamQuery(target, cursor string) (url.Values, error) {
	u, err := url.ParseRequestURI(target)
	if err != nil || u.Path != "/api/projects/current/agent-runs/events" || !allowedRequest(http.MethodGet, target, "") {
		return nil, fmt.Errorf("unsupported agent progress request")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(query["project_id"]) != 1 || len(query["project_revision"]) != 1 || query.Get("project_id") == "" || query.Get("project_revision") == "" || query.Has("cursor") {
		return nil, fmt.Errorf("provide exact progress scope; pass the cursor separately")
	}
	_, _, err = agentCursor(cursor)
	return query, err
}

type agentEventFrame struct {
	event, id string
	data      strings.Builder
	size      int
}

func readAgentEvents(reader io.Reader, projectID, revision, cursor string, receive func(AgentEventBatch) error) (string, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), maxAgentFrame+1)
	frame := agentEventFrame{}
	for scanner.Scan() {
		line := scanner.Text()
		frame.size += len(line) + 1
		if frame.size > maxAgentFrame {
			return cursor, fmt.Errorf("agent progress frame exceeds 1 MiB")
		}
		if line == "" {
			if frame.data.Len() != 0 {
				next, err := consumeAgentFrame(&frame, projectID, revision, cursor, receive)
				if err != nil {
					return cursor, err
				}
				cursor = next
			}
			frame = agentEventFrame{}
			continue
		}
		if err := frame.append(line); err != nil {
			return cursor, err
		}
	}
	if err := scanner.Err(); err != nil {
		return cursor, fmt.Errorf("read agent progress: %w", err)
	}
	if frame.data.Len() != 0 || frame.event != "" || frame.id != "" {
		return cursor, io.ErrUnexpectedEOF
	}
	return cursor, io.EOF
}

func (f *agentEventFrame) append(line string) error {
	if strings.HasPrefix(line, ":") {
		return nil
	}
	name, value, _ := strings.Cut(line, ":")
	value = strings.TrimPrefix(value, " ")
	switch name {
	case "event":
		f.event = value
	case "id":
		f.id = value
	case "data":
		f.data.WriteString(value)
		f.data.WriteByte('\n')
	default:
		return fmt.Errorf("unsupported agent progress field")
	}
	return nil
}

func agentCursor(cursor string) (string, uint64, error) {
	if cursor == "" {
		return "", 0, nil
	}
	epoch, seq, ok := strings.Cut(cursor, ":")
	if !ok || epoch == "" || len(cursor) > 180 || strings.ContainsAny(epoch, "/\\\x00\r\n ") {
		return "", 0, fmt.Errorf("invalid agent cursor")
	}
	sequence, err := strconv.ParseUint(seq, 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid agent sequence")
	}
	return epoch, sequence, nil
}

func consumeAgentFrame(frame *agentEventFrame, projectID, revision, cursor string, receive func(AgentEventBatch) error) (string, error) {
	batch, err := decodeAgentFrame(frame, projectID, revision)
	if err != nil {
		return cursor, err
	}
	epoch, sequence, err := agentCursor(batch.Cursor)
	if err != nil || epoch != batch.Epoch {
		return cursor, fmt.Errorf("agent progress cursor mismatch")
	}
	if err := validateAgentBatch(batch, sequence); err != nil {
		return cursor, err
	}
	previousEpoch, previous, err := agentCursor(cursor)
	if err != nil {
		return cursor, err
	}
	if !batch.Reset && epoch != previousEpoch {
		return cursor, fmt.Errorf("agent progress requires a snapshot reset")
	}
	if !batch.Reset {
		if sequence <= previous {
			return cursor, nil
		}
		filtered := batch.Events[:0]
		for _, event := range batch.Events {
			if event.Sequence > previous {
				filtered = append(filtered, event)
			}
		}
		batch.Events = filtered
	}
	if err := receive(batch); err != nil {
		return cursor, err
	}
	return batch.Cursor, nil
}

func decodeAgentFrame(frame *agentEventFrame, projectID, revision string) (AgentEventBatch, error) {
	var batch AgentEventBatch
	decoder := json.NewDecoder(strings.NewReader(frame.data.String()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&batch); err != nil {
		return batch, fmt.Errorf("decode agent progress: %w", err)
	}
	if decoder.Decode(new(any)) != io.EOF || frame.event != "agent_progress" || frame.id != batch.Cursor || batch.ProjectID != projectID || batch.ProjectRevision != revision {
		return batch, fmt.Errorf("agent progress identity mismatch")
	}
	return batch, nil
}

func validateAgentBatch(batch AgentEventBatch, sequence uint64) error {
	if batch.Reset != (batch.Snapshot != nil) || len(batch.Events) > 128 {
		return fmt.Errorf("invalid agent progress batch")
	}
	if batch.Reset {
		return validateAgentSnapshot(batch)
	}
	var previous uint64
	for _, event := range batch.Events {
		if event.ProjectID != batch.ProjectID || event.ProjectRevision == "" || event.Sequence <= previous || event.Sequence > sequence || event.RunID == "" || event.InvocationID == "" {
			return fmt.Errorf("invalid agent event identity or order")
		}
		previous = event.Sequence
	}
	return nil
}

func validateAgentSnapshot(batch AgentEventBatch) error {
	snapshot := batch.Snapshot
	if snapshot.ProjectID != batch.ProjectID || snapshot.ProjectRevision != batch.ProjectRevision || len(snapshot.Runs) > 100 || len(batch.Events) != 0 || batch.HasMore {
		return fmt.Errorf("invalid agent progress snapshot")
	}
	for _, run := range snapshot.Runs {
		if run.ProjectID != batch.ProjectID || run.ID == "" || run.RootID != run.ID {
			return fmt.Errorf("foreign invocation in agent snapshot")
		}
	}
	return nil
}
