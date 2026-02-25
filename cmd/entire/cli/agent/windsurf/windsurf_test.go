package windsurf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/agent"
)

func TestAgentIdentity(t *testing.T) {
	t.Parallel()

	ag := &WindsurfAgent{}
	if ag.Name() != agent.AgentNameWindsurf {
		t.Fatalf("Name() = %q, want %q", ag.Name(), agent.AgentNameWindsurf)
	}
	if ag.Type() != agent.AgentTypeWindsurf {
		t.Fatalf("Type() = %q, want %q", ag.Type(), agent.AgentTypeWindsurf)
	}
	if !ag.IsPreview() {
		t.Fatal("IsPreview() = false, want true")
	}
	if len(ag.ProtectedDirs()) != 1 || ag.ProtectedDirs()[0] != ".windsurf" {
		t.Fatalf("ProtectedDirs() = %v, want [.windsurf]", ag.ProtectedDirs())
	}
	if ag.FormatResumeCommand("abc") != "windsurf" {
		t.Fatalf("FormatResumeCommand() = %q, want %q", ag.FormatResumeCommand("abc"), "windsurf")
	}
}

func TestDetectPresence(t *testing.T) {
	// Uses t.Chdir.
	dir := t.TempDir()
	t.Chdir(dir)

	ag := &WindsurfAgent{}
	present, err := ag.DetectPresence()
	if err != nil {
		t.Fatalf("DetectPresence() error = %v", err)
	}
	if present {
		t.Fatal("DetectPresence() = true, want false with no config")
	}

	if err := os.MkdirAll(filepath.Join(dir, ".windsurf"), 0o755); err != nil {
		t.Fatalf("failed to create .windsurf: %v", err)
	}
	present, err = ag.DetectPresence()
	if err != nil {
		t.Fatalf("DetectPresence() error = %v", err)
	}
	if !present {
		t.Fatal("DetectPresence() = false, want true")
	}
}

func TestReadWriteSession(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ag := &WindsurfAgent{}
	sessionPath := filepath.Join(dir, "session.jsonl")
	content := []byte(`{"agent_action_name":"pre_user_prompt","trajectory_id":"t1","tool_info":{"user_prompt":"hello"}}` + "\n")

	session := &agent.AgentSession{
		SessionID:  "t1",
		AgentName:  ag.Name(),
		SessionRef: sessionPath,
		NativeData: content,
	}
	if err := ag.WriteSession(session); err != nil {
		t.Fatalf("WriteSession() error = %v", err)
	}

	readSession, err := ag.ReadSession(&agent.HookInput{
		SessionID:  "t1",
		SessionRef: sessionPath,
	})
	if err != nil {
		t.Fatalf("ReadSession() error = %v", err)
	}
	if readSession.AgentName != ag.Name() {
		t.Fatalf("ReadSession().AgentName = %q, want %q", readSession.AgentName, ag.Name())
	}
	if string(readSession.NativeData) != string(content) {
		t.Fatalf("ReadSession().NativeData mismatch")
	}
}

func TestGetSessionDirAndResolveSessionFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ag := &WindsurfAgent{}
	sessionDir, err := ag.GetSessionDir(dir)
	if err != nil {
		t.Fatalf("GetSessionDir() error = %v", err)
	}
	expectedDir := filepath.Join(dir, ".entire", "tmp", "windsurf")
	if sessionDir != expectedDir {
		t.Fatalf("GetSessionDir() = %q, want %q", sessionDir, expectedDir)
	}

	file := ag.ResolveSessionFile(sessionDir, "2026-02-23:trajectory/1")
	wantSuffix := filepath.Join(expectedDir, "2026-02-23-trajectory-1.jsonl")
	if file != wantSuffix {
		t.Fatalf("ResolveSessionFile() = %q, want %q", file, wantSuffix)
	}
}
