package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func cloudClientFixture() (AgentSettingsScope, CloudPreview, CloudCopy) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}}
	target := CloudTarget{ID: "archive", Service: "gcs_artifact", Project: "123456789012", Region: "europe-west10", Resource: "archive-bucket", Principal: "archive-agent@example-project.iam.gserviceaccount.com", CredentialFileEnv: "CLOUD_KEY", RetentionDays: 30}
	data, _ := json.Marshal(target)
	hash := sha256.Sum256(data)
	p := CloudPreview{CloudSelection: CloudSelection{ProjectIdentity: scope.ProjectIdentity, TargetID: target.ID, Kind: "artifact", SourceID: strings.Repeat("a", 64)}, ID: "mo-" + strings.Repeat("b", 32), Target: target, Configuration: hex.EncodeToString(hash[:]), SourceRevision: "original-source", SourceVersion: 3, Hash: strings.Repeat("c", 64), Size: 123}
	now := time.Now().UTC()
	c := CloudCopy{ProjectIdentity: scope.ProjectIdentity, ID: p.ID, Version: 2, Target: target, Configuration: p.Configuration, Kind: p.Kind, SourceID: p.SourceID, SourceVersion: p.SourceVersion, SourceRevision: p.SourceRevision, Hash: p.Hash, Size: p.Size, State: "completed", CreatedAt: now, UpdatedAt: now, CleanupAt: now.Add(30 * 24 * time.Hour)}
	return scope, p, c
}

func cloudTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

