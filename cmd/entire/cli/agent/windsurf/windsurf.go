// Package windsurf implements the Agent interface for Windsurf Cascade hooks.
package windsurf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	"github.com/entireio/cli/cmd/entire/cli/paths"
)

//nolint:gochecknoinits // Agent self-registration is the intended pattern
func init() {
	agent.Register(agent.AgentNameWindsurf, NewWindsurfAgent)
}

// WindsurfAgent implements the Agent interface for Windsurf Cascade.
//
//nolint:revive // WindsurfAgent is clearer than Agent in this context
type WindsurfAgent struct{}

// NewWindsurfAgent creates a new Windsurf agent instance.
func NewWindsurfAgent() agent.Agent {
	return &WindsurfAgent{}
}

// --- Identity ---

func (a *WindsurfAgent) Name() agent.AgentName { return agent.AgentNameWindsurf }
func (a *WindsurfAgent) Type() agent.AgentType { return agent.AgentTypeWindsurf }
func (a *WindsurfAgent) Description() string {
	return "Windsurf Cascade - Codeium's AI coding assistant"
}
func (a *WindsurfAgent) IsPreview() bool         { return true }
func (a *WindsurfAgent) ProtectedDirs() []string { return []string{".windsurf"} }

// DetectPresence checks if Windsurf is configured in the repository.
func (a *WindsurfAgent) DetectPresence() (bool, error) {
	repoRoot, err := paths.WorktreeRoot()
	if err != nil {
		repoRoot = "."
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".windsurf")); err == nil {
		return true, nil
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".windsurf", WindsurfHooksFileName)); err == nil {
		return true, nil
	}
	return false, nil
}

// --- Transcript Storage ---

func (a *WindsurfAgent) ReadTranscript(sessionRef string) ([]byte, error) {
	data, err := os.ReadFile(sessionRef) //nolint:gosec // Path comes from hook input/metadata
	if err != nil {
		return nil, fmt.Errorf("failed to read windsurf transcript: %w", err)
	}
	return data, nil
}

func (a *WindsurfAgent) ChunkTranscript(content []byte, maxSize int) ([][]byte, error) {
	chunks, err := agent.ChunkJSONL(content, maxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk windsurf transcript: %w", err)
	}
	return chunks, nil
}

func (a *WindsurfAgent) ReassembleTranscript(chunks [][]byte) ([]byte, error) {
	return agent.ReassembleJSONL(chunks), nil
}

// --- Legacy methods ---

func (a *WindsurfAgent) GetSessionID(input *agent.HookInput) string {
	return input.SessionID
}

// GetSessionDir returns the directory where Entire stores Windsurf hook transcripts.
func (a *WindsurfAgent) GetSessionDir(repoPath string) (string, error) {
	root := strings.TrimSpace(repoPath)
	if root == "" {
		repoRoot, err := paths.WorktreeRoot()
		if err != nil {
			return "", fmt.Errorf("failed to get repo root: %w", err)
		}
		root = repoRoot
	}
	return filepath.Join(root, paths.EntireDir, "tmp", "windsurf"), nil
}

func (a *WindsurfAgent) ResolveSessionFile(sessionDir, agentSessionID string) string {
	return filepath.Join(sessionDir, sanitizeSessionIDForPath(agentSessionID)+".jsonl")
}

func (a *WindsurfAgent) ReadSession(input *agent.HookInput) (*agent.AgentSession, error) {
	if input == nil {
		return nil, errors.New("hook input is nil")
	}
	if input.SessionRef == "" {
		return nil, errors.New("session reference (transcript path) is required")
	}

	data, err := os.ReadFile(input.SessionRef)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript: %w", err)
	}

	modifiedFiles, err := ExtractModifiedFiles(data)
	if err != nil {
		// Non-fatal: retain native data even if extraction fails.
		modifiedFiles = nil
	}

	return &agent.AgentSession{
		SessionID:     input.SessionID,
		AgentName:     a.Name(),
		SessionRef:    input.SessionRef,
		NativeData:    data,
		ModifiedFiles: modifiedFiles,
	}, nil
}

func (a *WindsurfAgent) WriteSession(session *agent.AgentSession) error {
	if session == nil {
		return errors.New("session is nil")
	}
	if session.AgentName != "" && session.AgentName != a.Name() {
		return fmt.Errorf("session belongs to agent %q, not %q", session.AgentName, a.Name())
	}
	if session.SessionRef == "" {
		return errors.New("session reference (transcript path) is required")
	}
	if len(session.NativeData) == 0 {
		return errors.New("session has no native data to write")
	}

	dir := filepath.Dir(session.SessionRef)
	//nolint:gosec // Session directory is repository-local metadata.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	if err := os.WriteFile(session.SessionRef, session.NativeData, 0o600); err != nil {
		return fmt.Errorf("failed to write transcript: %w", err)
	}
	return nil
}

func (a *WindsurfAgent) FormatResumeCommand(_ string) string {
	// Windsurf doesn't currently expose a session-id-based resume CLI command.
	return "windsurf"
}

var nonPathSafeChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func sanitizeSessionIDForPath(sessionID string) string {
	clean := nonPathSafeChars.ReplaceAllString(sessionID, "-")
	clean = strings.Trim(clean, "-")
	if clean == "" {
		return "windsurf-session"
	}
	return clean
}
