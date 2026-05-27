package appcore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/HexmosTech/git-lrc/configpath"
	"github.com/HexmosTech/git-lrc/storage"
)

const (
	claudeManagedMatcher       = "Bash"
	claudeManagedCondition     = "Bash(git commit *)"
	claudeManagedStatusMessage = "Running blocking LiveReview gate before git commit"
	claudeManagedValidatorName = "blocking-review-git-commit.sh"
	claudeManagedWrapperName   = "run-blocking-review-git-commit.sh"
	claudeManagedHookTimeout   = 1260
)

type claudeGlobalInstallState struct {
	SettingsPath    string
	HooksDir        string
	ValidatorPath   string
	WrapperPath     string
	SkillPath       string
	SettingsManaged bool
	ValidatorExists bool
	WrapperExists   bool
	SkillExists     bool
}

func claudeGlobalInstallStatus() (claudeGlobalInstallState, error) {
	settingsPath, err := configpath.ResolveClaudeSettingsPath()
	if err != nil {
		return claudeGlobalInstallState{}, err
	}
	hooksDir, err := configpath.ResolveClaudeManagedHooksDir()
	if err != nil {
		return claudeGlobalInstallState{}, err
	}
	skillPath, err := configpath.ResolveClaudeLRCSkillPath()
	if err != nil {
		return claudeGlobalInstallState{}, err
	}
	validatorPath := filepath.Join(hooksDir, claudeManagedValidatorName)
	wrapperPath := filepath.Join(hooksDir, claudeManagedWrapperName)

	settingsBytes, err := readClaudeSettingsIfPresent(settingsPath)
	if err != nil {
		return claudeGlobalInstallState{}, err
	}

	return claudeGlobalInstallState{
		SettingsPath:    settingsPath,
		HooksDir:        hooksDir,
		ValidatorPath:   validatorPath,
		WrapperPath:     wrapperPath,
		SkillPath:       skillPath,
		SettingsManaged: hasManagedClaudeHook(settingsBytes),
		ValidatorExists: fileExists(validatorPath),
		WrapperExists:   fileExists(wrapperPath),
		SkillExists:     fileExists(skillPath),
	}, nil
}

func installClaudeGlobalHooks() (claudeGlobalInstallState, error) {
	state, err := claudeGlobalInstallStatus()
	if err != nil {
		return claudeGlobalInstallState{}, err
	}

	if err := storage.EnsureClaudeManagedHooksDir(state.HooksDir); err != nil {
		return claudeGlobalInstallState{}, err
	}
	if err := storage.WriteClaudeHookScript(state.ValidatorPath, []byte(generateGlobalClaudeValidatorScript())); err != nil {
		return claudeGlobalInstallState{}, err
	}
	if err := storage.WriteClaudeHookScript(state.WrapperPath, []byte(generateGlobalClaudeWrapperScript())); err != nil {
		return claudeGlobalInstallState{}, err
	}
	if err := storage.WriteClaudeSkillFile(state.SkillPath, []byte(generateClaudeLRCSkill())); err != nil {
		return claudeGlobalInstallState{}, err
	}

	settingsBytes, err := readClaudeSettingsIfPresent(state.SettingsPath)
	if err != nil {
		return claudeGlobalInstallState{}, err
	}
	next, err := ensureManagedClaudeHook(settingsBytes, state.ValidatorPath)
	if err != nil {
		return claudeGlobalInstallState{}, err
	}
	if err := storage.WriteClaudeSettingsFile(state.SettingsPath, next); err != nil {
		return claudeGlobalInstallState{}, err
	}

	return claudeGlobalInstallStatus()
}

