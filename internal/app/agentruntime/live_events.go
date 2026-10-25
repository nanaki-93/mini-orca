package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

func (b *liveBridge) readMessage() (*genai.LiveServerMessage, error) {
	kind, data, err := b.wire.conn.ReadMessage()
	if err != nil {
		return nil, liveIOError(b.ctx, err)
	}
	b.mu.Lock()
	b.snapshot.ReceivedBytes += int64(len(data))
	b.frames++
	exceeded := b.snapshot.ReceivedBytes > b.outputLimit || b.frames > 256
	b.mu.Unlock()
	if exceeded {
		return nil, &BudgetError{Limit: "live incoming events"}
	}
	if kind != websocket.TextMessage || !jsontext.Value(data).IsValid() {
		return nil, ErrLiveProtocol
	}
	var message genai.LiveServerMessage
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&message) != nil {
		return nil, ErrLiveProtocol
	}
	if err := validateLiveEnvelope(&message); err != nil {
		return nil, err
	}
	return &message, nil
}

func validateLiveEnvelope(message *genai.LiveServerMessage) error {
	if message.ToolCall != nil || message.ToolCallCancellation != nil {
		return ErrPolicyDenied
	}
	if message.SessionResumptionUpdate != nil {
		return ErrLiveProtocol
	}
	kinds := 0
	for _, present := range []bool{message.SetupComplete != nil, message.ServerContent != nil, message.GoAway != nil, message.VoiceActivity != nil, message.VoiceActivityDetectionSignal != nil} {
		if present {
			kinds++
		}
	}
	if kinds > 1 || kinds == 0 && message.UsageMetadata == nil {
		return ErrLiveProtocol
	}
	return nil
}

func (b *liveBridge) events(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		for {
			if err := b.client.checkPolicy(ctx); err != nil {
				yield(nil, err)
				return
			}
			message, err := b.readMessage()
			if err != nil {
				yield(nil, err)
				return
			}
			if err := b.client.checkPolicy(ctx); err != nil {
				yield(nil, err)
				return
			}
			event, err := b.projectMessage(ctx, message)
			if err != nil {
				yield(nil, err)
				return
			}
			if !yield(event, nil) {
				return
			}
		}
	}
}

func (b *liveBridge) projectMessage(ctx agent.InvocationContext, message *genai.LiveServerMessage) (*session.Event, error) {
	if err := b.observeLiveUsage(message.UsageMetadata); err != nil {
		return nil, err
	}
	if message.SetupComplete != nil {
		return nil, ErrLiveProtocol
	}
	if message.GoAway != nil {
		return nil, ErrLiveGoAway
	}
	text, audio, err := liveContent(message.ServerContent)
	if err != nil {
		return nil, err
	}
	if err := b.retainOutput(text, audio, message.ServerContent); err != nil {
		return nil, err
	}
	event := session.NewEvent(ctx, ctx.InvocationID())
	event.Author = b.Name()
	event.LLMResponse = model.LLMResponse{}
	if text != "" {
		event.Content = genai.NewContentFromText(text, genai.RoleModel)
	}
	if message.ServerContent != nil {
		event.TurnComplete = message.ServerContent.TurnComplete
	}
	return event, nil
}

func liveContent(content *genai.LiveServerContent) (string, []byte, error) {
	if content == nil {
		return "", nil, nil
	}
	if content.GroundingMetadata != nil || content.URLContextMetadata != nil {
		return "", nil, ErrPolicyDenied
	}
	var text strings.Builder
	for _, transcription := range []struct {
		label string
		value *genai.Transcription
	}{{"Input transcription: ", content.InputTranscription}, {"Model transcription: ", content.OutputTranscription}} {
		if transcription.value != nil {
			if !liveText(transcription.value.Text) {
				return "", nil, ErrLiveProtocol
			}
			text.WriteString(transcription.label + transcription.value.Text + "\n")
		}
	}
	audio, err := liveModelTurn(content.ModelTurn, &text)
	if err != nil {
		return "", nil, err
	}
	if content.Interrupted {
		text.WriteString("Provider reported an interrupted response.\n")
	}
	if content.TurnComplete {
		text.WriteString("Provider reported end of turn.\n")
	}
	return text.String(), audio, nil
}

func liveModelTurn(content *genai.Content, text *strings.Builder) ([]byte, error) {
	if content == nil {
		return nil, nil
	}
	if content.Role != genai.RoleModel || len(content.Parts) > 32 || onlyModelFields(content, "role", "parts") != nil {
		return nil, ErrLiveProtocol
	}
	var audio []byte
	for _, part := range content.Parts {
		if part == nil || onlyModelFields(part, "text", "inlineData", "thought", "thoughtSignature") != nil {
			return nil, ErrPolicyDenied
		}
		if part.Thought {
			continue
		}
		if !liveText(part.Text) {
			return nil, ErrLiveProtocol
		}
		if part.Text != "" {
			text.WriteString("Model: " + part.Text + "\n")
		}
		if part.InlineData != nil {
			blob := part.InlineData
			if !validLiveAudio(blob) || part.Text != "" {
				return nil, ErrLiveProtocol
			}
			audio = append(audio, blob.Data...)
		}
	}
	return audio, nil
}

