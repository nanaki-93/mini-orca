package agentruntime

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func multimodalFixture() ([]llm.ChatMessage, []llm.Attachment) {
	return []llm.ChatMessage{{Role: "system", Content: "Explicit advisory context."}, {Role: "user", Content: "PRIVATE_QUESTION"}}, []llm.Attachment{{Name: "brief.pdf", MIME: "application/pdf", Data: []byte("%PDF-1.7\n1 0 obj\nPRIVATE_MEDIA\nendobj\n%%EOF\n")}}
}

func nativeMediaFixtures(t *testing.T) []llm.Attachment {
	t.Helper()
	var pngData, jpegData bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if err := png.Encode(&pngData, picture); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, picture, nil); err != nil {
		t.Fatal(err)
	}
	wav := make([]byte, 48)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], 40)
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 16000)
	binary.LittleEndian.PutUint32(wav[28:], 32000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], 4)
	return []llm.Attachment{{Name: "diagram.png", MIME: "image/png", Data: pngData.Bytes()}, {Name: "photo.jpg", MIME: "image/jpeg", Data: jpegData.Bytes()}, {Name: "clip.wav", MIME: "audio/wav", Data: wav}}
}

func TestMultimodalImageAndAudioConformanceAcrossNativeBackends(t *testing.T) {
	for _, backend := range []config.ModelProvider{config.GeminiProvider, config.VertexProvider} {
		for _, media := range nativeMediaFixtures(t) {
			t.Run(string(backend)+"/"+media.MIME, func(t *testing.T) {
				var calls atomic.Int32
				client := googleFixture(t, func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if !strings.HasSuffix(r.URL.Path, "/gemini-2.5-flash:generateContent") {
						t.Error("unexpected native route")
					}
					var input struct {
						Contents []*genai.Content `json:"contents"`
					}
					if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
						t.Error(err)
						return
					}
					if len(input.Contents) != 1 || len(input.Contents[0].Parts) != 2 {
						t.Error("missing media")
						return
					}
					part := input.Contents[0].Parts[1]
					if part.FileData != nil || part.InlineData == nil || part.InlineData.MIMEType != media.MIME || !bytes.Equal(part.InlineData.Data, media.Data) {
						t.Error("media transformed or remotely fetched")
					}
					io.WriteString(w, googleTextResponse)
				})
				client.profile.Model, client.profile.Provider = "gemini-2.5-flash", backend
				ctx, _ := budgetContext(t, nil)
				messages, _ := multimodalFixture()
				if result, err := MultimodalAdvice(ctx, client, messages, []llm.Attachment{media}, func(context.Context) error { return nil }); err != nil || result.Content != "complete" || calls.Load() != 1 {
					t.Fatalf("native media: %+v %v calls=%d", result, err, calls.Load())
				}
			})
		}
	}
}

