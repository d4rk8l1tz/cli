//go:build integration

package integration

import (
	"strings"
	"testing"

	"github.com/entireio/cli/cmd/entire/cli/agent"
	_ "github.com/entireio/cli/cmd/entire/cli/agent/windsurf" // Register Windsurf agent
	"github.com/entireio/cli/cmd/entire/cli/paths"
	"github.com/entireio/cli/cmd/entire/cli/strategy"
)

// TestWindsurfHookFlow verifies the full hook flow for Windsurf:
// pre-user-prompt -> post-write-code -> post-cascade-response -> checkpoint -> commit/condense.
func TestWindsurfHookFlow(t *testing.T) {
	t.Parallel()

	RunForAllStrategies(t, func(t *testing.T, env *TestEnv, strategyName string) {
		env.InitEntireWithAgent(strategyName, agent.AgentNameWindsurf)

		session := env.NewWindsurfSession()

		// 1. pre-user-prompt (equivalent to turn start)
		if err := env.SimulateWindsurfPreUserPrompt(session.ID, "Add a feature"); err != nil {
			t.Fatalf("pre-user-prompt error: %v", err)
		}

		// 2. Agent writes file(s) after turn start
		env.WriteFile("feature.go", "package main\n// new feature")
		if err := env.SimulateWindsurfPostWriteCode(session.ID, "feature.go"); err != nil {
			t.Fatalf("post-write-code error: %v", err)
		}

		// 3. post-cascade-response (equivalent to turn end)
		if err := env.SimulateWindsurfPostCascadeResponse(session.ID, "Done!"); err != nil {
			t.Fatalf("post-cascade-response error: %v", err)
		}

		// 4. Verify checkpoint exists
		points := env.GetRewindPoints()
		if len(points) == 0 {
			t.Fatal("expected at least 1 rewind point after post-cascade-response")
		}

		// Regression: Windsurf pre_user_prompt should produce a non-default commit message
		// in auto-commit mode (prompt extraction must include the current turn prompt).
		if strategyName == strategy.StrategyNameAutoCommit {
			headMessage := env.GetCommitMessage(env.GetHeadHash())
			if strings.Contains(headMessage, "Claude Code session updates") {
				t.Fatalf("auto-commit used fallback message, prompt extraction likely failed: %q", headMessage)
			}
		}

		// 5. For manual-commit, user commit triggers condensation. Auto-commit already committed.
		if strategyName == strategy.StrategyNameManualCommit {
			env.GitCommitWithShadowHooks("Add feature", "feature.go")
		}

		// 6. Verify checkpoint is available on metadata branch
		checkpointID := env.TryGetLatestCheckpointID()
		if checkpointID == "" {
			t.Fatal("expected checkpoint on metadata branch")
		}

		transcriptPath := SessionFilePath(checkpointID, paths.TranscriptFileName)
		if _, found := env.ReadFileFromBranch(paths.MetadataBranchName, transcriptPath); !found {
			t.Error("condensed transcript should exist on metadata branch")
		}
	})
}

// TestWindsurfAgentStrategyComposition verifies Windsurf agent parsing and strategy checkpointing composition.
func TestWindsurfAgentStrategyComposition(t *testing.T) {
	t.Parallel()

	RunForAllStrategies(t, func(t *testing.T, env *TestEnv, strategyName string) {
		env.InitEntireWithAgent(strategyName, agent.AgentNameWindsurf)

		ag, err := agent.Get("windsurf")
		if err != nil {
			t.Fatalf("Get(windsurf) error = %v", err)
		}

		if _, err := strategy.Get(strategyName); err != nil {
			t.Fatalf("Get(%s) error = %v", strategyName, err)
		}

		session := env.NewWindsurfSession()
		transcriptPath := session.CreateWindsurfTranscript("Add a feature", []FileChange{
			{Path: "feature.go", Content: "package main\n// new feature"},
		})

		agentSession, err := ag.ReadSession(&agent.HookInput{
			SessionID:  session.ID,
			SessionRef: transcriptPath,
		})
		if err != nil {
			t.Fatalf("ReadSession() error = %v", err)
		}

		if len(agentSession.ModifiedFiles) == 0 {
			t.Error("agent.ReadSession() should compute ModifiedFiles")
		}

		// Simulate hook flow to verify strategy integration
		if err := env.SimulateWindsurfPreUserPrompt(session.ID, "Add a feature"); err != nil {
			t.Fatalf("pre-user-prompt error = %v", err)
		}
		env.WriteFile("feature.go", "package main\n// new feature")
		if err := env.SimulateWindsurfPostWriteCode(session.ID, "feature.go"); err != nil {
			t.Fatalf("post-write-code error = %v", err)
		}
		if err := env.SimulateWindsurfPostCascadeResponse(session.ID, "Done!"); err != nil {
			t.Fatalf("post-cascade-response error = %v", err)
		}

		points := env.GetRewindPoints()
		if len(points) == 0 {
			t.Fatal("expected at least 1 rewind point after hook flow")
		}
	})
}

