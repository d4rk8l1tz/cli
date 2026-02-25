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

const windsurfHooksRootKey = "hooks"

var windsurfActionKeys = []string{
	actionPreUserPrompt,
	actionPostWriteCode,
	actionPostCascadeResponse,
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

	rawSettings, rawHooks, hasNestedHooks, err := loadWindsurfHookConfig(hooksPath)
	if err != nil {
		return 0, err
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
	// Keep migrating legacy top-level formats to nested {"hooks":{...}}.
	if !force &&
		hasNestedHooks &&
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

	if err := writeWindsurfHookConfig(hooksPath, rawSettings, rawHooks); err != nil {
		return 0, err
	}

	return 3, nil
}

// UninstallHooks removes Entire hooks from .windsurf/hooks.json.
func (a *WindsurfAgent) UninstallHooks() error {
	hooksPath, err := windsurfHooksPath()
	if err != nil {
		return err
	}

	rawSettings, rawHooks, _, err := loadWindsurfHookConfig(hooksPath)
	if err != nil {
		return err
	}

	if len(rawSettings) == 0 && len(rawHooks) == 0 {
		return nil //nolint:nilerr // Nothing to uninstall.
	}

	for _, key := range windsurfActionKeys {
		hooks, err := parseHookList(rawHooks[key])
		if err != nil {
			return fmt.Errorf("failed to parse %s hooks: %w", key, err)
		}
		hooks = removeEntireHooks(hooks)
		if err := marshalHookList(rawHooks, key, hooks); err != nil {
			return fmt.Errorf("failed to encode %s hooks: %w", key, err)
		}
	}

	if err := writeWindsurfHookConfig(hooksPath, rawSettings, rawHooks); err != nil {
		return err
	}
	return nil
}

// AreHooksInstalled returns true when any Entire Windsurf hook is present.
func (a *WindsurfAgent) AreHooksInstalled() bool {
	hooksPath, err := windsurfHooksPath()
	if err != nil {
		return false
	}

	_, rawHooks, _, err := loadWindsurfHookConfig(hooksPath)
	if err != nil {
		return false
	}

	for _, key := range windsurfActionKeys {
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

func loadWindsurfHookConfig(path string) (map[string]json.RawMessage, map[string]json.RawMessage, bool, error) {
	rawSettings := make(map[string]json.RawMessage)
	rawHooks := make(map[string]json.RawMessage)

	data, err := os.ReadFile(path) //nolint:gosec // Path is repo-local.
	if err != nil {
		if os.IsNotExist(err) {
			return rawSettings, rawHooks, false, nil
		}
		return nil, nil, false, fmt.Errorf("failed to read hooks.json: %w", err)
	}

	if err := json.Unmarshal(data, &rawSettings); err != nil {
		return nil, nil, false, fmt.Errorf("failed to parse hooks.json: %w", err)
	}
	if rawSettings == nil {
		rawSettings = make(map[string]json.RawMessage)
	}

	hasNestedHooks := false
	if hooksSectionRaw, ok := rawSettings[windsurfHooksRootKey]; ok && len(hooksSectionRaw) > 0 {
		if err := json.Unmarshal(hooksSectionRaw, &rawHooks); err != nil {
			return nil, nil, false, fmt.Errorf("failed to parse hooks section: %w", err)
		}
		if rawHooks == nil {
			rawHooks = make(map[string]json.RawMessage)
		}
		hasNestedHooks = true
	}

	// Backward compatibility: support legacy top-level action keys.
	for _, key := range windsurfActionKeys {
		if _, exists := rawHooks[key]; exists {
			continue
		}
		if raw, ok := rawSettings[key]; ok {
			rawHooks[key] = raw
		}
	}

	return rawSettings, rawHooks, hasNestedHooks, nil
}

func writeWindsurfHookConfig(path string, rawSettings map[string]json.RawMessage, rawHooks map[string]json.RawMessage) error {
	if rawSettings == nil {
		rawSettings = make(map[string]json.RawMessage)
	}
	if rawHooks == nil {
		rawHooks = make(map[string]json.RawMessage)
	}

	// Remove legacy top-level action keys to keep the file in canonical Windsurf format.
	for _, key := range windsurfActionKeys {
		delete(rawSettings, key)
	}

	if len(rawHooks) == 0 {
		delete(rawSettings, windsurfHooksRootKey)
	} else {
		hooksSectionRaw, err := json.Marshal(rawHooks)
		if err != nil {
			return fmt.Errorf("failed to marshal hooks section: %w", err)
		}
		rawSettings[windsurfHooksRootKey] = hooksSectionRaw
	}

	//nolint:gosec // Repo-local config directory.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create .windsurf directory: %w", err)
	}

	data, err := json.MarshalIndent(rawSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write hooks.json: %w", err)
	}
	return nil
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
