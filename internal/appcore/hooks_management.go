package appcore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitops "github.com/HexmosTech/git-lrc/gitops"
	hooksvc "github.com/HexmosTech/git-lrc/hooks"
	"github.com/HexmosTech/git-lrc/internal/reviewapi"
	"github.com/HexmosTech/git-lrc/storage"
	"github.com/urfave/cli/v2"
)

type hooksMeta = hooksvc.Meta

func defaultGlobalHooksPath() (string, error) {
	return hooksvc.DefaultGlobalHooksPath(defaultGlobalHooksDir)
}

func currentHooksPath() (string, error) {
	return hooksvc.CurrentHooksPath()
}

func currentLocalHooksPath(repoRoot string) (string, error) {
	return hooksvc.CurrentLocalHooksPath(repoRoot)
}

func resolveRepoHooksPath(repoRoot, gitCommonDir string) (string, error) {
	return hooksvc.ResolveRepoHooksPath(repoRoot, gitCommonDir)
}

func resolveEffectiveHooksPath(repoRoot, gitCommonDir string) (string, error) {
	return hooksvc.ResolveEffectiveHooksPath(repoRoot, gitCommonDir)
}

func setGlobalHooksPath(path string) error {
	return hooksvc.SetGlobalHooksPath(path)
}

func unsetGlobalHooksPath() error {
	return hooksvc.UnsetGlobalHooksPath()
}

func hooksMetaPath(hooksPath string) string {
	return hooksvc.MetaPath(hooksPath, hooksMetaFilename)
}

func writeHooksMeta(hooksPath string, meta hooksMeta) error {
	return hooksvc.WriteMeta(hooksPath, hooksMetaFilename, meta)
}

func readHooksMeta(hooksPath string) (*hooksMeta, error) {
	return hooksvc.ReadMeta(hooksPath, hooksMetaFilename)
}

func removeHooksMeta(hooksPath string) error {
	return hooksvc.RemoveMeta(hooksPath, hooksMetaFilename)
}

func writeManagedHookScripts(dir string) error {
	return hooksvc.WriteManagedHookScripts(dir, hooksvc.TemplateConfig{
		MarkerBegin:       lrcMarkerBegin,
		MarkerEnd:         lrcMarkerEnd,
		Version:           version,
		CommitMessageFile: commitMessageFile,
		PushRequestFile:   pushRequestFile,
	})
}

func resolveRepoContext() (repoRoot, gitDir, gitCommonDir string, err error) {
	repoRoot, err = reviewapi.ResolveRepoRoot()
	if err != nil {
		return "", "", "", err
	}

	gitDir, err = reviewapi.ResolveGitDir()
	if err != nil {
		return "", "", "", err
	}

	gitCommonDir, err = reviewapi.ResolveGitCommonDir()
	if err != nil {
		return "", "", "", err
	}

	return repoRoot, gitDir, gitCommonDir, nil
}