func uninstallClaudeGlobalHooks() (claudeGlobalInstallState, error) {
	state, err := claudeGlobalInstallStatus()
	if err != nil {
		return claudeGlobalInstallState{}, err
	}

	settingsBytes, err := readClaudeSettingsIfPresent(state.SettingsPath)
	if err != nil {
		return claudeGlobalInstallState{}, err
	}
	next, changed, err := removeManagedClaudeHook(settingsBytes)
	if err != nil {
		return claudeGlobalInstallState{}, err
	}
	if changed {
		if err := storage.WriteClaudeSettingsFile(state.SettingsPath, next); err != nil {
			return claudeGlobalInstallState{}, err
		}
	}

	if err := storage.RemoveClaudeHookScript(state.ValidatorPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return claudeGlobalInstallState{}, err
	}
	if err := storage.RemoveClaudeHookScript(state.WrapperPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return claudeGlobalInstallState{}, err
	}
	if err := storage.RemoveClaudeSkillFile(state.SkillPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return claudeGlobalInstallState{}, err
	}
	_ = storage.RemoveDirIfEmpty(state.HooksDir)
	_ = storage.RemoveDirIfEmpty(filepath.Dir(state.HooksDir))
	_ = storage.RemoveDirIfEmpty(filepath.Dir(state.SkillPath))
	_ = storage.RemoveDirIfEmpty(filepath.Dir(filepath.Dir(state.SkillPath)))

	return claudeGlobalInstallStatus()
}

func detectLegacyRepoClaudeIntegration(repoRoot string) []string {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	candidates := []string{
		filepath.Join(repoRoot, ".claude", "settings.local.json"),
		filepath.Join(repoRoot, ".claude", "hooks", claudeManagedValidatorName),
		filepath.Join(repoRoot, ".claude", "hooks", claudeManagedWrapperName),
	}
	legacy := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if fileExists(candidate) {
			legacy = append(legacy, candidate)
		}
	}
	return legacy
}

func removeLegacyRepoClaudeIntegration(repoRoot string) ([]string, error) {
	legacy := detectLegacyRepoClaudeIntegration(repoRoot)
	if len(legacy) == 0 {
		return nil, nil
	}

	removed := make([]string, 0, len(legacy))
	for _, path := range legacy {
		existed, err := storage.RemoveFileIfExists(path, false)
		if err != nil {
			return removed, err
		}
		if existed {
			removed = append(removed, path)
		}
	}

	claudeHooksDir := filepath.Join(repoRoot, ".claude", "hooks")
	_ = storage.RemoveDirIfEmpty(claudeHooksDir)
	_ = storage.RemoveDirIfEmpty(filepath.Join(repoRoot, ".claude"))

	return removed, nil
}

func readClaudeSettingsIfPresent(path string) ([]byte, error) {
	settingsBytes, err := storage.ReadClaudeSettingsFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return settingsBytes, nil
}

func hasManagedClaudeHook(settingsBytes []byte) bool {
	root, err := decodeClaudeSettings(settingsBytes)
	if err != nil {
		return false
	}

	preToolUse, ok := preToolUseMatchers(root)
	if !ok {
		return false
	}
	for _, matcher := range preToolUse {
		matcherMap, ok := matcher.(map[string]any)
		if !ok || strings.TrimSpace(stringValue(matcherMap["matcher"])) != claudeManagedMatcher {
			continue
		}
		hooks, ok := objectArray(matcherMap["hooks"])
		if !ok {
			continue
		}
		for _, hookEntry := range hooks {
			if isManagedClaudeHookEntry(hookEntry) {
				return true
			}
		}
	}
	return false
}

func ensureManagedClaudeHook(settingsBytes []byte, validatorPath string) ([]byte, error) {
	root, err := decodeClaudeSettings(settingsBytes)
	if err != nil {
		return nil, err
	}

	managedHook := map[string]any{
		"type":          "command",
		"if":            claudeManagedCondition,
		"command":       validatorPath,
		"args":          []any{},
		"timeout":       claudeManagedHookTimeout,
		"statusMessage": claudeManagedStatusMessage,
	}

	hooksMap := ensureObjectField(root, "hooks")
	preToolUse, _ := objectArray(hooksMap["PreToolUse"])
	matcherIndex := -1
	for i, matcher := range preToolUse {
		matcherMap, ok := matcher.(map[string]any)
		if ok && strings.TrimSpace(stringValue(matcherMap["matcher"])) == claudeManagedMatcher {
			matcherIndex = i
			break
		}
	}

	if matcherIndex == -1 {
		preToolUse = append(preToolUse, map[string]any{
			"matcher": claudeManagedMatcher,
			"hooks":   []any{managedHook},
		})
		hooksMap["PreToolUse"] = preToolUse
		root["hooks"] = hooksMap
		return marshalClaudeSettings(root)
	}

	matcherMap, _ := preToolUse[matcherIndex].(map[string]any)
	hooks, _ := objectArray(matcherMap["hooks"])
	hookIndex := -1
	for i, hookEntry := range hooks {
		if isManagedClaudeHookEntry(hookEntry) {
			hookIndex = i
			break
		}
	}
	if hookIndex >= 0 {
		hooks[hookIndex] = managedHook
	} else {
		hooks = append(hooks, managedHook)
	}
	matcherMap["hooks"] = hooks
	preToolUse[matcherIndex] = matcherMap
	hooksMap["PreToolUse"] = preToolUse
	root["hooks"] = hooksMap
	return marshalClaudeSettings(root)
}