func validLiveAudio(blob *genai.Blob) bool {
	return onlyModelFields(blob, "data", "mimeType") == nil && blob.MIMEType == "audio/pcm;rate=24000" && len(blob.Data) > 0 && len(blob.Data)%2 == 0
}

func liveText(value string) bool {
	return len(value) <= 64<<10 && utf8.ValidString(value) && !strings.ContainsRune(value, 0)
}

func (b *liveBridge) retainOutput(text string, audio []byte, content *genai.LiveServerContent) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.transcript)+len(text) > 64<<10 || int64(len(b.audio)+len(audio)) > b.outputLimit {
		return &BudgetError{Limit: "live retained output"}
	}
	b.transcript += text
	b.audio = append(b.audio, audio...)
	if content != nil && content.TurnComplete {
		b.snapshot.CompletedTurns++
	}
	return nil
}

func (b *liveBridge) observeLiveUsage(usage *genai.UsageMetadata) error {
	if usage == nil {
		return nil
	}
	receipt := tokenReceipt{Input: int64(usage.PromptTokenCount) + int64(usage.ToolUsePromptTokenCount), Output: int64(usage.ResponseTokenCount) + int64(usage.ThoughtsTokenCount), Cached: int64(usage.CachedContentTokenCount), CacheKnown: true}
	if !receipt.valid() || usage.PromptTokenCount < 0 || usage.ToolUsePromptTokenCount != 0 || usage.ResponseTokenCount < 0 || usage.ThoughtsTokenCount < 0 || int64(usage.TotalTokenCount) < receipt.Input+receipt.Output {
		return ErrLiveProtocol
	}
	if err := b.reservation.observeUsage(receipt); err != nil {
		return err
	}
	b.mu.Lock()
	b.snapshot.Reported = true
	b.snapshot.ReportedInput = receipt.Input
	b.snapshot.ReportedOutput = receipt.Output
	b.mu.Unlock()
	if receipt.Input > b.reservation.input || receipt.Output > b.reservation.output {
		return &BudgetError{Limit: "live reported tokens"}
	}
	return nil
}

func (b *liveBridge) finish(result error) error {
	if b.wire != nil {
		b.wire.close()
	}
	cleanup, stop := context.WithTimeout(context.WithoutCancel(b.ctx), 5*time.Second)
	defer stop()
	state := liveEndState(result)
	persistErr := b.saveLiveArtifacts(cleanup, state)
	settleErr := b.reservation.settle(0, 0, false, errors.Is(result, ErrLiveClosed))
	finishResult := result
	if errors.Is(result, ErrLiveClosed) {
		finishResult = nil
	}
	finishErr := b.invocation.finish(cleanup, errors.Join(finishResult, persistErr, settleErr))
	combined := errors.Join(persistErr, settleErr, finishErr)
	if combined != nil {
		state = "failed"
	}
	b.mu.Lock()
	b.snapshot.State = state
	b.mu.Unlock()
	if combined != nil || errors.Is(result, ErrPolicyDenied) || errors.Is(result, ErrBudgetExceeded) {
		CancelRun(b.ctx, errors.Join(result, combined))
	}
	return combined
}

func (b *liveBridge) saveLiveArtifacts(ctx context.Context, state string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.transcript == "" && len(b.audio) == 0 {
		return nil
	}
	service := completionArtifacts(b.invocation)
	text := fmt.Sprintf("LIVE MODEL ADVICE — not verified project evidence\nConnection: %s\nFinal usage is unavailable; charges remain conservative.\n\n%s", state, b.transcript)
	if err := b.saveLiveArtifact(ctx, service, "live-transcript.txt", &genai.Part{Text: text}, state); err != nil {
		return err
	}
	if len(b.audio) > 0 {
		return b.saveLiveArtifact(ctx, service, "live-audio.pcm", &genai.Part{InlineData: &genai.Blob{MIMEType: "audio/pcm;rate=24000", Data: b.audio}}, state)
	}
	return nil
}

func (b *liveBridge) saveLiveArtifact(ctx context.Context, service artifact.Service, name string, part *genai.Part, state string) error {
	saved, err := service.Save(ctx, &artifact.SaveRequest{AppName: "mini-orca", UserID: "local", SessionID: b.invocation.session.ID(), FileName: name, Part: part, CustomMetadata: map[string]any{"kind": "live-advice-v1", "state": state}})
	if err != nil {
		return fmt.Errorf("%w: retain live output: %w", agentstate.ErrPersistence, err)
	}
	b.snapshot.Artifacts = append(b.snapshot.Artifacts, agentstate.ArtifactReference{Session: b.snapshot.Session, Name: name, Version: saved.Version})
	return nil
}
