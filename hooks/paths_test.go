package hooks

import "testing"

func TestResolveRepoHooksPathValueUsesCommonDirByDefault(t *testing.T) {
	got, err := resolveRepoHooksPathValue("/repo/worktree", "/repo/.git", "")
	if err != nil {
		t.Fatalf("expected default repo hooks path, got error: %v", err)
	}
	want := "/repo/.git/hooks"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveRepoHooksPathValueUsesRepoRelativeLocalPath(t *testing.T) {
	got, err := resolveRepoHooksPathValue("/repo/worktree", "/repo/.git", ".githooks")
	if err != nil {
		t.Fatalf("expected repo-relative hooks path, got error: %v", err)
	}
	want := "/repo/worktree/.githooks"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestResolveEffectiveHooksPathValuePrefersLocalThenGlobalThenDefault(t *testing.T) {
	t.Run("prefers local", func(t *testing.T) {
		got, err := resolveEffectiveHooksPathValue("/repo/worktree", "/repo/.git", ".local-hooks", "/global/hooks")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/repo/worktree/.local-hooks"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("falls back to global", func(t *testing.T) {
		got, err := resolveEffectiveHooksPathValue("/repo/worktree", "/repo/.git", "", "/global/hooks")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/global/hooks"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})

	t.Run("falls back to common-dir hooks", func(t *testing.T) {
		got, err := resolveEffectiveHooksPathValue("/repo/worktree", "/repo/.git", "", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := "/repo/.git/hooks"
		if got != want {
			t.Fatalf("expected %q, got %q", want, got)
		}
	})
}