func removeManagedClaudeHook(settingsBytes []byte) ([]byte, bool, error) {
	root, err := decodeClaudeSettings(settingsBytes)
	if err != nil {
		return nil, false, err
	}

	hooksValue, ok := root["hooks"]
	if !ok {
		next, err := marshalClaudeSettings(root)
		return next, false, err
	}
	hooksMap, ok := hooksValue.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("invalid Claude settings: hooks must be an object")
	}
	preToolUse, ok := objectArray(hooksMap["PreToolUse"])
	if !ok {
		next, err := marshalClaudeSettings(root)
		return next, false, err
	}

	changed := false
	nextMatchers := make([]any, 0, len(preToolUse))
	for _, matcher := range preToolUse {
		matcherMap, ok := matcher.(map[string]any)
		if !ok || strings.TrimSpace(stringValue(matcherMap["matcher"])) != claudeManagedMatcher {
			nextMatchers = append(nextMatchers, matcher)
			continue
		}

		hooks, _ := objectArray(matcherMap["hooks"])
		nextHooks := make([]any, 0, len(hooks))
		for _, hookEntry := range hooks {
			if isManagedClaudeHookEntry(hookEntry) {
				changed = true
				continue
			}
			nextHooks = append(nextHooks, hookEntry)
		}
		if len(nextHooks) == 0 {
			changed = true
			continue
		}
		matcherMap["hooks"] = nextHooks
		nextMatchers = append(nextMatchers, matcherMap)
	}

	if len(nextMatchers) == 0 {
		delete(hooksMap, "PreToolUse")
	} else {
		hooksMap["PreToolUse"] = nextMatchers
	}
	if len(hooksMap) == 0 {
		delete(root, "hooks")
	} else {
		root["hooks"] = hooksMap
	}

	next, err := marshalClaudeSettings(root)
	if err != nil {
		return nil, false, err
	}
	return next, changed, nil
}

func decodeClaudeSettings(settingsBytes []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(settingsBytes))) == 0 {
		return map[string]any{}, nil
	}
	var root map[string]any
	if err := json.Unmarshal(settingsBytes, &root); err != nil {
		return nil, fmt.Errorf("failed to parse Claude settings JSON: %w", err)
	}
	if root == nil {
		root = map[string]any{}
	}
	return root, nil
}

func marshalClaudeSettings(root map[string]any) ([]byte, error) {
	if root == nil {
		root = map[string]any{}
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to encode Claude settings JSON: %w", err)
	}
	return append(data, '\n'), nil
}

func preToolUseMatchers(root map[string]any) ([]any, bool) {
	hooksValue, ok := root["hooks"]
	if !ok {
		return nil, false
	}
	hooksMap, ok := hooksValue.(map[string]any)
	if !ok {
		return nil, false
	}
	return objectArray(hooksMap["PreToolUse"])
}

func ensureObjectField(root map[string]any, key string) map[string]any {
	if existing, ok := root[key].(map[string]any); ok {
		return existing
	}
	created := map[string]any{}
	root[key] = created
	return created
}

