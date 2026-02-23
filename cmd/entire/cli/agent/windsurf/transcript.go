package windsurf

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ParseEvents parses Windsurf hook JSONL transcript bytes.
// Invalid lines are skipped to preserve resilience to partial writes.
func ParseEvents(data []byte) ([]hookInputRaw, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var events []hookInputRaw
	reader := bufio.NewReader(bytes.NewReader(data))

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read windsurf transcript: %w", err)
		}

		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			var event hookInputRaw
			if jsonErr := json.Unmarshal(trimmed, &event); jsonErr == nil {
				events = append(events, event)
			}
		}

		if err == io.EOF {
			break
		}
	}

	return events, nil
}

func parseEventsFromFile(path string) ([]hookInputRaw, error) {
	data, err := os.ReadFile(path) //nolint:gosec // Path comes from hook input/metadata.
	if err != nil {
		return nil, err //nolint:wrapcheck // Callers need to test os.IsNotExist.
	}
	return ParseEvents(data)
}

// GetTranscriptPosition returns the current transcript position for incremental parsing.
// If the latest event is pre_user_prompt, it returns the position before that event so
// TurnStart capture can include the current prompt at TurnEnd extraction time.
func (a *WindsurfAgent) GetTranscriptPosition(path string) (int, error) {
	events, err := parseEventsFromFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	if len(events) > 0 && events[len(events)-1].eventName() == actionPreUserPrompt {
		return len(events) - 1, nil
	}
	return len(events), nil
}

// ExtractModifiedFilesFromOffset extracts file paths from post_write_code events.
func (a *WindsurfAgent) ExtractModifiedFilesFromOffset(path string, startOffset int) ([]string, int, error) {
	events, err := parseEventsFromFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, err
	}

	files := extractModifiedFilesFromEvents(events, startOffset)
	return files, len(events), nil
}

// ExtractPrompts extracts pre_user_prompt prompts from a transcript.
func (a *WindsurfAgent) ExtractPrompts(sessionRef string, fromOffset int) ([]string, error) {
	events, err := parseEventsFromFile(sessionRef)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	return extractPromptsFromEvents(events, fromOffset), nil
}

// ExtractSummary extracts the latest cascade_response text.
func (a *WindsurfAgent) ExtractSummary(sessionRef string) (string, error) {
	events, err := parseEventsFromFile(sessionRef)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	for i := len(events) - 1; i >= 0; i-- {
		name := events[i].eventName()
		if name != actionPostCascadeResponse {
			continue
		}
		var info postCascadeResponseInfo
		if err := json.Unmarshal(events[i].ToolInfo, &info); err == nil && info.CascadeResponse != "" {
			return info.CascadeResponse, nil
		}
	}
	return "", nil
}

// ExtractModifiedFiles extracts modified files from raw JSONL transcript bytes.
func ExtractModifiedFiles(data []byte) ([]string, error) {
	events, err := ParseEvents(data)
	if err != nil {
		return nil, err
	}
	return extractModifiedFilesFromEvents(events, 0), nil
}

// ExtractAllUserPrompts extracts all user prompts from raw JSONL transcript bytes.
func ExtractAllUserPrompts(data []byte) ([]string, error) {
	events, err := ParseEvents(data)
	if err != nil {
		return nil, err
	}
	return extractPromptsFromEvents(events, 0), nil
}

func extractModifiedFilesFromEvents(events []hookInputRaw, startOffset int) []string {
	seen := make(map[string]bool)
	var files []string

	if startOffset < 0 {
		startOffset = 0
	}
	for i := startOffset; i < len(events); i++ {
		name := events[i].eventName()
		if name != actionPostWriteCode {
			continue
		}

		var info postWriteCodeInfo
		if err := json.Unmarshal(events[i].ToolInfo, &info); err != nil {
			continue
		}
		if info.FilePath == "" || seen[info.FilePath] {
			continue
		}
		seen[info.FilePath] = true
		files = append(files, info.FilePath)
	}
	return files
}

func extractPromptsFromEvents(events []hookInputRaw, fromOffset int) []string {
	if fromOffset < 0 {
		fromOffset = 0
	}

	var prompts []string
	for i := fromOffset; i < len(events); i++ {
		name := events[i].eventName()
		if name != actionPreUserPrompt {
			continue
		}

		var info preUserPromptInfo
		if err := json.Unmarshal(events[i].ToolInfo, &info); err != nil {
			continue
		}
		if info.UserPrompt != "" {
			prompts = append(prompts, info.UserPrompt)
		}
	}
	return prompts
}
