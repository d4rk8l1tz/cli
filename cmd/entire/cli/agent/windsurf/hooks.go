package windsurf

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	"github.com/entireio/cli/cmd/entire/cli/paths"
)

// Ensure WindsurfAgent implements HookSupport.
var _ agent.HookSupport = (*WindsurfAgent)(nil)

var entireHookPrefixes = []string{
	"entire ",
	"go run ${WINDSURF_PROJECT_DIR}/cmd/entire/main.go ",
}

func windsurfHooksPath() (string, error) {
	repoRoot, err := paths.RepoRoot()
	if err != nil {
		//nolint:forbidigo // Fallback for tests outside git repositories.
		repoRoot, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
	}
	return filepath.Join(repoRoot, ".windsurf", WindsurfHooksFileName), nil
}

// InstallHooks installs Windsurf hook commands into .windsurf/hooks.json.
func (a *WindsurfAgent) InstallHooks(localDev bool, force bool) (int, error) {
	hooksPath, err := windsurfHooksPath()
	if err != nil {
		return 0, err
	}

	var rawHooks map[string]json.RawMessage
	if data, readErr := os.ReadFile(hooksPath); readErr == nil { //nolint:gosec // Path is repo-local.
		if err := json.Unmarshal(data, &rawHooks); err != nil {
			return 0, fmt.Errorf("failed to parse hooks.json: %w", err)
		}
	} else {
		rawHooks = make(map[string]json.RawMessage)
	}
	if rawHooks == nil {
		rawHooks = make(map[string]json.RawMessage)
	}

	prePromptHooks, err := parseHookList(rawHooks[actionPreUserPrompt])
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s hooks: %w", actionPreUserPrompt, err)
	}
	postWriteHooks, err := parseHookList(rawHooks[actionPostWriteCode])
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s hooks: %w", actionPostWriteCode, err)
	}
	postResponseHooks, err := parseHookList(rawHooks[actionPostCascadeResponse])
	if err != nil {
		return 0, fmt.Errorf("failed to parse %s hooks: %w", actionPostCascadeResponse, err)
	}

	cmdPrefix := "entire hooks windsurf "
	if localDev {
		cmdPrefix = "go run ${WINDSURF_PROJECT_DIR}/cmd/entire/main.go hooks windsurf "
	}

	prePromptCmd := cmdPrefix + HookNamePreUserPrompt
	postWriteCmd := cmdPrefix + HookNamePostWriteCode
	postResponseCmd := cmdPrefix + HookNamePostCascadeResponse

	// Idempotent fast-path for same mode.
	if !force &&
		hookCommandExists(prePromptHooks, prePromptCmd) &&
		hookCommandExists(postWriteHooks, postWriteCmd) &&
		hookCommandExists(postResponseHooks, postResponseCmd) {
		return 0, nil
	}

	prePromptHooks = removeEntireHooks(prePromptHooks)
	postWriteHooks = removeEntireHooks(postWriteHooks)
	postResponseHooks = removeEntireHooks(postResponseHooks)

	prePromptHooks = append(prePromptHooks, WindsurfHookConfig{Command: prePromptCmd})
	postWriteHooks = append(postWriteHooks, WindsurfHookConfig{Command: postWriteCmd})
	postResponseHooks = append(postResponseHooks, WindsurfHookConfig{Command: postResponseCmd})

	if err := marshalHookList(rawHooks, actionPreUserPrompt, prePromptHooks); err != nil {
		return 0, fmt.Errorf("failed to encode %s hooks: %w", actionPreUserPrompt, err)
	}
	if err := marshalHookList(rawHooks, actionPostWriteCode, postWriteHooks); err != nil {
		return 0, fmt.Errorf("failed to encode %s hooks: %w", actionPostWriteCode, err)
	}
	if err := marshalHookList(rawHooks, actionPostCascadeResponse, postResponseHooks); err != nil {
		return 0, fmt.Errorf("failed to encode %s hooks: %w", actionPostCascadeResponse, err)
	}

	//nolint:gosec // Repo-local config directory.
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0o755); err != nil {
		return 0, fmt.Errorf("failed to create .windsurf directory: %w", err)
	}

	data, err := json.MarshalIndent(rawHooks, "", "  ")
	if err != nil {
		return 0, fmt.Errorf("failed to marshal hooks config: %w", err)
	}

	if err := os.WriteFile(hooksPath, data, 0o600); err != nil {
		return 0, fmt.Errorf("failed to write hooks.json: %w", err)
	}

	return 3, nil
}