func objectArray(value any) ([]any, bool) {
	if value == nil {
		return []any{}, false
	}
	items, ok := value.([]any)
	return items, ok
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func isManagedClaudeHookEntry(value any) bool {
	hookMap, ok := value.(map[string]any)
	if !ok {
		return false
	}
	if strings.TrimSpace(stringValue(hookMap["type"])) != "command" {
		return false
	}
	if strings.TrimSpace(stringValue(hookMap["if"])) != claudeManagedCondition {
		return false
	}
	return isManagedClaudeCommandPath(stringValue(hookMap["command"]))
}

func isManagedClaudeCommandPath(path string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(path), `\`, "/")
	return strings.HasSuffix(normalized, "/.lrc/claude/hooks/"+claudeManagedValidatorName)
}

func generateGlobalClaudeValidatorScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
helper_path="$script_dir/run-blocking-review-git-commit.sh"
payload_file=$(mktemp)

cleanup() {
  rm -f "$payload_file"
}

trap cleanup EXIT

cat >"$payload_file"

emit_deny() {
  local reason="$1"
  printf '%s\n' "{\"hookSpecificOutput\":{\"hookEventName\":\"PreToolUse\",\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$reason\"}}"
}

validate_supported_commit_command() {
  python3 - "$payload_file" <<'PY'
import json
import shlex
import sys

try:
  with open(sys.argv[1], encoding="utf-8") as handle:
    payload = json.load(handle)
except Exception:
  print("Claude hook could not parse the git commit command payload", end="")
  raise SystemExit(1)

command = (payload.get("tool_input", {}).get("command") or "").strip()
if not command:
  print("Claude hook could not find the git commit command payload", end="")
  raise SystemExit(1)

lexer = shlex.shlex(command, posix=True, punctuation_chars=';&|()')
lexer.whitespace_split = True
try:
  tokens = list(lexer)
except ValueError:
  print("Claude LiveReview gate could not parse the shell command safely", end="")
  raise SystemExit(1)

operators = {"&&", "||", ";", "|", "&", "(", ")"}
if any(token in operators for token in tokens):
  print("Claude LiveReview gate currently supports a single git commit command only. Run staging or setup commands separately, then retry git commit.", end="")
  raise SystemExit(1)

if len(tokens) < 2 or tokens[0] != "git":
  print("Claude LiveReview gate currently supports a single git commit command only. Retry with git commit as a separate command.", end="")
  raise SystemExit(1)

if tokens[1] != "commit":
  print("Claude LiveReview gate currently supports a single git commit command only. Retry with git commit as a separate command.", end="")
  raise SystemExit(1)
PY
}

emit_allow_with_wrapper() {
  local reason="$1"
  if ! REVIEW_REASON="$reason" CLAUDE_HELPER_PATH="$helper_path" CLAUDE_PROJECT_DIR_VALUE="$CLAUDE_PROJECT_DIR" python3 - "$payload_file" <<'PY'
import json
import os
import shlex
import sys

try:
    with open(sys.argv[1], encoding="utf-8") as handle:
        payload = json.load(handle)
except Exception:
    raise SystemExit(1)

tool_input = dict(payload.get("tool_input", {}))
command = (tool_input.get("command") or "").strip()

if not command:
    raise SystemExit(1)

project_dir = os.environ["CLAUDE_PROJECT_DIR_VALUE"]
helper_path = os.environ["CLAUDE_HELPER_PATH"]

tool_input["command"] = " ".join([
    f"LRC_CLAUDE_PROJECT_DIR={shlex.quote(project_dir)}",
    f"LRC_ORIGINAL_GIT_COMMIT={shlex.quote(command)}",
    shlex.quote(helper_path),
])

print(json.dumps({
    "hookSpecificOutput": {
        "hookEventName": "PreToolUse",
        "permissionDecision": "allow",
        "permissionDecisionReason": os.environ["REVIEW_REASON"],
        "updatedInput": tool_input,
    }
}))
PY
  then
    emit_deny "Blocking review completed, but the Claude hook could not rewrite the git commit command safely"
    return 0
  fi
}

if ! command -v lrc >/dev/null 2>&1; then
  emit_deny "lrc is not available on PATH, so the blocking review gate cannot run"
  exit 0
fi

if ! command -v python3 >/dev/null 2>&1; then
  emit_deny "python3 is required for the local Claude blocking-review hook"
  exit 0
fi

if [ ! -x "$helper_path" ]; then
  emit_deny "Claude blocking-review helper script is missing or not executable"
  exit 0
fi

if ! validation_reason=$(validate_supported_commit_command); then
  emit_deny "$validation_reason"
  exit 0
fi

emit_allow_with_wrapper "Blocking review wrapper installed; git commit will run after review resolves"
`
}

func generateGlobalClaudeWrapperScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

project_dir="${LRC_CLAUDE_PROJECT_DIR:-$PWD}"
original_command="${LRC_ORIGINAL_GIT_COMMIT:-}"
blocking_timeout="${LRC_BLOCKING_REVIEW_TIMEOUT:-20m}"

if [[ -z "$original_command" ]]; then
  echo "LiveReview: missing original git commit command for Claude wrapper" >&2
  exit 1
fi

if ! command -v lrc >/dev/null 2>&1; then
  echo "LiveReview: lrc is not available on PATH, so the blocking review gate cannot run" >&2
  exit 1
fi

lrc_bin="$(command -v lrc)"

lrc_review_mode="$($lrc_bin version 2>/dev/null | awk -F': ' '/Review mode/ {print $2; exit}')"

if [[ "$lrc_review_mode" == "fake" ]]; then
  echo "LiveReview: refusing to use fake-review lrc binary at $lrc_bin" >&2
  echo "LiveReview: rebuild the real CLI with 'make build-local && lrc hooks install' before retrying git commit" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "LiveReview: python3 is required for the local Claude blocking-review helper" >&2
  exit 1
fi

if ! initial_message=$(LRC_ORIGINAL_GIT_COMMIT="$original_command" python3 - <<'PY'
import os
import re
import shlex

command = os.environ.get("LRC_ORIGINAL_GIT_COMMIT", "")

def extract_heredoc_command_substitution(value):
	if not value.startswith("$(") or not value.endswith(")"):
		return None

	body = value[2:-1]
	lines = body.splitlines()
	if not lines:
		return None

	first_line = lines[0].strip()
	match = re.fullmatch(r"cat\s+<<-?\s*(?:'([^']+)'|\"([^\"]+)\"|([A-Za-z_][A-Za-z0-9_]*))", first_line)
	if not match:
		return None

	marker = next(group for group in match.groups() if group is not None)
	if len(lines) < 2 or lines[-1].strip() != marker:
		return None

	return "\n".join(lines[1:-1])

try:
    tokens = shlex.split(command, posix=True)
except ValueError:
    print("", end="")
    raise SystemExit(0)

message = ""
i = 0
while i < len(tokens):
    token = tokens[i]
    if token in ("-m", "--message") and i + 1 < len(tokens):
        message = tokens[i + 1]
        break
    if token.startswith("--message="):
        message = token.split("=", 1)[1]
        break
    if token.startswith("-m") and token != "-m" and len(token) > 2:
        message = token[2:]
        break
    i += 1

resolved_message = extract_heredoc_command_substitution(message)
if resolved_message is not None:
	message = resolved_message

print(message, end="")
PY
); then
  echo "LiveReview: failed to parse the original git commit command" >&2
  exit 1
fi

cd "$project_dir"

git_dir="$(git rev-parse --git-dir 2>/dev/null || echo .git)"
lrc_dir="$git_dir/lrc"
disabled_file="$lrc_dir/disabled"
disabled_claude_file="$lrc_dir/disabled-claude"

if [[ -f "$disabled_file" || -f "$disabled_claude_file" ]]; then
  echo "LiveReview: Claude review hook disabled for this repository; proceeding with git commit." >&2
  exec bash -c "$original_command"
fi

echo "LiveReview: checking whether the current staged tree already has a valid review." >&2
echo "LiveReview: if not, a blocking browser review will open before git commit can continue." >&2

review_log=$(mktemp)
cleanup() {
  rm -f "$review_log"
}
trap cleanup EXIT

set +e
if [[ -n "$initial_message" ]]; then
  LRC_INITIAL_MESSAGE="$initial_message" lrc review --staged --blocking-review --blocking-review-timeout "$blocking_timeout" 2>&1 | tee "$review_log"
  review_status=${PIPESTATUS[0]}
else
  lrc review --staged --blocking-review --blocking-review-timeout "$blocking_timeout" 2>&1 | tee "$review_log"
  review_status=${PIPESTATUS[0]}
fi
set -e

case "$review_status" in
  0|2)
    exec env LRC_CLAUDE_REVIEW_HANDLED=1 bash -c "$original_command"
    ;;
  1)
    if grep -q "attestation already present for current tree" "$review_log"; then
      echo "LiveReview: current tree is already reviewed; proceeding with git commit." >&2
      exec env LRC_CLAUDE_REVIEW_HANDLED=1 bash -c "$original_command"
    fi
    if grep -q "Commit aborted by user" "$review_log"; then
      echo "LiveReview: commit intentionally aborted in the browser; git commit was not run." >&2
      exit 0
    fi
    echo "LiveReview: blocking review exited with code 1 before git commit could continue" >&2
    exit 1
    ;;
  *)
    echo "LiveReview: blocking review failed before git commit could continue" >&2
    exit 1
    ;;
