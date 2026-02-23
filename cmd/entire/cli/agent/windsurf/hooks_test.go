package windsurf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallHooks_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ag := &WindsurfAgent{}
	count, err := ag.InstallHooks(false, false)
	if err != nil {
		t.Fatalf("InstallHooks() error = %v", err)
	}
	if count != 3 {
		t.Fatalf("InstallHooks() count = %d, want 3", count)
	}

	hooks := readHooksFile(t, dir)
	verifyHookCommand(t, hooks[actionPreUserPrompt], "entire hooks windsurf "+HookNamePreUserPrompt)
	verifyHookCommand(t, hooks[actionPostWriteCode], "entire hooks windsurf "+HookNamePostWriteCode)
	verifyHookCommand(t, hooks[actionPostCascadeResponse], "entire hooks windsurf "+HookNamePostCascadeResponse)
}

func TestInstallHooks_Idempotent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ag := &WindsurfAgent{}
	if _, err := ag.InstallHooks(false, false); err != nil {
		t.Fatalf("first InstallHooks() error = %v", err)
	}

	count, err := ag.InstallHooks(false, false)
	if err != nil {
		t.Fatalf("second InstallHooks() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("second InstallHooks() count = %d, want 0", count)
	}

	hooks := readHooksFile(t, dir)
	if len(hooks[actionPreUserPrompt]) != 1 {
		t.Fatalf("pre_user_prompt hooks = %d, want 1", len(hooks[actionPreUserPrompt]))
	}
	if len(hooks[actionPostWriteCode]) != 1 {
		t.Fatalf("post_write_code hooks = %d, want 1", len(hooks[actionPostWriteCode]))
	}
	if len(hooks[actionPostCascadeResponse]) != 1 {
		t.Fatalf("post_cascade_response hooks = %d, want 1", len(hooks[actionPostCascadeResponse]))
	}
}

func TestInstallHooks_LocalDev(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ag := &WindsurfAgent{}
	if _, err := ag.InstallHooks(true, false); err != nil {
		t.Fatalf("InstallHooks(localDev=true) error = %v", err)
	}

	hooks := readHooksFile(t, dir)
	verifyHookCommand(t, hooks[actionPreUserPrompt], "go run ${WINDSURF_PROJECT_DIR}/cmd/entire/main.go hooks windsurf "+HookNamePreUserPrompt)
	verifyHookCommand(t, hooks[actionPostWriteCode], "go run ${WINDSURF_PROJECT_DIR}/cmd/entire/main.go hooks windsurf "+HookNamePostWriteCode)
	verifyHookCommand(t, hooks[actionPostCascadeResponse], "go run ${WINDSURF_PROJECT_DIR}/cmd/entire/main.go hooks windsurf "+HookNamePostCascadeResponse)
}

func TestInstallHooks_PreservesUnknownFields(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.MkdirAll(filepath.Join(dir, ".windsurf"), 0o755); err != nil {
		t.Fatalf("failed to create .windsurf: %v", err)
	}

	existing := `{
  "custom_setting": "keep-me",
  "pre_user_prompt": [
    {"command": "echo custom-user-hook"}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, ".windsurf", WindsurfHooksFileName), []byte(existing), 0o644); err != nil {
		t.Fatalf("failed to write hooks.json: %v", err)
	}

	ag := &WindsurfAgent{}
	if _, err := ag.InstallHooks(false, false); err != nil {
		t.Fatalf("InstallHooks() error = %v", err)
	}

	raw := readRawHooks(t, dir)
	if _, ok := raw["custom_setting"]; !ok {
		t.Fatalf("custom_setting field should be preserved")
	}

	hooks := readHooksFile(t, dir)
	verifyHookCommand(t, hooks[actionPreUserPrompt], "echo custom-user-hook")
	verifyHookCommand(t, hooks[actionPreUserPrompt], "entire hooks windsurf "+HookNamePreUserPrompt)
}

func TestUninstallHooks_RemovesEntireHooksOnly(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	if err := os.MkdirAll(filepath.Join(dir, ".windsurf"), 0o755); err != nil {
		t.Fatalf("failed to create .windsurf: %v", err)
	}
	content := `{
  "pre_user_prompt": [
    {"command": "echo user-hook"},
    {"command": "entire hooks windsurf pre-user-prompt"}
  ],
  "post_write_code": [
    {"command": "entire hooks windsurf post-write-code"}
  ],
  "post_cascade_response": [
    {"command": "entire hooks windsurf post-cascade-response"}
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, ".windsurf", WindsurfHooksFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write hooks.json: %v", err)
	}

	ag := &WindsurfAgent{}
	if err := ag.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks() error = %v", err)
	}

	hooks := readHooksFile(t, dir)
	verifyHookCommand(t, hooks[actionPreUserPrompt], "echo user-hook")
	if hasEntireHook(hooks[actionPreUserPrompt]) {
		t.Fatal("expected no Entire hooks in pre_user_prompt")
	}
	if len(hooks[actionPostWriteCode]) != 0 {
		t.Fatalf("post_write_code hooks = %d, want 0", len(hooks[actionPostWriteCode]))
	}
	if len(hooks[actionPostCascadeResponse]) != 0 {
		t.Fatalf("post_cascade_response hooks = %d, want 0", len(hooks[actionPostCascadeResponse]))
	}
}

func TestAreHooksInstalled(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	ag := &WindsurfAgent{}
	if ag.AreHooksInstalled() {
		t.Fatal("AreHooksInstalled() = true, want false before install")
	}

	if _, err := ag.InstallHooks(false, false); err != nil {
		t.Fatalf("InstallHooks() error = %v", err)
	}

	if !ag.AreHooksInstalled() {
		t.Fatal("AreHooksInstalled() = false, want true after install")
	}

	if err := ag.UninstallHooks(); err != nil {
		t.Fatalf("UninstallHooks() error = %v", err)
	}
	if ag.AreHooksInstalled() {
		t.Fatal("AreHooksInstalled() = true, want false after uninstall")
	}
}

func readHooksFile(t *testing.T, tempDir string) map[string][]WindsurfHookConfig {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(tempDir, ".windsurf", WindsurfHooksFileName))
	if err != nil {
		t.Fatalf("failed to read hooks file: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse hooks.json: %v", err)
	}

	out := make(map[string][]WindsurfHookConfig)
	for _, key := range []string{actionPreUserPrompt, actionPostWriteCode, actionPostCascadeResponse} {
		hooks, err := parseHookList(raw[key])
		if err != nil {
			t.Fatalf("failed to parse hook list %s: %v", key, err)
		}
		out[key] = hooks
	}
	return out
}

func readRawHooks(t *testing.T, tempDir string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(tempDir, ".windsurf", WindsurfHooksFileName))
	if err != nil {
		t.Fatalf("failed to read hooks file: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse hooks.json: %v", err)
	}
	return raw
}

func verifyHookCommand(t *testing.T, hooks []WindsurfHookConfig, command string) {
	t.Helper()
	for _, hook := range hooks {
		if hook.Command == command {
			return
		}
	}
	t.Fatalf("expected hook command %q not found in %#v", command, hooks)
}