// runHooksInstall installs dispatchers and managed hook scripts under either global core.hooksPath or the current repo hooks path when --local is used
func runHooksInstall(c *cli.Context) error {
	localInstall := c.Bool("local")
	requestedPath := strings.TrimSpace(c.String("path"))
	var hooksPath string
	var prevGlobalPath string
	setConfig := false

	if localInstall {
		if !isGitRepository() {
			return fmt.Errorf("not in a git repository (no .git directory found)")
		}

		repoRoot, _, gitCommonDir, err := resolveRepoContext()
		if err != nil {
			return err
		}
		hooksPath, err = resolveRepoHooksPath(repoRoot, gitCommonDir)
		if err != nil {
			return err
		}
	} else {
		prevGlobalPath, _ = currentHooksPath()
		currentPath := prevGlobalPath
		defaultPath, err := defaultGlobalHooksPath()
		if err != nil {
			return fmt.Errorf("failed to determine default hooks path: %w", err)
		}

		hooksPath = requestedPath
		if hooksPath == "" {
			if currentPath != "" {
				hooksPath = currentPath
			} else {
				hooksPath = defaultPath
			}
		}

		if currentPath == "" {
			setConfig = true
		} else if requestedPath != "" && requestedPath != currentPath {
			setConfig = true
		}
	}

	absHooksPath, err := hooksvc.NormalizeHooksPath(hooksPath)
	if err != nil {
		return fmt.Errorf("failed to resolve hooks path: %w", err)
	}

	if !localInstall && setConfig {
		if err := setGlobalHooksPath(absHooksPath); err != nil {
			return fmt.Errorf("failed to set core.hooksPath: %w", err)
		}
	}

	if err := storage.EnsureHooksPathDir(absHooksPath); err != nil {
		return fmt.Errorf("failed to create hooks path %s: %w", absHooksPath, err)
	}

	managedDir := filepath.Join(absHooksPath, "lrc")
	backupDir := filepath.Join(absHooksPath, ".lrc_backups")
	if err := storage.EnsureHooksBackupDir(backupDir); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	if err := writeManagedHookScripts(managedDir); err != nil {
		return err
	}

	for _, hookName := range managedHooks {
		hookPath := filepath.Join(absHooksPath, hookName)
		dispatcher := generateDispatcherHook(hookName)
		if err := installHook(hookPath, dispatcher, hookName, backupDir, true); err != nil {
			return fmt.Errorf("failed to install dispatcher for %s: %w", hookName, err)
		}
	}

	if !localInstall {
		if err := writeHooksMeta(absHooksPath, hooksMeta{Path: absHooksPath, PrevPath: prevGlobalPath, SetByLRC: setConfig}); err != nil {
			return fmt.Errorf("failed to write hooks metadata: %w", err)
		}
	}
	_ = cleanOldBackups(backupDir, 5)

	if localInstall {
		fmt.Printf("✅ LiveReview hooks installed in repo path: %s\n", absHooksPath)
	} else {
		fmt.Printf("✅ LiveReview global hooks installed at %s\n", absHooksPath)
	}
	fmt.Println("Dispatchers will chain repo-local hooks when present.")
	fmt.Println("Use 'lrc hooks disable' in a repo to bypass LiveReview hooks there.")

	return nil
}