esac
`
}

func generateClaudeLRCSkill() string {
	skill := `---
name: lrc
description: >
  Manage LiveReview code review for staged changes in this repo. Run /lrc review to open a
  browser-based AI review of staged changes before committing. Use /lrc skip to bypass review and
  write an attestation, or /lrc vouch to manually approve without AI. Check and toggle hook state
  with /lrc hooks status, hooks disable, hooks enable, hooks install, or hooks uninstall. Invoke
	when the user explicitly wants to review code, explicitly skip a review, vouch for changes,
	check if LiveReview is active, turn off hooks, or reinstall the Claude git-commit gate.
argument-hint: "review | skip | vouch | hooks [status|enable|disable|install|uninstall] [--surface claude|git]"
---

# lrc

Run commands from the current repository root. Map $ARGUMENTS to the matching command below.
Translate plain natural-language requests to the nearest command before running.

## Review commands

| Intent | Command |
|--------|---------|
| Review staged changes | ` + "`lrc review --staged`" + ` |
| Review staged + block until browser decision | ` + "`lrc review --staged --blocking-review`" + ` |
| Skip review (write attestation, no AI) | ` + "`lrc review --staged --skip`" + ` |
| Vouch for changes manually | ` + "`lrc review --staged --vouch`" + ` |
| Review a specific prior commit | ` + "`lrc review --commit HEAD`" + ` |