func TestMultimodalUsesNativeADKWithImmutableMediaAndPrivateManifest(t *testing.T) {
	messages, attachments := multimodalFixture()
	want := slices.Clone(attachments[0].Data)
	var calls atomic.Int32
	client := googleFixture(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/v1beta/models/gemini-2.5-flash:generateContent" || r.Header.Get("X-Goog-Api-Key") != "configured-key" {
			t.Error("native destination or credential changed")
		}
		var input struct {
			Contents []*genai.Content `json:"contents"`
			Tools    []any            `json:"tools"`
			Cached   string           `json:"cachedContent"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if len(input.Contents) != 1 || len(input.Contents[0].Parts) != 2 || len(input.Tools) != 0 || input.Cached != "" {
			t.Errorf("unexpected multimodal request: %+v", input)
			return
		}
		media := input.Contents[0].Parts[1].InlineData
		if media == nil || !bytes.Equal(media.Data, want) || media.MIMEType != "application/pdf" || input.Contents[0].Parts[0].Text != "PRIVATE_QUESTION" {
			t.Error("captured input was replaced")
		}
		io.WriteString(w, googleTextResponse)
	})
	client.profile.Model = "gemini-2.5-flash"
	ctx, budget, store := persistedContext(t, t.TempDir())
	ctx, err := WithPolicy(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = WithContextConfig(ctx, config.AgentContext{Compaction: true, LocalCache: true, ProviderCache: true, ProviderTTLSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	checks := 0
	result, err := MultimodalAdvice(ctx, client, messages, attachments, func(context.Context) error {
		checks++
		if checks == 2 {
			attachments[0].Data[10] = 'x'
			messages[1].Content = "replaced"
		}
		return nil
	})
	if err != nil || result.Content != "complete" || !result.UsageKnown || calls.Load() != 1 || checks < 3 {
		t.Fatalf("advice=%+v checks=%d calls=%d err=%v", result, checks, calls.Load(), err)
	}
	if usage := budget.Snapshot(); usage.ModelCalls != 1 || usage.ToolCalls != 0 || usage.ActiveCalls != 0 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
	sessions, err := store.Sessions(t.Context(), "mini-orca", "local")
	if err != nil || len(sessions) != 1 {
		t.Fatalf("private sessions: %v %v", sessions, err)
	}
	data, _ := json.Marshal(sessions[0].State)
	if !bytes.Contains(data, []byte("mini_orca.attachments")) || bytes.Contains(data, []byte("PRIVATE_MEDIA")) || bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString(want))) {
		t.Fatal("binary bytes escaped the artifact-owned manifest")
	}
	history, err := store.History(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	data, _ = json.Marshal(history)
	if bytes.Contains(data, []byte("PRIVATE_QUESTION")) || bytes.Contains(data, []byte("PRIVATE_MEDIA")) {
		t.Fatal("private content escaped into run metadata")
	}
	if len(history.Runs) != 1 || history.Runs[0].SchemaVersion != "multimodal-v1" {
		t.Fatalf("missing multimodal identity: %+v", history.Runs)
	}
}

func TestMultimodalRejectsUnsupportedAndRevokedAdmissionBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	for _, kind := range []string{"provider", "model", "authority", "budget", "destination", "revoked", "empty question", "many media", "PCM", "URL", "token budget"} {
		t.Run(kind, func(t *testing.T) {
			client := googleFixture(t, func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); io.WriteString(w, googleTextResponse) })
			client.profile.Model = "gemini-2.5-flash"
			messages, media := multimodalFixture()
			ctx, _ := budgetContext(t, func(l *config.AgentLimits) {
				if kind == "token budget" {
					l.InputTokens = 1
				}
			})
			authorize := func(context.Context) error { return nil }
			switch kind {
			case "provider":
				client.profile.Provider = config.OpenAIProvider
			case "model":
				client.profile.Model = "unknown"
			case "authority":
				authorize = nil
			case "budget":
				ctx = t.Context()
			case "destination":
				var err error
				ctx, err = WithPolicy(ctx, NewProviderClient(config.ModelProfile{Provider: config.GeminiProvider, Model: "another"}))
				if err != nil {
					t.Fatal(err)
				}
			case "revoked":
				checks := 0
				authorize = func(context.Context) error {
					checks++
					if checks > 1 {
						return errors.New("scope revoked")
					}
					return nil
				}
			case "empty question":
				messages[1].Content = " "
			case "many media":
				media = []llm.Attachment{media[0], media[0], media[0], media[0], media[0]}
			case "PCM":
				media[0] = llm.Attachment{Name: "input.pcm", MIME: llm.PCM16MIME, Data: []byte{0, 0}}
			case "URL":
				media[0].Name = "https://private.example/media"
			}
			if result, err := MultimodalAdvice(ctx, client, messages, media, authorize); err == nil || result != (Result{}) {
				t.Fatalf("invalid admission succeeded: %+v %v", result, err)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid requests dispatched %d calls", calls.Load())
	}
}

func TestMultimodalNativePartsRemainExplicitAndUserOwned(t *testing.T) {
	_, items := multimodalFixture()
	part := &genai.Part{InlineData: &genai.Blob{MIMEType: items[0].MIME, Data: items[0].Data, DisplayName: items[0].Name}}
	for _, kind := range []string{"ordinary completion", "model role", "mixed", "provider URL", "schema role"} {
		copy := *part
		input := &model.LLMRequest{Contents: []*genai.Content{{Role: genai.RoleUser, Parts: []*genai.Part{&copy}}}}
		enabled := true
		switch kind {
		case "ordinary completion":
			enabled = false
		case "model role":
			input.Contents[0].Role = genai.RoleModel
		case "mixed":
			copy.Text = "hidden"
		case "provider URL":
			copy = genai.Part{FileData: &genai.FileData{FileURI: "https://private.example/media"}}
		case "schema role":
			input.Contents[0].Role = "system"
		}
		if validateNativeContent(input.Contents, enabled) == nil {
			t.Fatalf("unsupported %s accepted", kind)
		}
	}
}

func TestMultimodalCancellationClosesTheProviderAndRetainsUncertainCharge(t *testing.T) {
	entered := make(chan struct{})
	client := googleFixture(t, func(_ http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(entered)
		<-r.Context().Done()
	})
	client.profile.Model = "gemini-2.5-flash"
	parent, budget := budgetContext(t, nil)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	messages, media := multimodalFixture()
	done := make(chan error, 1)
	go func() {
		_, err := MultimodalAdvice(ctx, client, messages, media, func(context.Context) error { return nil })
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("provider not entered")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not join request")
	}
	usage := budget.Snapshot()
	if usage.ActiveCalls != 0 || usage.ModelCalls != 1 || usage.InputTokens == 0 || usage.OutputTokens == 0 {
		t.Fatalf("uncertain usage lost: %+v", usage)
	}
}

func TestMultimodalDoesNotRetryOrPublishOversizedOutput(t *testing.T) {
	for _, failed := range []bool{false, true} {
		var calls atomic.Int32
		client := googleFixture(t, func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			if failed {
				w.WriteHeader(http.StatusServiceUnavailable)
				io.WriteString(w, "PRIVATE_PROVIDER_BODY")
				return
			}
			io.WriteString(w, strings.Replace(googleTextResponse, "complete", strings.Repeat("x", (64<<10)+1), 1))
		})
		client.profile.Model = "gemini-2.5-flash"
		ctx, _ := budgetContext(t, nil)
		messages, media := multimodalFixture()
		result, err := MultimodalAdvice(ctx, client, messages, media, func(context.Context) error { return nil })
		if err == nil || result != (Result{}) || calls.Load() != 1 || strings.Contains(err.Error(), "PRIVATE_PROVIDER_BODY") {
			t.Fatalf("failed exchange published/retried: %+v %v calls=%d", result, err, calls.Load())
		}
	}
}
