package appcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureManagedClaudeHookAddsToEmptySettings(t *testing.T) {
	settings, err := ensureManagedClaudeHook(nil, "/home/test/.lrc/claude/hooks/blocking-review-git-commit.sh")
	if err != nil {
		t.Fatalf("ensureManagedClaudeHook() error = %v", err)
	}

	text := string(settings)
	if !strings.Contains(text, `"PreToolUse"`) {
		t.Fatalf("expected PreToolUse section in %s", text)
	}
	if !strings.Contains(text, claudeManagedCondition) {
		t.Fatalf("expected managed condition in %s", text)
	}
	if !strings.Contains(text, claudeManagedStatusMessage) {
		t.Fatalf("expected managed status message in %s", text)
	}
	if !hasManagedClaudeHook(settings) {
		t.Fatal("expected managed Claude hook to be detected")
	}
}

func TestEnsureManagedClaudeHookPreservesExistingHooks(t *testing.T) {
	existing := []byte(`{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "if": "Bash(git status)",
            "command": "/tmp/existing.sh"
          }
        ]
      }
    ]
  }
}
`)

	settings, err := ensureManagedClaudeHook(existing, "/home/test/.lrc/claude/hooks/blocking-review-git-commit.sh")
	if err != nil {
		t.Fatalf("ensureManagedClaudeHook() error = %v", err)
	}

	text := string(settings)
	if !strings.Contains(text, `"Bash(git status)"`) {
		t.Fatalf("expected existing hook to be preserved in %s", text)
	}
	if !strings.Contains(text, claudeManagedCondition) {
		t.Fatalf("expected managed hook to be added in %s", text)
	}
}

func TestRemoveManagedClaudeHookPreservesUnrelatedHooks(t *testing.T) {
	settings, changed, err := removeManagedClaudeHook([]byte(`{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "if": "Bash(git status)",
            "command": "/tmp/existing.sh"
          },
          {
            "type": "command",
            "if": "Bash(git commit *)",
            "command": "/home/test/.lrc/claude/hooks/blocking-review-git-commit.sh",
            "statusMessage": "Running blocking LiveReview gate before git commit"
          }
        ]
      }
    ]
  }
}
`))
	if err != nil {
		t.Fatalf("removeManagedClaudeHook() error = %v", err)
	}
	if !changed {
		t.Fatal("expected managed hook removal to report a change")
	}

	text := string(settings)
	if strings.Contains(text, claudeManagedCondition) {
		t.Fatalf("expected managed hook to be removed from %s", text)
	}
	if !strings.Contains(text, `"Bash(git status)"`) {
		t.Fatalf("expected unrelated hook to remain in %s", text)
	}
	if hasManagedClaudeHook(settings) {
		t.Fatal("expected managed Claude hook to be absent after removal")
	}
}

func TestIsManagedClaudeCommandPath(t *testing.T) {
	if !isManagedClaudeCommandPath(`/home/test/.lrc/claude/hooks/blocking-review-git-commit.sh`) {
		t.Fatal("expected managed Claude command path to match")
	}
	if isManagedClaudeCommandPath(`/tmp/blocking-review-git-commit.sh`) {
		t.Fatal("did not expect unrelated path to match")
	}
}

func TestGenerateClaudeLRCSkillContainsCanonicalCommands(t *testing.T) {
	skill := generateClaudeLRCSkill()
	if strings.Contains(skill, "\t") {
		t.Fatal("expected generated Claude skill to avoid tabs so YAML frontmatter stays parseable")
	}
	for _, fragment := range []string{
		"name: lrc",
		"lrc hooks status",
		"lrc hooks disable --surface claude",
		"lrc hooks install --surface claude",
		"Never use skip as a fallback",
		"Never disable hooks as a fallback",
	} {
		if !strings.Contains(skill, fragment) {
			t.Fatalf("expected skill to contain %q", fragment)
		}
	}
}

func TestGenerateGlobalClaudeWrapperScriptAllowsMissingReviewModeVersionLine(t *testing.T) {
	script := generateGlobalClaudeWrapperScript()

	if strings.Contains(script, `unable to determine lrc review mode`) {
		t.Fatal("expected wrapper to tolerate lrc binaries that omit the Review mode line")
	}
	if !strings.Contains(script, `if [[ "$lrc_review_mode" == "fake" ]]; then`) {
		t.Fatal("expected wrapper to keep rejecting fake-review binaries")
	}
}

func TestGenerateGlobalClaudeWrapperScriptExtractsHeredocCommitMessage(t *testing.T) {
	script := generateGlobalClaudeWrapperScript()

	for _, fragment := range []string{
		`import re`,
		`def extract_heredoc_command_substitution(value):`,
		`cat\s+<<-?`,
		`resolved_message = extract_heredoc_command_substitution(message)`,
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected wrapper script to contain %q", fragment)
		}
	}
}

func TestRemoveLegacyRepoClaudeIntegration(t *testing.T) {
	repoRoot := t.TempDir()
	claudeDir := filepath.Join(repoRoot, ".claude")
	hooksDir := filepath.Join(claudeDir, "hooks")

	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	legacyFiles := []string{
		filepath.Join(claudeDir, "settings.local.json"),
		filepath.Join(hooksDir, claudeManagedValidatorName),
		filepath.Join(hooksDir, claudeManagedWrapperName),
	}
	for _, path := range legacyFiles {
		if err := os.WriteFile(path, []byte("legacy"), 0644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}

	removed, err := removeLegacyRepoClaudeIntegration(repoRoot)
	if err != nil {
		t.Fatalf("removeLegacyRepoClaudeIntegration() error = %v", err)
	}
	if len(removed) != len(legacyFiles) {
		t.Fatalf("removed count = %d, want %d", len(removed), len(legacyFiles))
	}

	for _, path := range legacyFiles {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed, stat err = %v", path, err)
		}
	}

	if _, err := os.Stat(hooksDir); !os.IsNotExist(err) {
		t.Fatalf("expected hooks dir to be removed when empty, stat err = %v", err)
	}
	if _, err := os.Stat(claudeDir); !os.IsNotExist(err) {
		t.Fatalf("expected .claude dir to be removed when empty, stat err = %v", err)
	}
}