// runHooksUninstall removes lrc-managed sections from dispatchers and managed scripts (global or local)
func runHooksUninstall(c *cli.Context) error {
	localUninstall := c.Bool("local")
	requestedPath := strings.TrimSpace(c.String("path"))
	var hooksPath string

	if localUninstall {
		if !isGitRepository() {
			return fmt.Errorf("not in a git repository (no .git directory found)")
		}
		repoRoot, _, gitCommonDir, err := resolveRepoContext()
		if err != nil {
			return err
		}
		hooksPath, err = resolveRepoHooksPath(repoRoot, gitCommonDir)
		if err != nil {
			return err
		}
	} else {
		if requestedPath != "" {
			hooksPath = requestedPath
		} else {
			hooksPath, _ = currentHooksPath()
			if hooksPath == "" {
				var err error
				hooksPath, err = defaultGlobalHooksPath()
				if err != nil {
					return fmt.Errorf("failed to determine hooks path: %w", err)
				}
			}
		}
	}

	absHooksPath, err := hooksvc.NormalizeHooksPath(hooksPath)
	if err != nil {
		return fmt.Errorf("failed to resolve hooks path: %w", err)
	}

	currentGlobalPath, _ := currentHooksPath()

	var meta *hooksMeta
	if !localUninstall {
		meta, _ = readHooksMeta(absHooksPath)
	}

	removed := 0
	for _, hookName := range managedHooks {
		hookPath := filepath.Join(absHooksPath, hookName)
		if err := uninstallHook(hookPath, hookName); err != nil {
			fmt.Printf("⚠️  Warning: failed to uninstall %s: %v\n", hookName, err)
		} else {
			removed++
		}
	}

	_ = storage.RemoveManagedHooksDir(absHooksPath)
	_ = storage.RemoveHooksBackupDir(absHooksPath)
	if !localUninstall {
		_ = removeHooksMeta(absHooksPath)
	}

	if !localUninstall {
		restoredHooksPath := false

		if meta != nil && meta.SetByLRC {
			if meta.PrevPath == "" {
				if err := unsetGlobalHooksPath(); err != nil {
					fmt.Printf("⚠️  Warning: failed to unset core.hooksPath: %v\n", err)
				} else {
					fmt.Println("✅ Unset core.hooksPath (was set by lrc)")
					restoredHooksPath = true
				}
			} else {
				if err := setGlobalHooksPath(meta.PrevPath); err != nil {
					fmt.Printf("⚠️  Warning: failed to restore core.hooksPath to %s: %v\n", meta.PrevPath, err)
				} else {
					fmt.Printf("✅ Restored core.hooksPath to %s\n", meta.PrevPath)
					restoredHooksPath = true
				}
			}
		} else if meta == nil && currentGlobalPath != "" && pathsEqual(currentGlobalPath, absHooksPath) {
			if err := unsetGlobalHooksPath(); err != nil {
				fmt.Printf("⚠️  Warning: failed to unset core.hooksPath: %v\n", err)
			} else {
				fmt.Println("✅ Unset core.hooksPath (was pointing to uninstalled hooks dir)")
				restoredHooksPath = true
			}
		}

		postPath, _ := currentHooksPath()
		if postPath != "" && pathsEqual(postPath, absHooksPath) && !restoredHooksPath {
			fmt.Printf("⚠️  Warning: core.hooksPath is still set to %s\n", postPath)
			fmt.Println("   This may prevent repo-local hooks from working.")
			fmt.Println("   Run: git config --global --unset core.hooksPath")
		}
	}

	if !localUninstall {
		cleanEmptyHooksDir(absHooksPath)
	}

	if removed > 0 {
		fmt.Printf("✅ Removed LiveReview sections from %d hook(s) at %s\n", removed, absHooksPath)
	} else {
		fmt.Printf("ℹ️  No LiveReview sections found in %s\n", absHooksPath)
	}

	return nil
}

// pathsEqual compares two filesystem paths robustly, resolving symlinks
func pathsEqual(a, b string) bool {
	return hooksvc.PathsEqual(a, b)
}

// cleanEmptyHooksDir removes the hooks directory if it's empty or contains only lrc artifacts
func cleanEmptyHooksDir(dir string) {
	hooksvc.CleanEmptyHooksDir(dir)
}

func runHooksDisable(c *cli.Context) error {
	gitDir, err := reviewapi.ResolveGitDir()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	lrcDir := filepath.Join(gitDir, "lrc")
	if err := storage.EnsureRepoLRCStateDir(lrcDir); err != nil {
		return fmt.Errorf("failed to create lrc directory: %w", err)
	}

	marker := filepath.Join(lrcDir, "disabled")
	if err := storage.WriteFile(marker, []byte("disabled\n"), 0644); err != nil {
		return fmt.Errorf("failed to write disable marker: %w", err)
	}

	fmt.Println("🔕 LiveReview hooks disabled for this repository")
	return nil
}

func runHooksEnable(c *cli.Context) error {
	gitDir, err := reviewapi.ResolveGitDir()
	if err != nil {
		return fmt.Errorf("not in a git repository: %w", err)
	}

	marker := filepath.Join(gitDir, "lrc", "disabled")
	if err := storage.RemoveRepoHooksDisabledMarker(marker); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove disable marker: %w", err)
	}

	fmt.Println("🔔 LiveReview hooks enabled for this repository")
	return nil
}

func hookHasManagedSection(path string) bool {
	return hooksvc.HookHasManagedSection(path, lrcMarkerBegin)
}

