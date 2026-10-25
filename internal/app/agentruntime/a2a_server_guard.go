package agentruntime

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"io"
	"net/http"
	"regexp"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv"
)

var serverIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type serverEnvelope struct {
	Version string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type serverSend struct {
	Message *struct {
		ID    string `json:"messageId"`
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"message"`
}

func guardServerRequest(w http.ResponseWriter, r *http.Request, inputBytes int) error {
	if version := r.Header.Get("A2A-Version"); version != "" && version != "1.0" {
		return a2a.ErrInvalidRequest
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, int64(inputBytes)*2))
	if err != nil {
		return err
	}
	var envelope serverEnvelope
	if strictServerJSON(body, &envelope) != nil || envelope.Version != "2.0" || !serverIdentity.MatchString(envelope.ID) {
		return a2a.ErrInvalidRequest
	}
	if err := validateServerParams(envelope, inputBytes); err != nil {
		return err
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}

func strictServerJSON(data []byte, value any) error {
	if !jsontext.Value(data).IsValid() {
		return a2a.ErrInvalidParams
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func validateServerParams(envelope serverEnvelope, inputBytes int) error {
	switch envelope.Method {
	case "SendMessage", "SendStreamingMessage":
		return validateServerSend(envelope.Params, inputBytes)
	case "GetTask":
		var request a2a.GetTaskRequest
		if strictServerJSON(envelope.Params, &request) != nil || request.Tenant != "" || !serverIdentity.MatchString(string(request.ID)) {
			return a2a.ErrInvalidParams
		}
		if request.HistoryLength != nil && (*request.HistoryLength < 0 || *request.HistoryLength > 1) {
			return a2a.ErrInvalidParams
		}
		return nil
	case "CancelTask", "SubscribeToTask":
		var request struct {
			ID string `json:"id"`
		}
		if strictServerJSON(envelope.Params, &request) != nil || !serverIdentity.MatchString(request.ID) {
			return a2a.ErrInvalidParams
		}
		return nil
	default:
		return a2a.ErrUnsupportedOperation
	}
}

func validateServerSend(data []byte, inputBytes int) error {
	var request serverSend
	if strictServerJSON(data, &request) != nil || request.Message == nil {
		return a2a.ErrInvalidParams
	}
	msg := request.Message
	if !serverIdentity.MatchString(msg.ID) || msg.Role != "ROLE_USER" || len(msg.Parts) != 1 || len(msg.Parts[0].Text) == 0 || len(msg.Parts[0].Text) > inputBytes {
		return a2a.ErrInvalidParams
	}
	return nil
}

type serverInterceptor struct{ s *AgentServer }

func (i serverInterceptor) Before(ctx context.Context, call *a2asrv.CallContext, request *a2asrv.Request) (context.Context, any, error) {
	scope, ok := ctx.Value(serverScopeKey{}).(serverScope)
	if !ok || scope.admission.Validate(ctx) != nil {
		return ctx, nil, a2a.ErrUnauthenticated
	}
	call.User = a2asrv.NewAuthenticatedUser(scope.admission.Owner, nil)
	var taskID a2a.TaskID
	switch value := request.Payload.(type) {
	case *a2a.SendMessageRequest:
		return ctx, nil, i.s.reserveMessage(value.Message.ID)
	case *a2a.GetTaskRequest:
		taskID = value.ID
	case *a2a.CancelTaskRequest:
		taskID = value.ID
	case *a2a.SubscribeToTaskRequest:
		taskID = value.ID
	default:
		return ctx, nil, a2a.ErrUnsupportedOperation
	}
	// Active subscriptions and cancellation must pass ownership admission before
	// the SDK execution manager can attach to a running task.
	_, err := i.s.store.Get(ctx, taskID)
	if err != nil {
		return ctx, nil, a2a.ErrTaskNotFound
	}
	return ctx, nil, nil
}

func (i serverInterceptor) After(ctx context.Context, _ *a2asrv.CallContext, response *a2asrv.Response) error {
	scope, ok := ctx.Value(serverScopeKey{}).(serverScope)
	if !ok || scope.admission.Validate(ctx) != nil {
		response.Payload, response.Err = nil, a2a.ErrUnauthenticated
	}
	if response.Err == nil {
		return nil
	}
	for _, allowed := range []error{a2a.ErrTaskNotFound, a2a.ErrTaskNotCancelable, a2a.ErrUnauthenticated, a2a.ErrInvalidParams, a2a.ErrUnsupportedOperation} {
		if errors.Is(response.Err, allowed) {
			response.Err = allowed
			return nil
		}
	}
	response.Err = a2a.ErrInternalError
	return nil
}

func (s *AgentServer) reserveMessage(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.messages[id] || len(s.messages) >= 128 {
		return a2a.ErrUnsupportedOperation
	}
	// Failed or disconnected sends remain reserved: a transport retry must not
	// prepare another proposal with an uncertain original outcome.
	s.messages[id] = true
	return nil
}
