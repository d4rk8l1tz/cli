package windsurf

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/agent"
)

// These tests use t.Chdir and cannot run in parallel.

func TestParseHookEvent_TurnStart(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	ag := &WindsurfAgent{}
	input := `{
		"agent_action_name": "pre_user_prompt",
		"trajectory_id": "trajectory-123",
		"execution_id": "exec-1",
		"tool_info": {
			"user_prompt": "Create a helper function"
		}
	}`

	event, err := ag.ParseHookEvent(HookNamePreUserPrompt, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHookEvent() error = %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != agent.TurnStart {
		t.Fatalf("event.Type = %v, want %v", event.Type, agent.TurnStart)
	}
	if event.SessionID != "trajectory-123" {
		t.Fatalf("event.SessionID = %q, want trajectory-123", event.SessionID)
	}
	if event.Prompt != "Create a helper function" {
		t.Fatalf("event.Prompt = %q, want expected prompt", event.Prompt)
	}
	if event.SessionRef == "" {
		t.Fatal("event.SessionRef is empty")
	}

	if _, err := os.Stat(event.SessionRef); err != nil {
		t.Fatalf("expected transcript file to exist at %s: %v", event.SessionRef, err)
	}
}

func TestParseHookEvent_TurnEnd(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	ag := &WindsurfAgent{}
	input := `{
		"agent_action_name": "post_cascade_response",
		"trajectory_id": "trajectory-456",
		"execution_id": "exec-2",
		"tool_info": {
			"cascade_response": "Done. I updated the file."
		}
	}`

	event, err := ag.ParseHookEvent(HookNamePostCascadeResponse, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHookEvent() error = %v", err)
	}
	if event == nil {
		t.Fatal("expected event, got nil")
	}
	if event.Type != agent.TurnEnd {
		t.Fatalf("event.Type = %v, want %v", event.Type, agent.TurnEnd)
	}
	if event.SessionID != "trajectory-456" {
		t.Fatalf("event.SessionID = %q, want trajectory-456", event.SessionID)
	}
}

func TestParseHookEvent_PostWriteCodeCaptureOnly(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	ag := &WindsurfAgent{}
	input := `{
		"agent_action_name": "post_write_code",
		"trajectory_id": "trajectory-789",
		"execution_id": "exec-3",
		"tool_info": {
			"file_path": "internal/app.go"
		}
	}`

	event, err := ag.ParseHookEvent(HookNamePostWriteCode, strings.NewReader(input))
	if err != nil {
		t.Fatalf("ParseHookEvent() error = %v", err)
	}
	if event != nil {
		t.Fatalf("expected nil event for post-write-code, got %+v", event)
	}

	transcriptPath := filepath.Join(tempDir, ".entire", "tmp", "windsurf", "trajectory-789.jsonl")
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("expected transcript file: %v", err)
	}
	if !strings.Contains(string(data), `"post_write_code"`) {
		t.Fatalf("expected transcript to contain post_write_code payload, got: %s", data)
	}
}

func TestParseHookEvent_EmptyInput(t *testing.T) {
	t.Parallel()

	ag := &WindsurfAgent{}
	_, err := ag.ParseHookEvent(HookNamePreUserPrompt, strings.NewReader(""))
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	if !strings.Contains(err.Error(), "empty hook input") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseHookEvent_MissingTrajectoryID(t *testing.T) {
	t.Parallel()

	ag := &WindsurfAgent{}
	_, err := ag.ParseHookEvent(HookNamePreUserPrompt, strings.NewReader(`{"tool_info":{"user_prompt":"x"}}`))
	if err == nil {
		t.Fatal("expected error when trajectory_id is missing")
	}
}