// UninstallHooks removes Entire hooks from .windsurf/hooks.json.
func (a *WindsurfAgent) UninstallHooks() error {
	hooksPath, err := windsurfHooksPath()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(hooksPath) //nolint:gosec // Path is repo-local.
	if err != nil {
		return nil //nolint:nilerr // Nothing to uninstall.
	}

	var rawHooks map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawHooks); err != nil {
		return fmt.Errorf("failed to parse hooks.json: %w", err)
	}

	for _, key := range []string{actionPreUserPrompt, actionPostWriteCode, actionPostCascadeResponse} {
		hooks, err := parseHookList(rawHooks[key])
		if err != nil {
			return fmt.Errorf("failed to parse %s hooks: %w", key, err)
		}
		hooks = removeEntireHooks(hooks)
		if err := marshalHookList(rawHooks, key, hooks); err != nil {
			return fmt.Errorf("failed to encode %s hooks: %w", key, err)
		}
	}

	output, err := json.MarshalIndent(rawHooks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks config: %w", err)
	}
	if err := os.WriteFile(hooksPath, output, 0o600); err != nil {
		return fmt.Errorf("failed to write hooks.json: %w", err)
	}
	return nil
}

// AreHooksInstalled returns true when any Entire Windsurf hook is present.
func (a *WindsurfAgent) AreHooksInstalled() bool {
	hooksPath, err := windsurfHooksPath()
	if err != nil {
		return false
	}

	data, err := os.ReadFile(hooksPath) //nolint:gosec // Path is repo-local.
	if err != nil {
		return false
	}

	var rawHooks map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawHooks); err != nil {
		return false
	}

	for _, key := range []string{actionPreUserPrompt, actionPostWriteCode, actionPostCascadeResponse} {
		hooks, err := parseHookList(rawHooks[key])
		if err != nil {
			continue
		}
		if hasEntireHook(hooks) {
			return true
		}
	}

	return false
}

func parseHookList(raw json.RawMessage) ([]WindsurfHookConfig, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var hooks []WindsurfHookConfig
	if err := json.Unmarshal(raw, &hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}

func marshalHookList(rawHooks map[string]json.RawMessage, key string, hooks []WindsurfHookConfig) error {
	if len(hooks) == 0 {
		delete(rawHooks, key)
		return nil
	}

	data, err := json.Marshal(hooks)
	if err != nil {
		return err
	}
	rawHooks[key] = data
	return nil
}

func hasEntireHook(hooks []WindsurfHookConfig) bool {
	for _, hook := range hooks {
		if isEntireHook(hook.Command) {
			return true
		}
	}
	return false
}

func hookCommandExists(hooks []WindsurfHookConfig, command string) bool {
	for _, hook := range hooks {
		if strings.TrimSpace(hook.Command) == command {
			return true
		}
	}
	return false
}

func removeEntireHooks(hooks []WindsurfHookConfig) []WindsurfHookConfig {
	filtered := make([]WindsurfHookConfig, 0, len(hooks))
	for _, hook := range hooks {
		if !isEntireHook(hook.Command) {
			filtered = append(filtered, hook)
		}
	}
	return filtered
}

func isEntireHook(command string) bool {
	for _, prefix := range entireHookPrefixes {
		if strings.HasPrefix(command, prefix) {
			return true
		}
	}
	return false
}

