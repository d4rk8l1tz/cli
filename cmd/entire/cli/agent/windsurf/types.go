package windsurf

import "encoding/json"

// Hook names exposed as CLI subcommands under `entire hooks windsurf`.
const (
	HookNamePreUserPrompt       = "pre-user-prompt"
	HookNamePostWriteCode       = "post-write-code"
	HookNamePostCascadeResponse = "post-cascade-response"
)

// Windsurf action names from hook payloads (`agent_action_name`).
const (
	actionPreUserPrompt       = "pre_user_prompt"
	actionPostWriteCode       = "post_write_code"
	actionPostCascadeResponse = "post_cascade_response"
)

// WindsurfHooksFileName is the workspace-level hooks config file.
const WindsurfHooksFileName = "hooks.json"

// WindsurfHookConfig is a single hook command entry in .windsurf/hooks.json.
// Additional fields are optional configuration flags supported by Windsurf.
type WindsurfHookConfig struct {
	Command          string `json:"command"`
	ShowOutput       *bool  `json:"show_output,omitempty"`
	WorkingDirectory string `json:"working_directory,omitempty"`
}

// hookInputRaw is the common payload shape for Windsurf hook events.
// Field names match the hook docs exactly for native-format preservation.
type hookInputRaw struct {
	AgentActionName string          `json:"agent_action_name,omitempty"`
	HookEventName   string          `json:"hook_event_name,omitempty"` // Backward compatibility.
	Timestamp       string          `json:"timestamp,omitempty"`
	Cwd             string          `json:"cwd,omitempty"`
	TrajectoryID    string          `json:"trajectory_id"`
	ExecutionID     string          `json:"execution_id,omitempty"`
	ToolCallID      string          `json:"tool_call_id,omitempty"`
	ToolInfo        json.RawMessage `json:"tool_info,omitempty"`
}

func (h hookInputRaw) eventName() string {
	if h.AgentActionName != "" {
		return h.AgentActionName
	}
	return h.HookEventName
}

// preUserPromptInfo is the tool_info payload for pre_user_prompt.
type preUserPromptInfo struct {
	UserPrompt string `json:"user_prompt"`
}

// postWriteCodeInfo is the tool_info payload for post_write_code.
type postWriteCodeInfo struct {
	FilePath string `json:"file_path"`
}

// postCascadeResponseInfo is the tool_info payload for post_cascade_response.
type postCascadeResponseInfo struct {
	CascadeResponse string `json:"cascade_response"`
}
