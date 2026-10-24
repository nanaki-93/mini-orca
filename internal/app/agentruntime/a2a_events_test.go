package agentruntime

import (
	"testing"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

func TestRemoteAdvisoryConversionPinsIdentityAndUsage(t *testing.T) {
	acc := newRemoteAccumulator(remoteRequest().Previous)
	task := remoteTask(a2a.TaskStateWorking)
	task.Metadata = map[string]any{"adk_usage_metadata": map[string]any{"promptTokenCount": 3, "candidatesTokenCount": 4}, "adk_transfer_to_agent": "local_writer"}
	if err := acc.accept(task); err != nil {
		t.Fatal(err)
	}
	if acc.progress.Usage.Source != "remote_reported" || acc.progress.Usage.OutputTokens != 4 {
		t.Fatalf("usage=%+v", acc.progress.Usage)
	}
	changed := remoteTask(a2a.TaskStateCompleted)
	changed.ID = "another"
	if err := acc.accept(changed); err == nil || acc.progress.TaskID != "remote-task" {
		t.Fatal("identity switched")
	}
	malformed := remoteTask(a2a.TaskStateCompleted)
	malformed.Artifacts = []*a2a.Artifact{{ID: "artifact", Parts: a2a.ContentParts{{Content: a2a.Text("valid"), MediaType: "application/x-executable"}}}}
	if err := acc.accept(malformed); err == nil || acc.progress.State != "working" || len(acc.progress.Artifacts) != 0 {
		t.Fatal("malformed artifact published")
	}
	task.Metadata = map[string]any{"adk_usage_metadata": map[string]any{"promptTokenCount": nil, "candidatesTokenCount": 4}}
	if err := acc.accept(task); err == nil {
		t.Fatal("null usage became measured zero")
	}
}

func TestRemoteArtifactsRequireCompleteBoundedText(t *testing.T) {
	acc := newRemoteAccumulator(remoteRequest().Previous)
	if err := acc.accept(remoteTask(a2a.TaskStateWorking)); err != nil {
		t.Fatal(err)
	}
	artifact := &a2a.Artifact{ID: "report", Name: "Review", Parts: a2a.ContentParts{a2a.NewTextPart("first")}}
	if err := acc.artifact(artifact, false, false); err != nil {
		t.Fatal(err)
	}
	acc.progress.State = "completed"
	if err := acc.progress.Validate(); err == nil {
		t.Fatal("unfinished artifact completed")
	}
	acc.progress.State = "working"
	artifact.Parts = a2a.ContentParts{a2a.NewTextPart(" second")}
	if err := acc.artifact(artifact, true, false); err != nil {
		t.Fatal(err)
	}
	if acc.progress.Artifacts[0].Complete {
		t.Fatal("partial artifact sealed before task completion")
	}
	if err := acc.accept(&a2a.TaskStatusUpdateEvent{TaskID: "remote-task", ContextID: "remote-context", Status: a2a.TaskStatus{State: a2a.TaskStateCompleted}}); err != nil {
		t.Fatal(err)
	}
	if !acc.progress.Artifacts[0].Complete || acc.progress.Artifacts[0].Text != "first second" {
		t.Fatal("terminal task did not seal the artifact")
	}
	var absent *a2a.Task
	if err := acc.accept(absent); err == nil {
		t.Fatal("nil task accepted")
	}
}
