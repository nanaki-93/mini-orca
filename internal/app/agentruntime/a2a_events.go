package agentruntime

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"google.golang.org/adk/v2/server/adka2a/v2"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type remoteAccumulator struct {
	progress   agentstate.RemoteProgress
	events     int
	cleanupErr error
}

func newRemoteAccumulator(previous agentstate.RemoteProgress) *remoteAccumulator {
	previous.Artifacts = slices.Clone(previous.Artifacts)
	return &remoteAccumulator{progress: previous}
}

func remoteIdentity(value string) bool {
	return value != "" && len(value) <= 128 && value != "." && value != ".." && !strings.ContainsAny(value, "/\\\x00\r\n") && utf8.ValidString(value)
}

func (a *remoteAccumulator) accept(event a2a.Event) error {
	if event == nil || reflect.ValueOf(event).IsNil() || a.events >= 128 {
		return ErrRemoteProtocol
	}
	a.events++
	if err := a.identity(event.TaskInfo()); err != nil {
		return err
	}
	// Preserve a validated task identity even when its body is malformed, but
	// publish text and artifact changes only as one validated snapshot.
	trial := newRemoteAccumulator(a.progress)
	if err := trial.acceptBody(event); err != nil {
		return err
	}
	if err := trial.usage(remoteUsageMetadata(event)); err != nil {
		return err
	}
	if err := trial.progress.Validate(); err != nil {
		return err
	}
	a.progress = trial.progress
	return nil
}

func (a *remoteAccumulator) acceptBody(event a2a.Event) error {
	var err error
	switch value := event.(type) {
	case *a2a.Task:
		err = a.task(value)
	case *a2a.Message:
		a.progress.Text, err = remoteMessageText(value)
		a.progress.State = "completed"
	case *a2a.TaskStatusUpdateEvent:
		err = a.status(value.Status)
	case *a2a.TaskArtifactUpdateEvent:
		err = a.artifact(value.Artifact, value.Append, value.LastChunk)
	default:
		return ErrRemoteProtocol
	}
	return err
}

func (a *remoteAccumulator) identity(info a2a.TaskInfo) error {
	if info.TaskID == "" {
		if a.progress.TaskID != "" {
			return ErrRemoteProtocol
		}
		return nil
	}
	if !remoteIdentity(string(info.TaskID)) || !remoteIdentity(info.ContextID) {
		return ErrRemoteProtocol
	}
	if a.progress.TaskID != "" && a.progress.TaskID != string(info.TaskID) || a.progress.ContextID != "" && a.progress.ContextID != info.ContextID {
		return ErrRemoteProtocol
	}
	a.progress.TaskID, a.progress.ContextID = string(info.TaskID), info.ContextID
	return nil
}

func (a *remoteAccumulator) task(task *a2a.Task) error {
	if task.ID == "" || len(task.Artifacts) > 4 || len(task.History) > 4 {
		return ErrRemoteProtocol
	}
	a.progress.Artifacts = []agentstate.RemoteArtifact{}
	for _, artifact := range task.Artifacts {
		if err := a.artifact(artifact, false, true); err != nil {
			return err
		}
	}
	return a.status(task.Status)
}

func (a *remoteAccumulator) status(status a2a.TaskStatus) error {
	states := map[a2a.TaskState]string{a2a.TaskStateSubmitted: "submitted", a2a.TaskStateWorking: "working", a2a.TaskStateInputRequired: "input_required", a2a.TaskStateAuthRequired: "auth_required", a2a.TaskStateCompleted: "completed", a2a.TaskStateFailed: "failed", a2a.TaskStateRejected: "rejected", a2a.TaskStateCanceled: "canceled"}
	state, ok := states[status.State]
	if !ok || a.progress.TaskID == "" {
		return ErrRemoteProtocol
	}
	if agentstate.RemoteTerminal(a.progress.State) && state != a.progress.State {
		return ErrRemoteProtocol
	}
	if status.Message != nil {
		text, err := remoteMessageText(status.Message)
		if err != nil {
			return err
		}
		a.progress.Text = text
	}
	a.progress.State = state
	// ADK's artifact-per-run mode seals its final artifact with the terminal
	// task status, without a separate LastChunk update. Interrupted streams
	// never reach this transition and retain their incomplete flags.
	if state == "completed" {
		for i := range a.progress.Artifacts {
			a.progress.Artifacts[i].Complete = true
		}
	}
	return nil
}

