package appcore

import (
	"os"
	"testing"
	"time"

	"github.com/HexmosTech/git-lrc/internal/reviewapi"
	"github.com/HexmosTech/git-lrc/internal/reviewmodel"
)

func TestIsFakeReviewBuild(t *testing.T) {
	oldMode := reviewMode
	defer func() { reviewMode = oldMode }()

	reviewMode = "fake"
	if !isFakeReviewBuild() {
		t.Fatalf("expected fake mode to be enabled")
	}

	reviewMode = "prod"
	if isFakeReviewBuild() {
		t.Fatalf("expected fake mode to be disabled")
	}
}

func TestFakeReviewWaitDuration(t *testing.T) {
	const envKey = "LRC_FAKE_REVIEW_WAIT"
	old := os.Getenv(envKey)
	defer func() {
		if old == "" {
			_ = os.Unsetenv(envKey)
			return
		}
		_ = os.Setenv(envKey, old)
	}()

	_ = os.Unsetenv(envKey)
	d, err := fakeReviewWaitDuration()
	if err != nil {
		t.Fatalf("unexpected error for default wait: %v", err)
	}
	if d != 30*time.Second {
		t.Fatalf("default wait = %s, want %s", d, 30*time.Second)
	}

	if err := os.Setenv(envKey, "3s"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	d, err = fakeReviewWaitDuration()
	if err != nil {
		t.Fatalf("unexpected error for valid wait: %v", err)
	}
	if d != 3*time.Second {
		t.Fatalf("wait = %s, want %s", d, 3*time.Second)
	}

	if err := os.Setenv(envKey, "not-a-duration"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	if _, err := fakeReviewWaitDuration(); err == nil {
		t.Fatalf("expected error for invalid duration")
	}

	if err := os.Setenv(envKey, "0s"); err != nil {
		t.Fatalf("setenv failed: %v", err)
	}
	if _, err := fakeReviewWaitDuration(); err == nil {
		t.Fatalf("expected error for zero duration")
	}
}

func TestBuildFakeCompletedResult(t *testing.T) {
	result := buildFakeCompletedResult()
	if result == nil {
		t.Fatalf("expected fake result")
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.Summary == "" {
		t.Fatalf("expected non-empty fake summary")
	}
	if len(result.Files) != 0 {
		t.Fatalf("expected zero files in base fake result, got %d", len(result.Files))
	}
}

func TestBuildFakeCompletedResultForFiles(t *testing.T) {
	baseFiles := []reviewmodel.DiffReviewFileResult{
		{
			FilePath: "src/ui_connectors_handlers.go",
			Hunks: []reviewmodel.DiffReviewHunk{
				{
					OldStartLine: 1,
					OldLineCount: 0,
					NewStartLine: 1,
					NewLineCount: 3,
					Content:      "@@ -0,0 +1,3 @@\n+func handleConnector(payload map[string]any) {\n+\tpayload[\"provider\"] = \"live\"\n+}\n",
				},
			},
		},
		{
			FilePath: "src/only_one_line.txt",
			Hunks: []reviewmodel.DiffReviewHunk{
				{
					OldStartLine: 0,
					OldLineCount: 0,
					NewStartLine: 1,
					NewLineCount: 1,
					Content:      "@@ -0,0 +1 @@\n+single-line\n",
				},
			},
		},
	}

	result := buildFakeCompletedResultForFiles(baseFiles)
	if result == nil {
		t.Fatalf("expected fake result")
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if len(result.Files) != len(baseFiles) {
		t.Fatalf("files len = %d, want %d", len(result.Files), len(baseFiles))
	}
	if countTotalComments(result.Files) == 0 {
		t.Fatalf("expected synthetic comments in fake result")
	}
}

func TestBuildSyntheticCommentsByFileUsesConfiguredLinePicks(t *testing.T) {
	files := []reviewmodel.DiffReviewFileResult{
		{
			FilePath: "src/edge_cases.txt",
			Hunks: []reviewmodel.DiffReviewHunk{
				{
					OldStartLine: 0,
					OldLineCount: 0,
					NewStartLine: 1,
					NewLineCount: 4,
					Content:      "@@ -0,0 +1,4 @@\n+alpha-updated\n+beta-stable\n+gamma-shifted\n+delta-updated\n",
				},
			},
		},
	}

	commentsByFile := buildSyntheticCommentsByFile(files)
	comments := commentsByFile["src/edge_cases.txt"]
	if len(comments) != 2 {
		t.Fatalf("comments len = %d, want 2", len(comments))
	}

	firstFound := false
	lastFound := false
	for _, c := range comments {
		if c.Content == "`alpha-updated` is inconsistent with downstream parser expectations; update the canonical test fixture." {
			firstFound = true
			if c.Line != 1 {
				t.Fatalf("first configured line = %d, want 1", c.Line)
			}
		}
		if c.Content == "`delta-updated` does not match the expected integration test output — realign the test data." {
			lastFound = true
			if c.Line != 4 {
				t.Fatalf("last configured line = %d, want 4", c.Line)
			}
		}
	}

	if !firstFound || !lastFound {
		t.Fatalf("missing required scenarios: first=%v last=%v", firstFound, lastFound)
	}
}

func TestCollectHunkAddedLineNumbersIncludesInteriorAdditions(t *testing.T) {
	hunk := reviewmodel.DiffReviewHunk{
		OldStartLine: 0,
		OldLineCount: 0,
		NewStartLine: 1,
		NewLineCount: 6,
		Content:      "@@ -0,0 +1,6 @@\n+line-1\n line-context-a\n+line-2\n line-context-b\n+line-3\n+line-4\n",
	}

	got := collectHunkAddedLineNumbers(hunk)
	want := []int{1, 3, 5, 6}
	if len(got) != len(want) {
		t.Fatalf("line count = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("line[%d] = %d, want %d (all=%v)", i, got[i], want[i], got)
		}
	}
}

func TestPollReviewFakeCompletes(t *testing.T) {
	baseFiles := []reviewmodel.DiffReviewFileResult{
		{
			FilePath: "src/fake_large_config.toml",
			Hunks: []reviewmodel.DiffReviewHunk{{
				OldStartLine: 1,
				OldLineCount: 0,
				NewStartLine: 1,
				NewLineCount: 2,
				Content:      "@@ -0,0 +1,2 @@\n+enable_telemetry = true\n+env = \"local\"\n",
			}},
		},
	}

	result, err := pollReviewFake("fake-test", 2*time.Millisecond, 1*time.Millisecond, false, nil, baseFiles, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatalf("expected fake poll result")
	}
	if result.Status != "completed" {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	if result.Summary == "" {
		t.Fatalf("expected non-empty summary")
	}
	if countTotalComments(result.Files) == 0 {
		t.Fatalf("expected fake poll result to include comments")
	}
}

func TestPollReviewFakeCancelled(t *testing.T) {
	cancel := make(chan struct{})
	close(cancel)

	_, err := pollReviewFake("fake-test", 10*time.Millisecond, 1*time.Second, false, cancel, nil, nil)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if err != reviewapi.ErrPollCancelled {
		t.Fatalf("error = %v, want %v", err, reviewapi.ErrPollCancelled)
	}
}