func runHooksStatus(c *cli.Context) error {
	globalHooksPath, _ := currentHooksPath()
	defaultPath, _ := defaultGlobalHooksPath()

	repoRoot, gitDir, gitCommonDir, gitErr := resolveRepoContext()
	repoDisabled := false
	if gitErr == nil {
		repoDisabled = fileExists(filepath.Join(gitDir, "lrc", "disabled"))
	}

	hooksPath := globalHooksPath
	if gitErr == nil {
		var err error
		hooksPath, err = resolveEffectiveHooksPath(repoRoot, gitCommonDir)
		if err != nil {
			return err
		}
	} else if hooksPath == "" {
		hooksPath = defaultPath
	}

	absHooksPath, err := hooksvc.NormalizeHooksPath(hooksPath)
	if err != nil {
		return fmt.Errorf("failed to resolve hooks path: %w", err)
	}

	fmt.Printf("hooksPath: %s\n", absHooksPath)
	if localCfg, _ := currentLocalHooksPath(repoRoot); gitErr == nil && localCfg != "" {
		fmt.Printf("repo core.hooksPath: %s\n", localCfg)
	}
	if globalHooksPath != "" {
		fmt.Printf("global core.hooksPath: %s\n", globalHooksPath)
	} else {
		fmt.Println("global core.hooksPath: not set")
	}

	if gitErr == nil {
		disabledPath := filepath.Join(gitDir, "lrc", "disabled")
		fmt.Printf("repo: %s\n", repoRoot)
		if repoDisabled {
			fmt.Printf("status: disabled via %s\n", disabledPath)
		} else {
			fmt.Println("status: enabled")
		}
	} else {
		fmt.Println("repo: not detected")
	}

	for _, hookName := range managedHooks {
		hookPath := filepath.Join(absHooksPath, hookName)
		fmt.Printf("%s: ", hookName)
		if hookHasManagedSection(hookPath) {
			fmt.Println("LiveReview dispatcher present")
		} else if fileExists(hookPath) {
			fmt.Println("custom hook (no LiveReview block)")
		} else {
			fmt.Println("missing")
		}
	}

	return nil
}

// isGitRepository checks if current directory is in a git repository
func isGitRepository() bool {
	return gitops.IsGitRepositoryCurrentDir()
}

// installHook installs or updates a hook with lrc managed section
func installHook(hookPath, lrcSection, hookName, backupDir string, force bool) error {
	return hooksvc.InstallHook(hookPath, lrcSection, hookName, backupDir, lrcMarkerBegin, lrcMarkerEnd, force)
}

// uninstallHook removes lrc-managed section from a hook file
func uninstallHook(hookPath, hookName string) error {
	return hooksvc.UninstallHook(hookPath, hookName, lrcMarkerBegin, lrcMarkerEnd)
}

// installEditorWrapper sets core.editor to an lrc-managed wrapper that injects
// the precommit-provided message when available and falls back to the user's editor.
func installEditorWrapper(gitDir string) error {
	repoRoot, err := reviewapi.ResolveRepoRoot()
	if err != nil {
		return fmt.Errorf("failed to resolve repository root: %w", err)
	}
	scriptPath := filepath.Join(gitDir, editorWrapperScript)
	backupPath := filepath.Join(gitDir, editorBackupFile)

	currentEditor, _ := readGitConfig(repoRoot, "core.editor")
	if currentEditor != "" {
		_ = storage.WriteFile(backupPath, []byte(currentEditor), 0600)
	}

	script := fmt.Sprintf(`#!/bin/sh
set -e

OVERRIDE_FILE="%s"

if [ -f "$OVERRIDE_FILE" ] && [ -s "$OVERRIDE_FILE" ]; then
    cat "$OVERRIDE_FILE" > "$1"
    exit 0
fi

if [ -n "$LRC_FALLBACK_EDITOR" ]; then
    exec $LRC_FALLBACK_EDITOR "$@"
fi

if [ -n "$VISUAL" ]; then
    exec "$VISUAL" "$@"
fi

if [ -n "$EDITOR" ]; then
    exec "$EDITOR" "$@"
fi

exec vi "$@"
`, filepath.Join(gitDir, commitMessageFile))

	if err := storage.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to write editor wrapper: %w", err)
	}

	if err := setGitConfig(repoRoot, "core.editor", scriptPath); err != nil {
		return fmt.Errorf("failed to set core.editor: %w", err)
	}

	return nil
}