func remoteUsageMetadata(event a2a.Event) map[string]any {
	metadata := event.Meta()
	if _, exists := metadata[adka2a.ToA2AMetaKey("usage_metadata")]; exists {
		return metadata
	}
	// Artifact-per-event attaches usage to the artifact; the legacy mode
	// attaches it to the update event. Both remain remote-reported values.
	if update, ok := event.(*a2a.TaskArtifactUpdateEvent); ok && update.Artifact != nil {
		return update.Artifact.Metadata
	}
	return metadata
}

func (a *remoteAccumulator) artifact(value *a2a.Artifact, appendText, complete bool) error {
	if value == nil || !remoteIdentity(string(value.ID)) || len(value.Name) > 256 || len(value.Extensions) > 0 {
		return ErrRemoteProtocol
	}
	text, err := remotePartsText(value.Parts)
	if err != nil {
		return err
	}
	index := slices.IndexFunc(a.progress.Artifacts, func(item agentstate.RemoteArtifact) bool { return item.ID == string(value.ID) })
	if index < 0 {
		if appendText || len(a.progress.Artifacts) >= 4 {
			return ErrRemoteProtocol
		}
		a.progress.Artifacts = append(a.progress.Artifacts, agentstate.RemoteArtifact{ID: string(value.ID)})
		index = len(a.progress.Artifacts) - 1
	}
	item := &a.progress.Artifacts[index]
	if appendText {
		if item.Complete {
			return ErrRemoteProtocol
		}
		item.Text += text
	} else {
		item.Text = text
	}
	item.Name, item.Complete = value.Name, complete
	return nil
}

func remoteMessageText(message *a2a.Message) (string, error) {
	if message == nil || message.Role != a2a.MessageRoleAgent || len(message.Extensions) > 0 || len(message.ReferenceTasks) > 0 {
		return "", ErrRemoteProtocol
	}
	return remotePartsText(message.Parts)
}

func remotePartsText(parts a2a.ContentParts) (string, error) {
	if len(parts) > 16 {
		return "", ErrRemoteProtocol
	}
	var result strings.Builder
	for _, part := range parts {
		if part == nil || part.Filename != "" || part.MediaType != "" && part.MediaType != "text/plain" {
			return "", ErrRemoteProtocol
		}
		text, ok := part.Content.(a2a.Text)
		if !ok {
			return "", ErrRemoteProtocol
		}
		result.WriteString(string(text))
		if result.Len() > 64<<10 {
			return "", ErrRemoteProtocol
		}
	}
	return result.String(), nil
}

func (a *remoteAccumulator) usage(metadata map[string]any) error {
	value, exists := metadata[adka2a.ToA2AMetaKey("usage_metadata")]
	if !exists {
		return nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ErrRemoteProtocol
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil {
		return ErrRemoteProtocol
	}
	if fields["promptTokenCount"] == nil || fields["candidatesTokenCount"] == nil {
		return nil
	}
	for _, key := range []string{"promptTokenCount", "candidatesTokenCount", "thoughtsTokenCount"} {
		if string(fields[key]) == "null" {
			return ErrRemoteProtocol
		}
	}
	var usage genai.GenerateContentResponseUsageMetadata
	if err := json.Unmarshal(data, &usage); err != nil {
		return ErrRemoteProtocol
	}
	if usage.PromptTokenCount < 0 || usage.CandidatesTokenCount < 0 || usage.ThoughtsTokenCount < 0 {
		return fmt.Errorf("%w: invalid reported remote usage", ErrRemoteProtocol)
	}
	a.progress.Usage = agentstate.RemoteUsage{InputTokens: int64(usage.PromptTokenCount), OutputTokens: int64(usage.CandidatesTokenCount) + int64(usage.ThoughtsTokenCount), Source: "remote_reported"}
	return nil
}