func TestCloudClientExactCopyAndOriginalRevisionRecovery(t *testing.T) {
	scope, p, initial := cloudClientFixture()
	var calls atomic.Int32
	var current atomic.Pointer[CloudCopy]
	current.Store(&initial)
	cl := cloudTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Has("worktree_path") || r.URL.Query().Get("project_id") != scope.ProjectID {
			t.Error("lost main checkout binding")
		}
		switch r.URL.Path {
		case "/api/projects/current/cloud-copies":
			identity := ProjectIdentity{ProjectID: r.URL.Query().Get("project_id"), ProjectRevision: r.URL.Query().Get("project_revision")}
			_ = json.NewEncoder(w).Encode(CloudCatalog{ProjectIdentity: identity, Destinations: []CloudDestination{{Target: p.Target, Configuration: p.Configuration, State: "configured"}}, Copies: []CloudCopy{*current.Load()}})
		case "/api/projects/current/cloud-copies/preview":
			var selection CloudSelection
			if json.NewDecoder(r.Body).Decode(&selection) != nil || selection != p.CloudSelection {
				t.Error("preview changed selection")
			}
			_ = json.NewEncoder(w).Encode(p)
		case "/api/projects/current/cloud-copies/control":
			var request CloudControl
			if json.NewDecoder(r.Body).Decode(&request) != nil || !request.Confirm {
				t.Error("missing explicit confirmation")
			}
			result := *current.Load()
			if request.Action == "copy" {
				if !request.ConfirmSourceSharing || request.Preview == nil || *request.Preview != p {
					t.Error("missing exact source-sharing consent")
				}
			} else {
				result.Version = request.Version + 2
			}
			if request.Action == "delete" {
				result.State = "deleted"
			}
			result.UpdatedAt = time.Now().UTC()
			current.Store(&result)
			_ = json.NewEncoder(w).Encode(result)
		default:
			t.Error("unexpected route", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	if _, err := cl.ReadCloudCopies(t.Context(), "read", scope); err != nil {
		t.Fatal(err)
	}
	preview, err := cl.PreviewCloudCopy(t.Context(), "preview", scope, p.CloudSelection)
	if err != nil {
		t.Fatal(err)
	}
	request := CloudControl{ProjectIdentity: scope.ProjectIdentity, Action: "copy", ID: p.ID, Preview: preview, Confirm: true, ConfirmSourceSharing: true}
	copy, err := cl.ControlCloudCopy(t.Context(), "copy", scope, request, nil)
	if err != nil || copy.State != "completed" {
		t.Fatal(copy, err)
	}
	scope.ProjectRevision = "new-revision"
	catalog, err := cl.ReadCloudCopies(t.Context(), "reindexed", scope)
	if err != nil || catalog.Copies[0].ProjectRevision != p.ProjectRevision {
		t.Fatal("original revision lost", catalog, err)
	}
	for _, action := range []string{"check", "delete"} {
		request = CloudControl{ProjectIdentity: scope.ProjectIdentity, Action: action, ID: copy.ID, Version: copy.Version, Confirm: true}
		copy, err = cl.ControlCloudCopy(t.Context(), action, scope, request, copy)
		if err != nil {
			t.Fatal(action, err)
		}
	}
	if copy.Version != 6 || copy.State != "deleted" || copy.SourceRevision != "original-source" {
		t.Fatal(copy)
	}
	before := calls.Load()
	request.Confirm = false
	if _, err = cl.ControlCloudCopy(t.Context(), "unconfirmed", scope, request, copy); err == nil {
		t.Fatal("unconfirmed control accepted")
	}
	scope.WorktreePath = "/foreign"
	if _, err = cl.ReadCloudCopies(t.Context(), "foreign", scope); err == nil || calls.Load() != before {
		t.Fatal("foreign/unconfirmed control reached transport")
	}
	for _, route := range [][2]string{{"POST", "cloud-copies"}, {"GET", "cloud-copies/control"}, {"GET", "cloud-copies/preview"}, {"DELETE", "cloud-copies"}} {
		if allowedRequest(route[0], "/api/projects/current/"+route[1], "{}") {
			t.Fatal("unexpected route admitted", route)
		}
	}
}

func TestCloudClientRejectsCorruptOrForeignCatalogs(t *testing.T) {
	scope, p, c := cloudClientFixture()
	for name, change := range map[string]func(*CloudCatalog){
		"foreign checkout":      func(v *CloudCatalog) { v.ProjectRevision = "foreign" },
		"foreign receipt":       func(v *CloudCatalog) { v.Copies[0].ProjectID = "foreign" },
		"wrong target hash":     func(v *CloudCatalog) { v.Destinations[0].Target.Resource = "different-bucket" },
		"invented availability": func(v *CloudCatalog) { v.Destinations[0].State = "connected" },
		"duplicate receipt":     func(v *CloudCatalog) { v.Copies = append(v.Copies, v.Copies[0]) },
		"duplicate target":      func(v *CloudCatalog) { v.Destinations = append(v.Destinations, v.Destinations[0]) },
		"false deletion":        func(v *CloudCatalog) { v.Copies[0].State = "success" },
		"missing lease":         func(v *CloudCatalog) { v.Copies[0].State = "copying" },
		"bad cleanup":           func(v *CloudCatalog) { v.Copies[0].CleanupAt = v.Copies[0].CreatedAt },
		"unsafe source":         func(v *CloudCatalog) { v.Copies[0].SourceID = "escape\x1b[31m" },
	} {
		t.Run(name, func(t *testing.T) {
			v := CloudCatalog{ProjectIdentity: scope.ProjectIdentity, Destinations: []CloudDestination{{Target: p.Target, Configuration: p.Configuration, State: "configured"}}, Copies: []CloudCopy{c}}
			change(&v)
			cl := cloudTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(v) })
			if _, err := cl.ReadCloudCopies(t.Context(), "read", scope); err == nil {
				t.Fatal("corrupt catalog accepted")
			}
		})
	}
	for _, body := range []string{`null`, `{}`, `{"project_id":"project","project_revision":"revision","automatic_upload":true}`, `{} {}`} {
		t.Run(body, func(t *testing.T) {
			cl := cloudTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
			if _, err := cl.ReadCloudCopies(t.Context(), "bad", scope); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func TestCloudClientBindsPreviewAndReceiptToConfirmedContent(t *testing.T) {
	scope, p, c := cloudClientFixture()
	for name, change := range map[string]func(*CloudCopy){
		"hash":              func(v *CloudCopy) { v.Hash = strings.Repeat("d", 64) },
		"revision":          func(v *CloudCopy) { v.SourceRevision = "replacement" },
		"source":            func(v *CloudCopy) { v.SourceID = "replacement" },
		"version":           func(v *CloudCopy) { v.Version++ },
		"unexpected delete": func(v *CloudCopy) { v.State = "deleted" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := c
			change(&bad)
			cl := cloudTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(bad) })
			r := CloudControl{ProjectIdentity: scope.ProjectIdentity, Action: "copy", ID: p.ID, Preview: &p, Confirm: true, ConfirmSourceSharing: true}
			if _, err := cl.ControlCloudCopy(t.Context(), "bad", scope, r, nil); err == nil {
				t.Fatal("substituted receipt accepted")
			}
		})
	}
	bad := p
	bad.SourceID = "different"
	cl := cloudTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(bad) })
	if _, err := cl.PreviewCloudCopy(t.Context(), "preview", scope, p.CloudSelection); err == nil {
		t.Fatal("different source preview accepted")
	}
	unknown := c
	unknown.State = "unknown"
	cl = cloudTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(unknown) })
	r := CloudControl{ProjectIdentity: scope.ProjectIdentity, Action: "copy", ID: p.ID, Preview: &p, Confirm: true, ConfirmSourceSharing: true}
	receipt, err := cl.ControlCloudCopy(t.Context(), "unknown", scope, r, nil)
	if err != nil || receipt.State != "unknown" {
		t.Fatal("uncertain outcome became success", receipt, err)
	}
}

func TestCloudClientCancellationAndRefreshIdentity(t *testing.T) {
	scope, _, _ := cloudClientFixture()
	started := make(chan struct{})
	cl := cloudTestClient(t, func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := cl.ReadCloudCopies(ctx, "pending", scope); done <- err }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cl.Cancel("pending")
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("canceled transport succeeded")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cl = cloudTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: ProjectIdentity{ProjectID: "foreign", ProjectRevision: "rev"}})
	})
	if _, _, err := cl.RefreshCloudCopies(t.Context(), "refresh", scope); err == nil {
		t.Fatal("refresh adopted a different project")
	}
}
