package hooks

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/HexmosTech/git-lrc/configpath"
	"github.com/HexmosTech/git-lrc/storage"
)

// Meta tracks hook path ownership so uninstall can restore prior state.
type Meta struct {
	Path     string `json:"path"`
	PrevPath string `json:"prev_path,omitempty"`
	SetByLRC bool   `json:"set_by_lrc"`
}

func DefaultGlobalHooksPath(defaultDir string) (string, error) {
	home, err := configpath.ResolveHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, defaultDir), nil
}

func CurrentHooksPath() (string, error) {
	cmd := exec.Command("git", "config", "--global", "--get", "core.hooksPath")
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}

	return strings.TrimSpace(string(out)), nil
}

func CurrentLocalHooksPath(repoRoot string) (string, error) {
	cmd := exec.Command("git", "config", "--local", "--get", "core.hooksPath")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", nil
	}

	return strings.TrimSpace(string(out)), nil
}

func resolveConfiguredHooksPath(baseDir, configuredPath string) string {
	if filepath.IsAbs(configuredPath) {
		return configuredPath
	}
	return filepath.Join(baseDir, configuredPath)
}

func resolveRepoHooksPathValue(repoRoot, gitCommonDir, localPath string) (string, error) {
	trimmedLocal := strings.TrimSpace(localPath)
	if trimmedLocal != "" {
		return resolveConfiguredHooksPath(repoRoot, trimmedLocal), nil
	}
	trimmedCommon := strings.TrimSpace(gitCommonDir)
	if trimmedCommon == "" {
		return "", fmt.Errorf("failed to resolve repo hooks path: empty git common dir")
	}
	return filepath.Join(trimmedCommon, "hooks"), nil
}

func resolveEffectiveHooksPathValue(repoRoot, gitCommonDir, localPath, globalPath string) (string, error) {
	if path, err := resolveRepoHooksPathValue(repoRoot, gitCommonDir, localPath); err == nil && strings.TrimSpace(localPath) != "" {
		return path, nil
	}
	trimmedGlobal := strings.TrimSpace(globalPath)
	if trimmedGlobal != "" {
		return trimmedGlobal, nil
	}
	return resolveRepoHooksPathValue(repoRoot, gitCommonDir, localPath)
}

func ResolveRepoHooksPath(repoRoot, gitCommonDir string) (string, error) {
	localPath, _ := CurrentLocalHooksPath(repoRoot)
	return resolveRepoHooksPathValue(repoRoot, gitCommonDir, localPath)
}

func ResolveEffectiveHooksPath(repoRoot, gitCommonDir string) (string, error) {
	localPath, _ := CurrentLocalHooksPath(repoRoot)
	globalPath, _ := CurrentHooksPath()
	return resolveEffectiveHooksPathValue(repoRoot, gitCommonDir, localPath, globalPath)
}

func NormalizeHooksPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("failed to normalize hooks path: empty path")
	}

	if strings.HasPrefix(trimmed, "~") {
		home, err := configpath.ResolveHomeDir()
		if err != nil {
			return "", err
		}

		if trimmed == "~" {
			trimmed = home
		} else if strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "~\\") {
			suffix := strings.TrimLeft(trimmed[1:], "/\\")
			trimmed = filepath.Join(home, suffix)
		}
	}

	absPath, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("failed to resolve hooks path %s: %w", trimmed, err)
	}

	return absPath, nil
}

func SetGlobalHooksPath(path string) error {
	cmd := exec.Command("git", "config", "--global", "core.hooksPath", path)
	return cmd.Run()
}

func UnsetGlobalHooksPath() error {
	cmd := exec.Command("git", "config", "--global", "--unset", "core.hooksPath")
	return cmd.Run()
}

func MetaPath(hooksPath, metaFilename string) string {
	return filepath.Join(hooksPath, metaFilename)
}

func WriteMeta(hooksPath, metaFilename string, meta Meta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	if err := storage.EnsureHooksPathDir(hooksPath); err != nil {
		return err
	}
	if err := storage.WriteFile(MetaPath(hooksPath, metaFilename), data, 0644); err != nil {
		return err
	}

	return nil
}

func ReadMeta(hooksPath, metaFilename string) (*Meta, error) {
	data, err := storage.ReadHookMetaFile(hooksPath, metaFilename)
	if err != nil {
		return nil, err
	}

	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

func RemoveMeta(hooksPath, metaFilename string) error {
	return storage.RemoveHookMetaFile(hooksPath, metaFilename)
}

func PathsEqual(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return a == b
	}
	if absA == absB {
		return true
	}
	realA, errA := filepath.EvalSymlinks(absA)
	realB, errB := filepath.EvalSymlinks(absB)
	if errA != nil || errB != nil {
		return absA == absB
	}
	return realA == realB
}

func CleanEmptyHooksDir(dir string) {
	_ = storage.RemoveDirIfEmpty(dir)
}

func HookHasManagedSection(path, markerBegin string) bool {
	content, err := storage.ReadHookFile(path)
	if err != nil {
		return false
	}

	return strings.Contains(string(content), markerBegin)
}
