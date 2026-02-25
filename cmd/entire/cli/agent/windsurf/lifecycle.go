package windsurf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/validation"
)

// Compile-time interface assertions for supported capabilities.
var _ agent.TranscriptAnalyzer = (*WindsurfAgent)(nil)

// HookNames returns Windsurf hook verbs supported by Entire.
func (a *WindsurfAgent) HookNames() []string {
	return []string{
		HookNamePreUserPrompt,
		HookNamePostWriteCode,
		HookNamePostCascadeResponse,
	}
}

// ParseHookEvent maps Windsurf hooks to normalized lifecycle events.
func (a *WindsurfAgent) ParseHookEvent(hookName string, stdin io.Reader) (*agent.Event, error) {
	rawInput, err := readHookInputBytes(stdin)
	if err != nil {
		return nil, err
	}

	var input hookInputRaw
	if err := json.Unmarshal(rawInput, &input); err != nil {
		return nil, fmt.Errorf("failed to parse hook input: %w", err)
	}
	if input.TrajectoryID == "" {
		return nil, fmt.Errorf("missing trajectory_id in %s hook", hookName)
	}
	if err := validation.ValidateSessionID(input.TrajectoryID); err != nil {
		return nil, fmt.Errorf("invalid trajectory_id: %w", err)
	}

	sessionRef, err := a.sessionRefForTrajectory(input.TrajectoryID)
	if err != nil {
		return nil, err
	}
	if err := appendHookPayload(sessionRef, rawInput); err != nil {
		return nil, err
	}

	switch hookName {
	case HookNamePreUserPrompt:
		return a.parseTurnStart(sessionRef, &input)
	case HookNamePostCascadeResponse:
		return &agent.Event{
			Type:       agent.TurnEnd,
			SessionID:  input.TrajectoryID,
			SessionRef: sessionRef,
			Timestamp:  time.Now(),
		}, nil
	case HookNamePostWriteCode:
		// Capture-only hook for file extraction from transcript.
		return nil, nil //nolint:nilnil // No lifecycle transition for this hook.
	default:
		return nil, nil //nolint:nilnil // Unknown hooks are no-ops.
	}
}

func (a *WindsurfAgent) parseTurnStart(sessionRef string, input *hookInputRaw) (*agent.Event, error) {
	var info preUserPromptInfo
	if len(input.ToolInfo) > 0 {
		if err := json.Unmarshal(input.ToolInfo, &info); err != nil {
			return nil, fmt.Errorf("failed to parse pre_user_prompt tool_info: %w", err)
		}
	}

	return &agent.Event{
		Type:       agent.TurnStart,
		SessionID:  input.TrajectoryID,
		SessionRef: sessionRef,
		Prompt:     info.UserPrompt,
		Timestamp:  time.Now(),
	}, nil
}

func (a *WindsurfAgent) sessionRefForTrajectory(trajectoryID string) (string, error) {
	repoRoot, err := paths.WorktreeRoot()
	if err != nil {
		//nolint:forbidigo // Fallback for tests that run outside a git repo.
		repoRoot, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to determine working directory: %w", err)
		}
	}
	sessionDir, err := a.GetSessionDir(repoRoot)
	if err != nil {
		return "", err
	}
	return a.ResolveSessionFile(sessionDir, trajectoryID), nil
}

func appendHookPayload(sessionRef string, payload []byte) error {
	line := bytes.TrimSpace(payload)
	if len(line) == 0 {
		return nil
	}

	//nolint:gosec // Session transcript path is repository-local metadata.
	if err := os.MkdirAll(filepath.Dir(sessionRef), 0o755); err != nil {
		return fmt.Errorf("failed to create transcript directory: %w", err)
	}

	//nolint:gosec // Session transcript file is repository-local metadata.
	f, err := os.OpenFile(sessionRef, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open transcript file: %w", err)
	}
	defer f.Close() //nolint:errcheck // Best effort.

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("failed to append transcript line: %w", err)
	}
	if _, err := f.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to append transcript newline: %w", err)
	}

	return nil
}

func readHookInputBytes(stdin io.Reader) ([]byte, error) {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return nil, fmt.Errorf("failed to read hook input: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("empty hook input")
	}
	return data, nil
}