// uninstallEditorWrapper restores the previous editor (if backed up) and removes wrapper files.
func uninstallEditorWrapper(gitDir string) error {
	repoRoot, err := reviewapi.ResolveRepoRoot()
	if err != nil {
		return fmt.Errorf("failed to resolve repository root: %w", err)
	}
	scriptPath := filepath.Join(gitDir, editorWrapperScript)
	backupPath := filepath.Join(gitDir, editorBackupFile)

	if data, err := storage.ReadEditorBackupFile(backupPath); err == nil {
		value := strings.TrimSpace(string(data))
		if value != "" {
			_ = setGitConfig(repoRoot, "core.editor", value)
		}
	} else {
		_ = unsetGitConfig(repoRoot, "core.editor")
	}

	_ = storage.RemoveEditorWrapperScript(scriptPath)
	_ = storage.RemoveEditorBackupStateFile(backupPath)

	return nil
}

// readGitConfig reads a single git config key from the repository root.
func readGitConfig(repoRoot, key string) (string, error) {
	return gitops.ReadGitConfig(repoRoot, key)
}

// setGitConfig sets a git config key in the given repository.
func setGitConfig(repoRoot, key, value string) error {
	return gitops.SetGitConfig(repoRoot, key, value)
}

// unsetGitConfig removes a git config key in the given repository.
func unsetGitConfig(repoRoot, key string) error {
	return gitops.UnsetGitConfig(repoRoot, key)
}

// replaceLrcSection replaces the lrc-managed section in hook content
func replaceLrcSection(content, newSection string) string {
	return hooksvc.ReplaceManagedSection(content, newSection, lrcMarkerBegin, lrcMarkerEnd)
}

// removeLrcSection removes the lrc-managed section from hook content
func removeLrcSection(content string) string {
	return hooksvc.RemoveManagedSection(content, lrcMarkerBegin, lrcMarkerEnd)
}

// generatePreCommitHook generates the pre-commit hook script
func generatePreCommitHook() string {
	return hooksvc.GeneratePreCommitHook(hooksvc.TemplateConfig{
		MarkerBegin: lrcMarkerBegin,
		MarkerEnd:   lrcMarkerEnd,
		Version:     version,
	})
}

// generatePrepareCommitMsgHook generates the prepare-commit-msg hook script
func generatePrepareCommitMsgHook() string {
	return hooksvc.GeneratePrepareCommitMsgHook(hooksvc.TemplateConfig{
		MarkerBegin: lrcMarkerBegin,
		MarkerEnd:   lrcMarkerEnd,
		Version:     version,
	})
}

// generateCommitMsgHook generates the commit-msg hook script
func generateCommitMsgHook() string {
	return hooksvc.GenerateCommitMsgHook(hooksvc.TemplateConfig{
		MarkerBegin:       lrcMarkerBegin,
		MarkerEnd:         lrcMarkerEnd,
		Version:           version,
		CommitMessageFile: commitMessageFile,
	})
}

// generatePostCommitHook runs a safe pull (ff-only) and push when requested.
func generatePostCommitHook() string {
	return hooksvc.GeneratePostCommitHook(hooksvc.TemplateConfig{
		MarkerBegin:     lrcMarkerBegin,
		MarkerEnd:       lrcMarkerEnd,
		Version:         version,
		PushRequestFile: pushRequestFile,
	})
}

func generateDispatcherHook(hookName string) string {
	return hooksvc.GenerateDispatcherHook(hookName, hooksvc.TemplateConfig{
		MarkerBegin: lrcMarkerBegin,
		MarkerEnd:   lrcMarkerEnd,
		Version:     version,
	})
}

// cleanOldBackups removes old backup files, keeping only the last N
func cleanOldBackups(backupDir string, keepLast int) error {
	return hooksvc.CleanOldBackups(backupDir, keepLast)
}
