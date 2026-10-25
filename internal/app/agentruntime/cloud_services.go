package agentruntime

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/storage"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/adk/v2/artifact"
	"google.golang.org/adk/v2/artifact/gcsartifact"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/session"
	vertexsession "google.golang.org/adk/v2/session/vertexai"
	"google.golang.org/api/option"
	"google.golang.org/genai"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

var ErrCloudUnavailable = errors.New("cloud operation unavailable; the remote outcome may be retained")

// CloudServices copies explicitly admitted private data. It cannot replace the
// host journal or provide resume/approval authority.
type CloudServices interface {
	Copy(context.Context, agentstate.CloudCopy, []byte) error
	Inspect(context.Context, agentstate.CloudCopy) (bool, error)
	Delete(context.Context, agentstate.CloudCopy) error
	Close() error
}

type cloudServices struct {
	target    config.CloudTarget
	sessions  session.Service
	artifacts artifact.Service
	close     func() error
}

func cloudError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if cause := context.Cause(ctx); cause != nil {
		return cause
	}
	// SDK errors may echo credentials, object contents or source-bearing URLs.
	return ErrCloudUnavailable
}

func cloudCredential(ctx context.Context, target config.CloudTarget) (oauth2.TokenSource, error) {
	path := os.Getenv(target.CredentialFileEnv)
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrCloudUnavailable
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, ErrCloudUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err != nil || len(data) > 64<<10 {
		return nil, ErrCloudUnavailable
	}
	var identity struct {
		Type     string `json:"type"`
		Email    string `json:"client_email"`
		TokenURI string `json:"token_uri"`
		Universe string `json:"universe_domain"`
	}
	if json.Unmarshal(data, &identity) != nil || identity.Type != "service_account" || identity.Email != target.Principal || identity.TokenURI != "https://oauth2.googleapis.com/token" || identity.Universe != "" && identity.Universe != "googleapis.com" {
		return nil, ErrCloudUnavailable
	}
	jwt, err := google.JWTConfigFromJSON(data, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, ErrCloudUnavailable
	}
	return jwt.TokenSource(ctx), nil
}

func OpenCloudServices(ctx context.Context, target config.CloudTarget) (CloudServices, error) {
	if err := target.Validate(); err != nil {
		return nil, err
	}
	if err := context.Cause(ctx); err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout, transport.MaxResponseHeaderBytes = 15*time.Second, 64<<10
	authClient := &http.Client{Transport: transport, Timeout: time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, authClient)
	tokens, err := cloudCredential(ctx, target)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	if target.Service == "vertex_session" {
		conn, err := grpc.NewClient(target.Region+"-aiplatform.googleapis.com:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})), grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: tokens}), grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(6<<20), grpc.MaxCallSendMsgSize(6<<20)), grpc.WithUnaryInterceptor(cloudRPCLimit()))
		if err != nil {
			transport.CloseIdleConnections()
			return nil, cloudError(ctx, err)
		}
		close := func() error { transport.CloseIdleConnections(); return conn.Close() }
		return newCloudServices(ctx, target, close, option.WithGRPCConn(conn))
	}
	client := &http.Client{Transport: &cloudTransport{base: &oauth2.Transport{Source: tokens, Base: transport}}, Timeout: time.Minute, CheckRedirect: authClient.CheckRedirect}
	if err := checkCloudBucket(ctx, target, client); err != nil {
		transport.CloseIdleConnections()
		return nil, cloudError(ctx, err)
	}
	close := func() error { transport.CloseIdleConnections(); return nil }
	return newCloudServices(ctx, target, close, option.WithHTTPClient(client), option.WithEndpoint("https://storage.googleapis.com/storage/v1/"))
}

func newCloudServices(ctx context.Context, target config.CloudTarget, close func() error, opts ...option.ClientOption) (CloudServices, error) {
	s := &cloudServices{target: target, close: close}
	var err error
	if target.Service == "vertex_session" {
		s.sessions, err = vertexsession.NewSessionService(ctx, vertexsession.VertexAIServiceConfig{ProjectID: target.Project, Location: target.Region, ReasoningEngine: target.Resource}, opts...)
	} else {
		// Authentication belongs to the admitted HTTP transport. Suppress SDK
		// credential discovery and its separate background metrics exporter.
		opts = append(opts, option.WithoutAuthentication(), storage.WithDisabledClientMetrics())
		s.artifacts, err = gcsartifact.NewService(ctx, target.Resource, opts...)
	}
	if err != nil {
		return nil, cloudError(ctx, errors.Join(err, close()))
	}
	return s, nil
}

func (s *cloudServices) Close() error { return cloudError(context.Background(), s.close()) }

func cloudRPCLimit() grpc.UnaryClientInterceptor {
	var calls atomic.Int64
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoke grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if calls.Add(1) > 16 {
			return ErrCloudUnavailable
		}
		return invoke(ctx, method, req, reply, cc, opts...)
	}
}

