package hooks

import (
	"strings"
	"testing"
)

func testTemplateConfig() TemplateConfig {
	return TemplateConfig{
		MarkerBegin:       "# BEGIN lrc managed section - DO NOT EDIT",
		MarkerEnd:         "# END lrc managed section",
		Version:           "test-version",
		CommitMessageFile: "livereview_commit_message",
		PushRequestFile:   "livereview_push_request",
	}
}

func TestGeneratedHooksUseResolvedGitDirPaths(t *testing.T) {
	cfg := testTemplateConfig()

	tests := []struct {
		name      string
		hook      string
		contains  []string
		forbidden []string
	}{
		{
			name: "pre-commit",
			hook: GeneratePreCommitHook(cfg),
			contains: []string{
				"GIT_DIR=\"$(git rev-parse --git-dir 2>/dev/null || echo .git)\"",
				"LRC_DIR=\"$GIT_DIR/lrc\"",
				"ATTEST_FILE=\"$LRC_DIR/attestations/$TREE_HASH.json\"",
			},
			forbidden: []string{
				"DISABLED_FILE=\".git/lrc/disabled\"",
				"ATTEST_FILE=\".git/lrc/attestations/$TREE_HASH.json\"",
			},
		},
		{
			name: "prepare-commit-msg",
			hook: GeneratePrepareCommitMsgHook(cfg),
			contains: []string{
				"GIT_DIR=\"$(git rev-parse --git-dir 2>/dev/null || echo .git)\"",
				"STATE_FILE=\"$GIT_DIR/livereview_state\"",
				"LOCK_DIR=\"$GIT_DIR/livereview_state.lock\"",
				"INITIAL_MSG_FILE=\"$GIT_DIR/livereview_initial_message.$$\"",
			},
			forbidden: []string{
				"LRC_DIR=\".git/lrc\"",
				"STATE_FILE=\".git/livereview_state\"",
				"LOCK_DIR=\".git/livereview_state.lock\"",
				"INITIAL_MSG_FILE=\".git/livereview_initial_message.$$\"",
			},
		},
		{
			name: "commit-msg",
			hook: GenerateCommitMsgHook(cfg),
			contains: []string{
				"GIT_DIR=\"$(git rev-parse --git-dir 2>/dev/null || echo .git)\"",
				"COMMIT_MSG_OVERRIDE=\"$GIT_DIR/livereview_commit_message\"",
				"STATE_FILE=\"$GIT_DIR/livereview_state\"",
			},
			forbidden: []string{
				"COMMIT_MSG_OVERRIDE=\".git/livereview_commit_message\"",
				"LRC_DIR=\".git/lrc\"",
				"STATE_FILE=\".git/livereview_state\"",
			},
		},
		{
			name: "post-commit",
			hook: GeneratePostCommitHook(cfg),
			contains: []string{
				"GIT_DIR=\"$(git rev-parse --git-dir 2>/dev/null || echo .git)\"",
				"PUSH_FLAG=\"$GIT_DIR/livereview_push_request\"",
				"LRC_DIR=\"$GIT_DIR/lrc\"",
			},
			forbidden: []string{
				"PUSH_FLAG=\".git/livereview_push_request\"",
				"LRC_DIR=\".git/lrc\"",
			},
		},
		{
			name: "dispatcher",
			hook: GenerateDispatcherHook("pre-commit", cfg),
			contains: []string{
				"GIT_DIR=\"$(git rev-parse --git-dir 2>/dev/null || echo .git)\"",
				"GIT_COMMON_DIR=\"$(git rev-parse --git-common-dir 2>/dev/null || echo \"$GIT_DIR\")\"",
				"LRC_DISABLED_FILE=\"$GIT_DIR/lrc/disabled\"",
				"LOCAL_HOOK=\"$GIT_COMMON_DIR/hooks/pre-commit\"",
			},
			forbidden: []string{
				"LRC_DISABLED_FILE=\".git/lrc/disabled\"",
				"LOCAL_HOOK=\"$(git rev-parse --git-path hooks/pre-commit 2>/dev/null || echo .git/hooks/pre-commit)\"",
				"LOCAL_HOOK=\".git/hooks/pre-commit\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, want := range tt.contains {
				if !strings.Contains(tt.hook, want) {
					t.Fatalf("expected generated %s hook to contain %q", tt.name, want)
				}
			}
			for _, forbidden := range tt.forbidden {
				if strings.Contains(tt.hook, forbidden) {
					t.Fatalf("did not expect generated %s hook to contain %q", tt.name, forbidden)
				}
			}
		})
	}
}
