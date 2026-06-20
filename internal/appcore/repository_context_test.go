package appcore

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestResolveReviewRepositoryPathNormalizesRelativePath(t *testing.T) {
	relativePath := filepath.Join("tmp", "repo")
	want, err := filepath.Abs(relativePath)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	got, err := resolveReviewRepositoryPath(func() (string, error) {
		return relativePath, nil
	})
	if err != nil {
		t.Fatalf("resolveReviewRepositoryPath() error = %v", err)
	}

	if got != filepath.Clean(want) {
		t.Fatalf("resolveReviewRepositoryPath() = %q, want %q", got, filepath.Clean(want))
	}
}

func TestResolveReviewRepositoryPathRejectsEmptyPath(t *testing.T) {
	_, err := resolveReviewRepositoryPath(func() (string, error) {
		return "   ", nil
	})
	if err == nil {
		t.Fatal("resolveReviewRepositoryPath() error = nil, want non-nil")
	}
}

func TestResolveReviewRepositoryNameUsesExplicitOverride(t *testing.T) {
	got, err := resolveReviewRepositoryName("custom-name", "/tmp/example", func() (string, error) {
		return "/ignored", nil
	})
	if err != nil {
		t.Fatalf("resolveReviewRepositoryName() error = %v", err)
	}
	if got != "custom-name" {
		t.Fatalf("resolveReviewRepositoryName() = %q, want %q", got, "custom-name")
	}
}

func TestResolveReviewRepositoryNameUsesRepositoryPathBase(t *testing.T) {
	got, err := resolveReviewRepositoryName("", filepath.Join("/tmp", "example-repo"), func() (string, error) {
		return "/ignored", nil
	})
	if err != nil {
		t.Fatalf("resolveReviewRepositoryName() error = %v", err)
	}
	if got != "example-repo" {
		t.Fatalf("resolveReviewRepositoryName() = %q, want %q", got, "example-repo")
	}
}

func TestResolveReviewRepositoryNameFallsBackToNormalizedWorkingDir(t *testing.T) {
	relativeDir := filepath.Join("nested", "worktree")
	wantAbs, err := filepath.Abs(relativeDir)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	got, err := resolveReviewRepositoryName("", "", func() (string, error) {
		return relativeDir, nil
	})
	if err != nil {
		t.Fatalf("resolveReviewRepositoryName() error = %v", err)
	}

	if got != filepath.Base(filepath.Clean(wantAbs)) {
		t.Fatalf("resolveReviewRepositoryName() = %q, want %q", got, filepath.Base(filepath.Clean(wantAbs)))
	}
}

func TestResolveReviewRepositoryNameReturnsWorkingDirError(t *testing.T) {
	wantErr := errors.New("getwd failed")
	_, err := resolveReviewRepositoryName("", "", func() (string, error) {
		return "", wantErr
	})
	if err == nil {
		t.Fatal("resolveReviewRepositoryName() error = nil, want non-nil")
	}
}