## Hooks commands

| Intent | Command |
|--------|---------|
| Check hook status | ` + "`lrc hooks status`" + ` |
| Check Claude hook status only | ` + "`lrc hooks status --surface claude`" + ` |
| Disable all hooks in this repo | ` + "`lrc hooks disable`" + ` |
| Disable only the Claude gate | ` + "`lrc hooks disable --surface claude`" + ` |
| Re-enable hooks in this repo | ` + "`lrc hooks enable`" + ` |
| Re-enable only the Claude gate | ` + "`lrc hooks enable --surface claude`" + ` |
| Install global Claude hook | ` + "`lrc hooks install --surface claude`" + ` |
| Remove global Claude hook | ` + "`lrc hooks uninstall --surface claude`" + ` |

## Rules

- Prefer lrc hooks status before mutating hook state when intent is ambiguous.
- Use ` + "`lrc review --staged --skip`" + ` only when the user explicitly asks to skip or bypass review. Never use skip as a fallback after a hook, wrapper, or review failure.
- Use ` + "`lrc hooks disable`" + ` or ` + "`lrc hooks disable --surface claude`" + ` only when the user explicitly asks to disable hooks. Never disable hooks as a fallback for a failing review flow.
- Repo-local disable/enable uses marker files under .git/lrc/: disabled, disabled-git, disabled-claude.
- Global Claude integration lives in ~/.lrc/claude/hooks/ and ~/.claude/settings.json — manage only via lrc hooks install/uninstall.
- Never edit .claude/settings.local.json directly; it is not the control plane when the global install is active.
`
	return strings.ReplaceAll(skill, "\t", "  ")
}
