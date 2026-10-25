package agentruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pb "cloud.google.com/go/aiplatform/apiv1beta1/aiplatformpb"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func cloudRuntimeFixture(service string) (agentstate.CloudCopy, []byte) {
	data := []byte(`{"private":"SOURCE","integer":9007199254740999}`)
	hash := sha256.Sum256(data)
	now := time.Now().UTC()
	target := config.CloudTarget{ID: "archive", Service: service, Project: "123456789012", Region: "us-central1", Resource: "archive-bucket", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "CLOUD_KEY", RetentionDays: 30}
	kind := "artifact"
	if service == "vertex_session" {
		target.Resource = "12345"
		kind = "session"
	}
	return agentstate.CloudCopy{ID: "mo-" + strings.Repeat("a", 32), Version: 1, ProjectID: "project", ProjectRevision: "revision", Target: target, Kind: kind, Hash: hex.EncodeToString(hash[:]), Size: len(data), CreatedAt: now, CleanupAt: now.Add(30 * 24 * time.Hour)}, data
}

type cloudVertexFixture struct {
	pb.UnimplementedSessionServiceServer
	mu     sync.Mutex
	target string
	record *pb.Session
	event  *pb.SessionEvent
	fail   bool
	calls  int
}

func (f *cloudVertexFixture) CreateSession(ctx context.Context, r *pb.CreateSessionRequest) (*longrunningpb.Operation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if r.Parent != f.target || f.record != nil {
		return nil, status.Error(codes.InvalidArgument, "unexpected destination")
	}
	f.record = proto.Clone(r.Session).(*pb.Session)
	f.record.Name = r.Parent + "/sessions/" + r.SessionId
	f.record.UpdateTime = timestamppb.Now()
	response, _ := anypb.New(f.record)
	return &longrunningpb.Operation{Name: f.record.Name + "/operations/create", Done: true, Result: &longrunningpb.Operation_Response{Response: response}}, nil
}
func (f *cloudVertexFixture) GetSession(ctx context.Context, r *pb.GetSessionRequest) (*pb.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.record == nil || r.Name != f.record.Name {
		return nil, status.Error(codes.NotFound, "missing")
	}
	return proto.Clone(f.record).(*pb.Session), nil
}
func (f *cloudVertexFixture) AppendEvent(ctx context.Context, r *pb.AppendEventRequest) (*pb.AppendEventResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.record == nil || r.Name != f.record.Name {
		return nil, status.Error(codes.NotFound, "missing")
	}
	f.event = proto.Clone(r.Event).(*pb.SessionEvent)
	f.event.Name = r.Name + "/events/server-assigned-id"
	if f.fail {
		return nil, status.Error(codes.PermissionDenied, "PRIVATE_DATA token=secret")
	}
	return &pb.AppendEventResponse{}, nil
}
func (f *cloudVertexFixture) ListEvents(ctx context.Context, r *pb.ListEventsRequest) (*pb.ListEventsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.record == nil || r.Parent != f.record.Name {
		return nil, status.Error(codes.NotFound, "missing")
	}
	result := &pb.ListEventsResponse{}
	if f.event != nil {
		result.SessionEvents = []*pb.SessionEvent{proto.Clone(f.event).(*pb.SessionEvent)}
	}
	return result, nil
}
func (f *cloudVertexFixture) DeleteSession(ctx context.Context, r *pb.DeleteSessionRequest) (*longrunningpb.Operation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.record == nil || r.Name != f.record.Name {
		return nil, status.Error(codes.NotFound, "missing")
	}
	if f.fail {
		return nil, status.Error(codes.PermissionDenied, "PRIVATE_DELETE")
	}
	f.record, f.event = nil, nil
	response, _ := anypb.New(&emptypb.Empty{})
	return &longrunningpb.Operation{Name: r.Name + "/operations/delete", Done: true, Result: &longrunningpb.Operation_Response{Response: response}}, nil
}

