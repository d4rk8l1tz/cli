//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/agent/windsurf"
)

func TestSetupWindsurfHooks_AddsAllRequiredHooks(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	env.InitRepo()
	env.InitEntire("manual-commit")

	env.WriteFile("README.md", "# Test")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")

	output, err := env.RunCLIWithError("enable", "--agent", "windsurf")
	if err != nil {
		t.Fatalf("enable windsurf command failed: %v\nOutput: %s", err, output)
	}

	hooks := readWindsurfHooks(t, env)
	if len(hooks[actionPreUserPrompt]) == 0 {
		t.Fatal("pre_user_prompt hook should exist")
	}
	if len(hooks[actionPostWriteCode]) == 0 {
		t.Fatal("post_write_code hook should exist")
	}
	if len(hooks[actionPostCascadeResponse]) == 0 {
		t.Fatal("post_cascade_response hook should exist")
	}

	assertHookCommand(t, hooks[actionPreUserPrompt], "entire hooks windsurf pre-user-prompt")
	assertHookCommand(t, hooks[actionPostWriteCode], "entire hooks windsurf post-write-code")
	assertHookCommand(t, hooks[actionPostCascadeResponse], "entire hooks windsurf post-cascade-response")
}

func TestSetupWindsurfHooks_PreservesExistingSettings(t *testing.T) {
	t.Parallel()

	env := NewTestEnv(t)
	env.InitRepo()
	env.InitEntire("manual-commit")

	env.WriteFile("README.md", "# Test")
	env.GitAdd("README.md")
	env.GitCommit("Initial commit")

	windsurfDir := filepath.Join(env.RepoDir, ".windsurf")
	if err := os.MkdirAll(windsurfDir, 0o755); err != nil {
		t.Fatalf("failed to create .windsurf dir: %v", err)
	}

	existing := `{
  "custom_setting": "should-be-preserved",
  "pre_user_prompt": [
    {"command": "echo existing-hook"}
  ]
}`
	hooksPath := filepath.Join(windsurfDir, windsurf.WindsurfHooksFileName)
	if err := os.WriteFile(hooksPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("failed to write hooks.json: %v", err)
	}

	output, err := env.RunCLIWithError("enable", "--agent", "windsurf")
	if err != nil {
		t.Fatalf("enable windsurf failed: %v\nOutput: %s", err, output)
	}

	// Verify unknown field was preserved.
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse hooks.json: %v", err)
	}
	if raw["custom_setting"] != "should-be-preserved" {
		t.Fatalf("custom_setting = %v, want should-be-preserved", raw["custom_setting"])
	}

	hooks := readWindsurfHooks(t, env)
	assertHookCommand(t, hooks[actionPreUserPrompt], "echo existing-hook")
	assertHookCommand(t, hooks[actionPreUserPrompt], "entire hooks windsurf pre-user-prompt")
}

const (
	actionPreUserPrompt       = "pre_user_prompt"
	actionPostWriteCode       = "post_write_code"
	actionPostCascadeResponse = "post_cascade_response"
)

func readWindsurfHooks(t *testing.T, env *TestEnv) map[string][]windsurf.WindsurfHookConfig {
	t.Helper()

	hooksPath := filepath.Join(env.RepoDir, ".windsurf", windsurf.WindsurfHooksFileName)
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("failed to read hooks.json: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse hooks.json: %v", err)
	}

	result := make(map[string][]windsurf.WindsurfHookConfig)
	for _, key := range []string{actionPreUserPrompt, actionPostWriteCode, actionPostCascadeResponse} {
		var hooks []windsurf.WindsurfHookConfig
		if section, ok := raw[key]; ok {
			if err := json.Unmarshal(section, &hooks); err != nil {
				t.Fatalf("failed to parse %s hooks: %v", key, err)
			}
		}
		result[key] = hooks
	}
	return result
}

func assertHookCommand(t *testing.T, hooks []windsurf.WindsurfHookConfig, expected string) {
	t.Helper()
	for _, hook := range hooks {
		if hook.Command == expected {
			return
		}
	}
	t.Fatalf("expected hook command %q not found in %#v", expected, hooks)
}