type cloudTransport struct {
	base  http.RoundTripper
	calls atomic.Int64
}

func (t *cloudTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" || req.URL.Host != "storage.googleapis.com" || t.calls.Add(1) > 64 {
		return nil, ErrCloudUnavailable
	}
	res, err := t.base.RoundTrip(req)
	if err == nil {
		res.Body = http.MaxBytesReader(nil, res.Body, 6<<20)
	}
	return res, err
}

func checkCloudBucket(ctx context.Context, target config.CloudTarget, client *http.Client) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://storage.googleapis.com/storage/v1/b/"+target.Resource+"?fields=location,projectNumber", nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var attrs struct {
		Location      string      `json:"location"`
		ProjectNumber json.Number `json:"projectNumber"`
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, (16<<10)+1))
	if err != nil || res.StatusCode != http.StatusOK || len(data) > 16<<10 || json.Unmarshal(data, &attrs) != nil {
		return ErrCloudUnavailable
	}
	if !strings.EqualFold(attrs.Location, target.Region) || attrs.ProjectNumber.String() != target.Project {
		return fmt.Errorf("configured bucket location or owner does not match")
	}
	return nil
}

func cloudUser(r agentstate.CloudCopy) string {
	h := sha256.Sum256([]byte(r.ProjectID + "\x00" + r.Target.Principal))
	return hex.EncodeToString(h[:])
}

func cloudPayloadMatches(r agentstate.CloudCopy, data []byte) bool {
	h := sha256.Sum256(data)
	return len(data) == r.Size && hex.EncodeToString(h[:]) == r.Hash
}

func (s *cloudServices) admit(r agentstate.CloudCopy) error {
	if r.Target != s.target || !strings.HasPrefix(r.ID, "mo-") || len(r.ID) != 35 || strings.ContainsAny(r.ID, "/\\") || r.Size < 1 || r.Size > 4<<20 {
		return ErrPolicyDenied
	}
	_, err := hex.DecodeString(strings.TrimPrefix(r.ID, "mo-"))
	return err
}

func (s *cloudServices) Copy(ctx context.Context, r agentstate.CloudCopy, data []byte) error {
	if err := s.admit(r); err != nil {
		return err
	}
	if !cloudPayloadMatches(r, data) {
		return ErrPolicyDenied
	}
	part := &genai.Part{InlineData: &genai.Blob{MIMEType: "application/json", Data: data}}
	if s.artifacts != nil {
		res, err := s.artifacts.Save(ctx, &artifact.SaveRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID, FileName: "copy.json", Part: part, CustomMetadata: map[string]any{"sha256": r.Hash, "cleanup_at": r.CleanupAt.Format(time.RFC3339)}})
		if err == nil && (res == nil || res.Version != 1) {
			err = ErrPolicyDenied
		}
		return cloudError(ctx, err)
	}
	created, err := s.sessions.Create(ctx, &session.CreateRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID, State: map[string]any{"mini_orca_archive": r.Hash, "cleanup_at": r.CleanupAt.Format(time.RFC3339)}})
	if err != nil {
		return cloudError(ctx, err)
	}
	if created == nil || created.Session == nil || created.Session.ID() != r.ID || created.Session.UserID() != cloudUser(r) {
		return ErrCloudUnavailable
	}
	event := &session.Event{ID: "archive-" + r.ID, InvocationID: "archive-" + r.ID, Author: "mini_orca_archive", Timestamp: r.CreatedAt, LLMResponse: model.LLMResponse{Content: &genai.Content{Role: "user", Parts: []*genai.Part{part}}, TurnComplete: true}}
	return cloudError(ctx, s.sessions.AppendEvent(ctx, created.Session, event))
}

