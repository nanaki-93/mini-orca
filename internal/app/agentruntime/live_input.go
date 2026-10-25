package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"google.golang.org/adk/v2/agent"
	"google.golang.org/genai"
)

type LiveInput struct {
	Text  string
	Media *llm.Attachment
}
type liveInterrupt struct{}

func ValidateLiveInput(input LiveInput) error {
	if input.Media == nil {
		if strings.TrimSpace(input.Text) == "" || len(input.Text) > 2048 || !utf8.ValidString(input.Text) || strings.ContainsRune(input.Text, 0) {
			return fmt.Errorf("live text must contain 1–2048 UTF-8 bytes")
		}
		return nil
	}
	if input.Text != "" || len(input.Media.Data) > 64<<10 {
		return fmt.Errorf("send one media item up to 64 KiB separately from text")
	}
	if input.Media.MIME != "image/jpeg" && input.Media.MIME != llm.PCM16MIME {
		return fmt.Errorf("live media accepts JPEG or raw 16 kHz mono PCM; PDF, PNG and WAV are not converted")
	}
	return input.Media.Validate()
}

func (c *LiveConnection) Send(ctx context.Context, input LiveInput, authorize func(context.Context) error) error {
	if err := ValidateLiveInput(input); err != nil {
		return err
	}
	request := agent.LiveRequest{}
	if input.Media != nil {
		media := cloneAttachments([]llm.Attachment{*input.Media})[0]
		request.RealtimeInput = &genai.Blob{Data: media.Data, MIMEType: media.MIME, DisplayName: media.Name}
	} else {
		request.Content = genai.NewContentFromText(input.Text, genai.RoleUser)
	}
	return c.send(ctx, request, authorize)
}

func (c *LiveConnection) Interrupt(ctx context.Context, authorize func(context.Context) error) error {
	return c.send(ctx, agent.LiveRequest{RealtimeInput: liveInterrupt{}}, authorize)
}

func (c *LiveConnection) send(ctx context.Context, request agent.LiveRequest, authorize func(context.Context) error) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	if authorize == nil {
		return ErrPolicyDenied
	}
	if err := context.Cause(ctx); err != nil {
		c.cancel(err)
		return err
	}
	if err := context.Cause(c.ctx); err != nil {
		return err
	}
	c.bridge.sendCtx, c.bridge.sendAuthority = ctx, authorize
	stop := context.AfterFunc(ctx, func() { c.cancel(context.Cause(ctx)) })
	err := c.session.Send(request)
	stop()
	if err != nil {
		c.cancel(err)
	}
	return err
}

func liveInputFrames(request agent.LiveRequest) ([]any, string, error) {
	if request.Content != nil {
		content := request.Content
		if content.Role != genai.RoleUser || len(content.Parts) != 1 || content.Parts[0] == nil || onlyModelFields(content.Parts[0], "text") != nil {
			return nil, "", ErrPolicyDenied
		}
		if err := ValidateLiveInput(LiveInput{Text: content.Parts[0].Text}); err != nil {
			return nil, "", err
		}
		return []any{genai.LiveClientMessage{ClientContent: &genai.LiveClientContent{Turns: []*genai.Content{content}, TurnComplete: true}}}, "User input attempted: " + content.Parts[0].Text + "\n", nil
	}
	if _, ok := request.RealtimeInput.(liveInterrupt); ok {
		return []any{genai.LiveClientMessage{ClientContent: &genai.LiveClientContent{TurnComplete: true}}}, "User requested interruption.\n", nil
	}
	blob, ok := request.RealtimeInput.(*genai.Blob)
	if !ok || blob == nil {
		return nil, "", ErrPolicyDenied
	}
	media := llm.Attachment{Name: blob.DisplayName, MIME: blob.MIMEType, Data: blob.Data}
	if err := ValidateLiveInput(LiveInput{Media: &media}); err != nil {
		return nil, "", err
	}
	field := "audio"
	if media.MIME == "image/jpeg" {
		field = "video"
	}
	frames := []any{
		map[string]any{"realtimeInput": map[string]any{"activityStart": map[string]any{}}},
		map[string]any{"realtimeInput": map[string]any{field: map[string]any{"data": blob.Data, "mimeType": blob.MIMEType}}},
		map[string]any{"realtimeInput": map[string]any{"activityEnd": map[string]any{}}},
	}
	manifest := attachmentManifest([]llm.Attachment{media})[0]
	return frames, fmt.Sprintf("Media input attempted: %s (%s, %d bytes, SHA-256 %s)\n", manifest.Name, manifest.MIME, manifest.Size, manifest.Hash), nil
}

func (b *liveBridge) Send(request agent.LiveRequest) error {
	if err := b.client.checkPolicy(b.ctx); err != nil {
		return err
	}
	if err := b.sendAuthority(b.sendCtx); err != nil {
		return err
	}
	frames, note, err := liveInputFrames(request)
	if err != nil {
		return err
	}
	encoded := make([][]byte, 0, len(frames))
	total := 0
	for _, frame := range frames {
		data, err := json.Marshal(frame)
		if err != nil {
			return fmt.Errorf("encode live input: %w", err)
		}
		encoded = append(encoded, data)
		total += len(data)
	}
	if err := b.admitInput(total, note); err != nil {
		return err
	}
	for _, data := range encoded {
		if err := b.client.checkPolicy(b.ctx); err != nil {
			return err
		}
		if err := b.sendAuthority(b.sendCtx); err != nil {
			return err
		}
		deadline := time.Now().Add(5 * time.Second)
		if end, ok := b.sendCtx.Deadline(); ok && end.Before(deadline) {
			deadline = end
		}
		if err := b.wire.conn.SetWriteDeadline(deadline); err != nil {
			return liveIOError(b.ctx, err)
		}
		if err := b.wire.conn.WriteMessage(1, data); err != nil {
			return liveIOError(b.ctx, err)
		}
	}
	return nil
}

func (b *liveBridge) admitInput(size int, note string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.snapshot.Turns >= 8 {
		return &BudgetError{Limit: "live input turns"}
	}
	if int64(size) > b.inputLimit-b.snapshot.SentBytes {
		return &BudgetError{Limit: "live outgoing bytes"}
	}
	if len(b.transcript)+len(note) > 64<<10 {
		return &BudgetError{Limit: "live transcript bytes"}
	}
	b.snapshot.Turns++
	b.snapshot.SentBytes += int64(size)
	b.transcript += note
	return nil
}