func TestCloudVertexPinnedSDKCopyInspectDelete(t *testing.T) {
	r, data := cloudRuntimeFixture("vertex_session")
	fake := &cloudVertexFixture{target: "projects/" + r.Target.Project + "/locations/" + r.Target.Region + "/reasoningEngines/" + r.Target.Resource}
	listener := bufconn.Listen(8 << 20)
	server := grpc.NewServer()
	pb.RegisterSessionServiceServer(server, fake)
	done := make(chan struct{})
	go func() { defer close(done); _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close(); <-done })
	conn, err := grpc.NewClient("passthrough:///fake-vertex", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	backend, err := newCloudServices(t.Context(), r.Target, conn.Close, option.WithGRPCConn(conn))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := backend.Copy(t.Context(), r, data); err != nil {
		t.Fatal(err)
	}
	if found, err := backend.Inspect(t.Context(), r); err != nil || !found {
		t.Fatal("native archive round trip", found, err)
	}
	foreign := r
	foreign.ProjectID = "other"
	if found, err := backend.Inspect(t.Context(), foreign); err != nil || found {
		t.Fatal("cross-project read", found, err)
	}
	if err := backend.Delete(t.Context(), foreign); err != nil {
		t.Fatal(err)
	}
	if found, err := backend.Inspect(t.Context(), r); err != nil || !found {
		t.Fatal("foreign deletion affected owner", found, err)
	}
	fake.mu.Lock()
	fake.fail = true
	fake.mu.Unlock()
	if err := backend.Delete(t.Context(), r); err != ErrCloudUnavailable || strings.Contains(err.Error(), "PRIVATE") {
		t.Fatal("deletion failure leaked or swallowed", err)
	}
	fake.mu.Lock()
	fake.fail = false
	fake.mu.Unlock()
	if err := backend.Delete(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	if found, err := backend.Inspect(t.Context(), r); err != nil || found {
		t.Fatal("deletion not observable", found, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := backend.Inspect(ctx, r); err != context.Canceled {
		t.Fatal("cancellation lost", err)
	}
}

type cloudGCSFixture struct {
	mu       sync.Mutex
	name     string
	data     []byte
	fail     bool
	calls    int
	versions []int64
}

func cloudResponse(req *http.Request, code int, body []byte, contentType string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {contentType}}, Body: io.NopCloser(bytes.NewReader(body)), Request: req, ContentLength: int64(len(body))}
}
func (f *cloudGCSFixture) RoundTrip(req *http.Request) (*http.Response, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if err := context.Cause(req.Context()); err != nil {
		return nil, err
	}
	if f.fail {
		return cloudResponse(req, 403, []byte(`{"error":{"code":403,"message":"PRIVATE_DATA"}}`), "application/json"), nil
	}
	if req.Method == http.MethodPost {
		return f.upload(req)
	}
	if req.Method == http.MethodDelete {
		_, suffix, _ := strings.Cut(req.URL.Path[strings.LastIndex(req.URL.Path, "/"):], "/")
		version, err := strconv.ParseInt(suffix, 10, 64)
		if err != nil {
			return nil, err
		}
		f.versions = slices.DeleteFunc(f.versions, func(v int64) bool { return v == version })
		if len(f.versions) == 0 {
			f.data = nil
		}
		return cloudResponse(req, 204, nil, ""), nil
	}
	if req.URL.Query().Get("prefix") != "" {
		items := []map[string]string{}
		if f.data != nil && strings.HasPrefix(f.name, req.URL.Query().Get("prefix")) {
			for _, v := range f.versions {
				items = append(items, map[string]string{"name": f.name[:strings.LastIndex(f.name, "/")+1] + strconv.FormatInt(v, 10)})
			}
		}
		data, _ := json.Marshal(map[string]any{"items": items})
		return cloudResponse(req, 200, data, "application/json"), nil
	}
	if f.data == nil {
		return cloudResponse(req, 404, []byte(`{"error":{"code":404,"message":"missing"}}`), "application/json"), nil
	}
	if req.URL.Query().Get("alt") == "media" || !strings.Contains(req.URL.Path, "/storage/v1/") {
		return cloudResponse(req, 200, f.data, "application/json"), nil
	}
	data, _ := json.Marshal(map[string]string{"name": f.name, "bucket": "archive-bucket", "contentType": "application/json", "size": fmt.Sprint(len(f.data)), "generation": "1", "metageneration": "1"})
	return cloudResponse(req, 200, data, "application/json"), nil
}
func (f *cloudGCSFixture) upload(req *http.Request) (*http.Response, error) {
	if req.URL.Query().Get("ifGenerationMatch") != "0" {
		return nil, fmt.Errorf("SDK omitted create precondition")
	}
	_, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	reader := multipart.NewReader(req.Body, params["boundary"])
	metadata, err := reader.NextPart()
	if err != nil {
		return nil, err
	}
	var attrs struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(metadata).Decode(&attrs); err != nil {
		return nil, err
	}
	payload, err := reader.NextPart()
	if err != nil {
		return nil, err
	}
	f.data, err = io.ReadAll(payload)
	if err != nil {
		return nil, err
	}
	f.name = attrs.Name
	f.versions = []int64{1}
	data, _ := json.Marshal(map[string]string{"name": f.name, "bucket": "archive-bucket", "size": fmt.Sprint(len(f.data)), "generation": "1", "metageneration": "1"})
	return cloudResponse(req, 200, data, "application/json"), nil
}

func TestCloudGCSPinnedSDKCopyInspectDelete(t *testing.T) {
	r, data := cloudRuntimeFixture("gcs_artifact")
	fake := &cloudGCSFixture{}
	backend, err := newCloudServices(t.Context(), r.Target, func() error { return nil }, option.WithHTTPClient(&http.Client{Transport: fake}), option.WithEndpoint("https://storage.googleapis.com/storage/v1/"))
	if err != nil {
		t.Fatal(err)
	}
	defer backend.Close()
	if err := backend.Copy(t.Context(), r, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fake.name, cloudUser(r)+"/"+r.ID+"/copy.json/1") {
		t.Fatal("wrong scope", fake.name)
	}
	if found, err := backend.Inspect(t.Context(), r); err != nil || !found {
		t.Fatal("native artifact round trip", found, err)
	}
	fake.versions = []int64{1, 2}
	if found, err := backend.Inspect(t.Context(), r); err == nil || found {
		t.Fatal("ambiguous SDK retry versions reported complete", found, err)
	}
	fake.fail = true
	if err := backend.Delete(t.Context(), r); err != ErrCloudUnavailable {
		t.Fatal("failed delete claimed success", err)
	}
	fake.fail = false
	if err := backend.Delete(t.Context(), r); err != nil {
		t.Fatal(err)
	}
	if found, err := backend.Inspect(t.Context(), r); err != nil || found {
		t.Fatal("remote copy retained", found, err)
	}
}

type cloudRoundTripFunc func(*http.Request) (*http.Response, error)

func (f cloudRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestCloudIdentityRegionAndTransportAdmission(t *testing.T) {
	r, _ := cloudRuntimeFixture("gcs_artifact")
	path := filepath.Join(t.TempDir(), "credentials.json")
	t.Setenv("CLOUD_KEY", path)
	for _, data := range []string{`{"type":"external_account"}`, `{"type":"service_account","client_email":"other@example.com","token_uri":"https://oauth2.googleapis.com/token"}`, `{"type":"service_account","client_email":"archive-agent@example-project.iam.gserviceaccount.com","token_uri":"https://attacker.invalid/token"}`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := OpenCloudServices(t.Context(), r.Target); err != ErrCloudUnavailable {
			t.Fatal("unbound credential admitted", err)
		}
	}
	for _, attrs := range []string{`{"location":"US-CENTRAL1","projectNumber":"123456789012"}`, `{"location":"EUROPE-WEST1","projectNumber":"123456789012"}`, `{"location":"US-CENTRAL1","projectNumber":"999999999999"}`} {
		client := &http.Client{Transport: cloudRoundTripFunc(func(req *http.Request) (*http.Response, error) {
			return cloudResponse(req, 200, []byte(attrs), "application/json"), nil
		})}
		err := checkCloudBucket(t.Context(), r.Target, client)
		if (err == nil) != (strings.Contains(attrs, "US-CENTRAL1") && strings.Contains(attrs, "123456789012")) {
			t.Fatal("location/project not enforced", err)
		}
	}
	calls := 0
	transport := &cloudTransport{base: cloudRoundTripFunc(func(req *http.Request) (*http.Response, error) { calls++; return cloudResponse(req, 200, nil, ""), nil })}
	for _, url := range []string{"http://storage.googleapis.com/", "https://attacker.invalid/"} {
		req, _ := http.NewRequest(http.MethodGet, url, nil)
		if _, err := transport.RoundTrip(req); err == nil {
			t.Fatal("untrusted endpoint admitted")
		}
	}
	if calls != 0 {
		t.Fatal("credential-bearing transport escaped")
	}
	request, _ := http.NewRequest(http.MethodGet, "https://storage.googleapis.com/", nil)
	for range 64 {
		response, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
	}
	if _, err := transport.RoundTrip(request); err == nil || calls != 64 {
		t.Fatal("HTTP request bound not enforced", calls, err)
	}
	rpc := cloudRPCLimit()
	rpcCalls := 0
	invoke := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		rpcCalls++
		return nil
	}
	for range 16 {
		if err := rpc(t.Context(), "/read", nil, nil, nil, invoke); err != nil {
			t.Fatal(err)
		}
	}
	if err := rpc(t.Context(), "/read", nil, nil, nil, invoke); err == nil || rpcCalls != 16 {
		t.Fatal("RPC request bound not enforced", rpcCalls, err)
	}
}
