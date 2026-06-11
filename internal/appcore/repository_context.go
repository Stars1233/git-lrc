package appcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/HexmosTech/git-lrc/internal/reviewapi"
)

// resolveReviewRepositoryPath returns the absolute worktree root when available.
// Bare repositories or other setups without a worktree root should surface an
// error so callers can omit the local-only UI affordance and fall back cleanly.
func resolveReviewRepositoryPath(resolveRepoRoot func() (string, error)) (string, error) {
	repoRootPath, err := resolveRepoRoot()
	if err != nil {
		return "", err
	}

	repoRootPath = strings.TrimSpace(repoRootPath)
	if repoRootPath == "" {
		return "", fmt.Errorf("repository root path is empty")
	}

	if !filepath.IsAbs(repoRootPath) {
		repoRootPath, err = filepath.Abs(repoRootPath)
		if err != nil {
			return "", fmt.Errorf("failed to normalize repository root path: %w", err)
		}
	}

	return filepath.Clean(repoRootPath), nil
}

func resolveRuntimeRepositoryPath() (string, error) {
	return resolveReviewRepositoryPath(reviewapi.ResolveRepoRoot)
}

func resolveReviewRepositoryName(explicitRepoName, repositoryPath string, getwd func() (string, error)) (string, error) {
	repoName := strings.TrimSpace(explicitRepoName)
	if repoName != "" {
		return repoName, nil
	}

	if repositoryPath != "" {
		return filepath.Base(repositoryPath), nil
	}

	currentDir, err := getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	if !filepath.IsAbs(currentDir) {
		currentDir, err = filepath.Abs(currentDir)
		if err != nil {
			return "", fmt.Errorf("failed to normalize current directory: %w", err)
		}
	}

	return filepath.Base(filepath.Clean(currentDir)), nil
}

func resolveRuntimeRepositoryName(explicitRepoName, repositoryPath string) (string, error) {
	return resolveReviewRepositoryName(explicitRepoName, repositoryPath, os.Getwd)
}