func (s *cloudServices) Inspect(ctx context.Context, r agentstate.CloudCopy) (bool, error) {
	if err := s.admit(r); err != nil {
		return false, err
	}
	if s.artifacts != nil {
		return s.inspectArtifact(ctx, r)
	}
	response, err := s.sessions.Get(ctx, &session.GetRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID})
	if errors.Is(err, session.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, cloudError(ctx, err)
	}
	if response == nil || response.Session == nil {
		return false, ErrCloudUnavailable
	}
	return inspectCloudSession(r, response.Session)
}

func inspectCloudSession(r agentstate.CloudCopy, view session.Session) (bool, error) {
	hash, err := view.State().Get("mini_orca_archive")
	if err != nil || hash != r.Hash || view.ID() != r.ID || view.UserID() != cloudUser(r) || view.Events().Len() != 1 {
		return false, ErrCloudUnavailable
	}
	event := view.Events().At(0)
	if event == nil || event.Content == nil || len(event.Content.Parts) != 1 || event.InvocationID != "archive-"+r.ID || event.Author != "mini_orca_archive" {
		return false, ErrCloudUnavailable
	}
	return matchingCloudPart(r, event.Content.Parts[0])
}

func matchingCloudPart(r agentstate.CloudCopy, part *genai.Part) (bool, error) {
	if part == nil || part.InlineData == nil || part.InlineData.MIMEType != "application/json" || !cloudPayloadMatches(r, part.InlineData.Data) {
		return false, ErrCloudUnavailable
	}
	return true, nil
}

func (s *cloudServices) inspectArtifact(ctx context.Context, r agentstate.CloudCopy) (bool, error) {
	versions, err := s.artifacts.Versions(ctx, &artifact.VersionsRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID, FileName: "copy.json"})
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, cloudError(ctx, err)
	}
	if versions == nil {
		return false, ErrCloudUnavailable
	}
	if len(versions.Versions) == 0 {
		return false, nil
	}
	if len(versions.Versions) != 1 || versions.Versions[0] != 1 {
		return false, ErrCloudUnavailable
	}
	response, err := s.artifacts.Load(ctx, &artifact.LoadRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID, FileName: "copy.json", Version: 1})
	if err != nil {
		return false, cloudError(ctx, err)
	}
	if response == nil {
		return false, ErrCloudUnavailable
	}
	return matchingCloudPart(r, response.Part)
}

func (s *cloudServices) Delete(ctx context.Context, r agentstate.CloudCopy) error {
	if err := s.admit(r); err != nil {
		return err
	}
	var err error
	if s.artifacts != nil {
		err = s.deleteArtifactCopies(ctx, r)
	} else {
		err = s.sessions.Delete(ctx, &session.DeleteRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID})
	}
	if errors.Is(err, session.ErrNotFound) || errors.Is(err, fs.ErrNotExist) || errors.Is(err, storage.ErrObjectNotExist) {
		err = nil
	}
	return cloudError(ctx, err)
}

func (s *cloudServices) deleteArtifactCopies(ctx context.Context, r agentstate.CloudCopy) error {
	versions, err := s.artifacts.Versions(ctx, &artifact.VersionsRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID, FileName: "copy.json"})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if versions == nil || len(versions.Versions) > 16 {
		return ErrCloudUnavailable
	}
	// A lost GCS write receipt followed by an SDK precondition retry may have
	// produced more than one version. Delete the bounded, explicitly owned set
	// serially; a final Inspect detects any concurrent addition.
	for _, version := range versions.Versions {
		if version < 1 {
			return ErrCloudUnavailable
		}
		err := s.artifacts.Delete(ctx, &artifact.DeleteRequest{AppName: "mini_orca_cloud", UserID: cloudUser(r), SessionID: r.ID, FileName: "copy.json", Version: version})
		if err != nil && !errors.Is(err, storage.ErrObjectNotExist) && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}
