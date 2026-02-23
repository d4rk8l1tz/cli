package windsurf

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseEventsAndExtraction(t *testing.T) {
	t.Parallel()

	data := []byte(`{"agent_action_name":"pre_user_prompt","trajectory_id":"t1","tool_info":{"user_prompt":"First prompt"}}` + "\n" +
		`{"agent_action_name":"post_write_code","trajectory_id":"t1","tool_info":{"file_path":"a.go"}}` + "\n" +
		`{"agent_action_name":"post_write_code","trajectory_id":"t1","tool_info":{"file_path":"b.go"}}` + "\n" +
		`{"agent_action_name":"post_write_code","trajectory_id":"t1","tool_info":{"file_path":"a.go"}}` + "\n" +
		`{"agent_action_name":"post_cascade_response","trajectory_id":"t1","tool_info":{"cascade_response":"All done"}}` + "\n")

	events, err := ParseEvents(data)
	if err != nil {
		t.Fatalf("ParseEvents() error = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("ParseEvents() len = %d, want 5", len(events))
	}

	files, err := ExtractModifiedFiles(data)
	if err != nil {
		t.Fatalf("ExtractModifiedFiles() error = %v", err)
	}
	if len(files) != 2 || !slices.Contains(files, "a.go") || !slices.Contains(files, "b.go") {
		t.Fatalf("ExtractModifiedFiles() = %v, want [a.go b.go]", files)
	}

	prompts, err := ExtractAllUserPrompts(data)
	if err != nil {
		t.Fatalf("ExtractAllUserPrompts() error = %v", err)
	}
	if len(prompts) != 1 || prompts[0] != "First prompt" {
		t.Fatalf("ExtractAllUserPrompts() = %v, want [First prompt]", prompts)
	}
}

func TestTranscriptAnalyzerMethods(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	transcriptPath := filepath.Join(dir, "windsurf.jsonl")
	content := `{"agent_action_name":"pre_user_prompt","trajectory_id":"t1","tool_info":{"user_prompt":"Prompt 1"}}
{"agent_action_name":"post_write_code","trajectory_id":"t1","tool_info":{"file_path":"app/main.go"}}
{"agent_action_name":"post_cascade_response","trajectory_id":"t1","tool_info":{"cascade_response":"Response 1"}}
{"agent_action_name":"pre_user_prompt","trajectory_id":"t1","tool_info":{"user_prompt":"Prompt 2"}}
{"agent_action_name":"post_write_code","trajectory_id":"t1","tool_info":{"file_path":"app/utils.go"}}
{"agent_action_name":"post_cascade_response","trajectory_id":"t1","tool_info":{"cascade_response":"Response 2"}}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	ag := &WindsurfAgent{}
	pos, err := ag.GetTranscriptPosition(transcriptPath)
	if err != nil {
		t.Fatalf("GetTranscriptPosition() error = %v", err)
	}
	if pos != 6 {
		t.Fatalf("GetTranscriptPosition() = %d, want 6", pos)
	}

	files, currentPos, err := ag.ExtractModifiedFilesFromOffset(transcriptPath, 3)
	if err != nil {
		t.Fatalf("ExtractModifiedFilesFromOffset() error = %v", err)
	}
	if currentPos != 6 {
		t.Fatalf("currentPos = %d, want 6", currentPos)
	}
	if len(files) != 1 || files[0] != "app/utils.go" {
		t.Fatalf("files = %v, want [app/utils.go]", files)
	}

	prompts, err := ag.ExtractPrompts(transcriptPath, 1)
	if err != nil {
		t.Fatalf("ExtractPrompts() error = %v", err)
	}
	if len(prompts) != 1 || prompts[0] != "Prompt 2" {
		t.Fatalf("prompts = %v, want [Prompt 2]", prompts)
	}

	summary, err := ag.ExtractSummary(transcriptPath)
	if err != nil {
		t.Fatalf("ExtractSummary() error = %v", err)
	}
	if summary != "Response 2" {
		t.Fatalf("summary = %q, want %q", summary, "Response 2")
	}
}