// TestWindsurfRewind verifies rewind behavior for Windsurf checkpoints.
func TestWindsurfRewind(t *testing.T) {
	t.Parallel()

	env := NewFeatureBranchEnv(t, strategy.StrategyNameManualCommit)
	env.InitEntireWithAgent(strategy.StrategyNameManualCommit, agent.AgentNameWindsurf)

	session := env.NewWindsurfSession()

	// Turn 1: create file1
	if err := env.SimulateWindsurfPreUserPrompt(session.ID, "Create file1"); err != nil {
		t.Fatalf("pre-user-prompt turn 1 error: %v", err)
	}
	env.WriteFile("file1.go", "package main\n// file1 v1")
	if err := env.SimulateWindsurfPostWriteCode(session.ID, "file1.go"); err != nil {
		t.Fatalf("post-write-code turn 1 error: %v", err)
	}
	if err := env.SimulateWindsurfPostCascadeResponse(session.ID, "Done turn 1"); err != nil {
		t.Fatalf("post-cascade-response turn 1 error: %v", err)
	}

	points1 := env.GetRewindPoints()
	if len(points1) == 0 {
		t.Fatal("no rewind point after first turn")
	}
	checkpoint1ID := points1[0].ID

	// Turn 2: modify file1 and create file2
	if err := env.SimulateWindsurfPreUserPrompt(session.ID, "Modify file1 and create file2"); err != nil {
		t.Fatalf("pre-user-prompt turn 2 error: %v", err)
	}
	env.WriteFile("file1.go", "package main\n// file1 v2")
	env.WriteFile("file2.go", "package main\n// file2")
	if err := env.SimulateWindsurfPostWriteCode(session.ID, "file1.go"); err != nil {
		t.Fatalf("post-write-code file1 turn 2 error: %v", err)
	}
	if err := env.SimulateWindsurfPostWriteCode(session.ID, "file2.go"); err != nil {
		t.Fatalf("post-write-code file2 turn 2 error: %v", err)
	}
	if err := env.SimulateWindsurfPostCascadeResponse(session.ID, "Done turn 2"); err != nil {
		t.Fatalf("post-cascade-response turn 2 error: %v", err)
	}

	points2 := env.GetRewindPoints()
	if len(points2) < 2 {
		t.Fatalf("expected at least 2 rewind points, got %d", len(points2))
	}

	if err := env.Rewind(checkpoint1ID); err != nil {
		t.Fatalf("Rewind() error = %v", err)
	}

	content := env.ReadFile("file1.go")
	if content != "package main\n// file1 v1" {
		t.Errorf("file1.go after rewind = %q, want v1 content", content)
	}
	if env.FileExists("file2.go") {
		t.Error("file2.go should not exist after rewind to checkpoint 1")
	}
}

// TestWindsurfMultiTurnCondensation verifies Windsurf checkpoint condensation after commit.
func TestWindsurfMultiTurnCondensation(t *testing.T) {
	t.Parallel()

	env := NewFeatureBranchEnv(t, strategy.StrategyNameManualCommit)
	env.InitEntireWithAgent(strategy.StrategyNameManualCommit, agent.AgentNameWindsurf)

	session := env.NewWindsurfSession()

	if err := env.SimulateWindsurfPreUserPrompt(session.ID, "Create app.go"); err != nil {
		t.Fatalf("pre-user-prompt error: %v", err)
	}
	env.WriteFile("app.go", "package main\nfunc main() {}")
	if err := env.SimulateWindsurfPostWriteCode(session.ID, "app.go"); err != nil {
		t.Fatalf("post-write-code error: %v", err)
	}
	if err := env.SimulateWindsurfPostCascadeResponse(session.ID, "Implemented app"); err != nil {
		t.Fatalf("post-cascade-response error: %v", err)
	}

	points := env.GetRewindPoints()
	if len(points) == 0 {
		t.Fatal("expected rewind point after turn")
	}

	env.GitCommitWithShadowHooks("Implement app", "app.go")

	checkpointID := env.TryGetLatestCheckpointID()
	if checkpointID == "" {
		t.Fatal("expected checkpoint on metadata branch after commit")
	}

	env.ValidateCheckpoint(CheckpointValidation{
		CheckpointID: checkpointID,
		Strategy:     strategy.StrategyNameManualCommit,
		FilesTouched: []string{"app.go"},
		ExpectedTranscriptContent: []string{
			"Create app.go",
		},
	})
}
