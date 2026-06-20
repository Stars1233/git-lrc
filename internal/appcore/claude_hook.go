package appcore

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/urfave/cli/v2"
)

// claudeHookPayload is the subset of the Claude Code PreToolUse JSON we care about.
type claudeHookPayload struct {
	ToolName  string                 `json:"tool_name"`
	ToolInput map[string]interface{} `json:"tool_input"`
	CWD       string                 `json:"cwd"`
}

// claudeHookOutput is the JSON written back to Claude Code.
type claudeHookOutput struct {
	HookSpecificOutput claudeHookSpecific `json:"hookSpecificOutput"`
}

type claudeHookSpecific struct {
	HookEventName            string                 `json:"hookEventName"`
	PermissionDecision       string                 `json:"permissionDecision"`
	PermissionDecisionReason string                 `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]interface{} `json:"updatedInput,omitempty"`
}

// claudeRunCommitData is base64-JSON-encoded and passed as --encoded to run-commit.
type claudeRunCommitData struct {
	OriginalTokens []string `json:"original_tokens"`
	CWD            string   `json:"cwd"`
	ProjectDir     string   `json:"project_dir"`
}

// ── output helpers ──────────────────────────────────────────────────────────

func claudeEmitAllow(reason string, updatedInput map[string]interface{}) {
	out := claudeHookOutput{HookSpecificOutput: claudeHookSpecific{
		HookEventName:            "PreToolUse",
		PermissionDecision:       "allow",
		PermissionDecisionReason: reason,
		UpdatedInput:             updatedInput,
	}}
	_ = json.NewEncoder(os.Stdout).Encode(out)
}

func claudeEmitDeny(reason string) {
	out := claudeHookOutput{HookSpecificOutput: claudeHookSpecific{
		HookEventName:            "PreToolUse",
		PermissionDecision:       "deny",
		PermissionDecisionReason: reason,
	}}
	_ = json.NewEncoder(os.Stdout).Encode(out)
}

// ── shell parsing ────────────────────────────────────────────────────────────

var gitCommitRE = regexp.MustCompile(`(?i)^git(\.exe)?\s+commit(\s|$)`)

func isGitCommitCommand(command string) bool {
	return gitCommitRE.MatchString(strings.TrimSpace(command))
}

var shellOperatorSet = map[string]bool{
	"&&": true, "||": true, ";": true,
	"|": true, "&": true, "(": true, ")": true,
}

// shelxSplit splits a shell command string into tokens, handling single and
// double quotes and backslash escapes. Returns an error for unclosed quotes.
func shelxSplit(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inSingle, inDouble := false, false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\'' && !inDouble:
			inSingle = !inSingle
		case ch == '"' && !inSingle:
			inDouble = !inDouble
		case ch == '\\' && !inSingle:
			i++
			if i < len(s) {
				cur.WriteByte(s[i])
			}
		case (ch == ' ' || ch == '\t' || ch == '\n') && !inSingle && !inDouble:
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(ch)
		}
	}
	if inSingle || inDouble {
		return nil, fmt.Errorf("unclosed quote in command")
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}

func hasShellOperators(tokens []string) bool {
	for _, t := range tokens {
		if shellOperatorSet[t] {
			return true
		}
	}
	return false
}

// ── pre-tool-use ─────────────────────────────────────────────────────────────

func runInternalClaudePreToolUse(_ *cli.Context) error {
	rawPayload, err := io.ReadAll(os.Stdin)
	if err != nil {
		claudeEmitDeny("lrc hook: failed to read hook payload")
		return nil
	}

	var payload claudeHookPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		claudeEmitDeny("lrc hook: failed to parse hook payload")
		return nil
	}

	command := ""
	if cmd, ok := payload.ToolInput["command"].(string); ok {
		command = strings.TrimSpace(cmd)
	}
	if command == "" {
		claudeEmitAllow("lrc hook: no command found, passing through", nil)
		return nil
	}

	if !isGitCommitCommand(command) {
		claudeEmitAllow("lrc hook: not a git commit, passing through", nil)
		return nil
	}

	tokens, err := shelxSplit(command)
	if err != nil {
		claudeEmitDeny("lrc hook: could not parse the git commit command safely")
		return nil
	}
	if hasShellOperators(tokens) {
		claudeEmitDeny("lrc hook: git commit with shell operators is not supported; run git commit as a separate command")
		return nil
	}

	// Resolve CWD: prefer tool_input.cwd, then payload.cwd, then os.Getwd.
	cwd := ""
	if s, ok := payload.ToolInput["cwd"].(string); ok {
		cwd = strings.TrimSpace(s)
	}
	if cwd == "" {
		cwd = strings.TrimSpace(payload.CWD)
	}
	if cwd == "" {
		if wd, werr := os.Getwd(); werr == nil {
			cwd = wd
		}
	}
	if cwd == "" {
		claudeEmitDeny("lrc hook: could not determine working directory")
		return nil
	}

	projectDir := strings.TrimSpace(os.Getenv("CLAUDE_PROJECT_DIR"))
	if projectDir == "" {
		projectDir = cwd
	}

	data := claudeRunCommitData{
		OriginalTokens: tokens,
		CWD:            cwd,
		ProjectDir:     projectDir,
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		claudeEmitDeny("lrc hook: failed to encode run-commit data")
		return nil
	}
	encoded := base64.StdEncoding.EncodeToString(dataJSON)

	lrcExe, err := os.Executable()
	if err != nil {
		claudeEmitDeny("lrc hook: failed to locate lrc executable")
		return nil
	}

	updatedInput := make(map[string]interface{}, len(payload.ToolInput))
	for k, v := range payload.ToolInput {
		updatedInput[k] = v
	}
	updatedInput["command"] = claudeBuildRewrittenCommand(payload.ToolName, lrcExe, encoded)

	claudeEmitAllow(
		"lrc hook: git commit intercepted; review gate will run before commit proceeds",
		updatedInput,
	)
	return nil
}

// claudeBuildRewrittenCommand builds the shell command string that Claude will
// execute in place of the original git commit. Quoting is adapted to the host shell.
func claudeBuildRewrittenCommand(toolName, lrcExe, encoded string) string {
	// Single-quote the path — literal in both bash (POSIX) and PowerShell.
	// An embedded single quote is escaped as '' in both shells.
	quoted := "'" + strings.ReplaceAll(lrcExe, "'", "''") + "'"
	if toolName == "PowerShell" {
		// PowerShell requires the & call operator to invoke a quoted path expression.
		return fmt.Sprintf("& %s internal claude run-commit --encoded %s", quoted, encoded)
	}
	return fmt.Sprintf("%s internal claude run-commit --encoded %s", quoted, encoded)
}

// ── run-commit ───────────────────────────────────────────────────────────────

func runInternalClaudeRunCommit(c *cli.Context) error {
	encoded := c.String("encoded")
	if encoded == "" {
		fmt.Fprintln(os.Stderr, "LiveReview: missing --encoded flag for run-commit")
		os.Exit(1)
	}

	dataJSON, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		fmt.Fprintln(os.Stderr, "LiveReview: failed to decode run-commit data")
		os.Exit(1)
	}

	var data claudeRunCommitData
	if err := json.Unmarshal(dataJSON, &data); err != nil {
		fmt.Fprintln(os.Stderr, "LiveReview: failed to parse run-commit data")
		os.Exit(1)
	}

	if len(data.OriginalTokens) < 2 || strings.ToLower(data.OriginalTokens[1]) != "commit" {
		fmt.Fprintln(os.Stderr, "LiveReview: run-commit data contains invalid git commit tokens")
		os.Exit(1)
	}

	if reviewMode == "fake" {
		fmt.Fprintln(os.Stderr, "LiveReview: refusing to use fake-review lrc binary")
		os.Exit(1)
	}

	if err := os.Chdir(data.CWD); err != nil {
		fmt.Fprintf(os.Stderr, "LiveReview: failed to change to working directory %q: %v\n", data.CWD, err)
		os.Exit(1)
	}

	// Check if the Claude hook gate has been disabled for this repo.
	gitDir := claudeGetGitDir()
	lrcDir := filepath.Join(gitDir, "lrc")
	if claudeFileExists(filepath.Join(lrcDir, "disabled")) || claudeFileExists(filepath.Join(lrcDir, "disabled-claude")) {
		fmt.Fprintln(os.Stderr, "LiveReview: Claude review hook disabled for this repository; proceeding with git commit.")
		claudeRunOriginalCommit(data.OriginalTokens, false)
		return nil
	}

	lrcExe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "LiveReview: failed to locate lrc executable")
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "LiveReview: checking whether the current staged tree already has a valid review.")
	fmt.Fprintln(os.Stderr, "LiveReview: if not, a blocking browser review will open before git commit can continue.")

	timeout := os.Getenv("LRC_BLOCKING_REVIEW_TIMEOUT")
	if timeout == "" {
		timeout = "20m"
	}

	var reviewBuf bytes.Buffer
	reviewArgs := []string{"review", "--staged", "--blocking-review", "--blocking-review-timeout", timeout}
	// Safe: re-invokes this same lrc binary (os.Executable) with a fixed subcommand and flags.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	reviewCmd := exec.Command(lrcExe, reviewArgs...)
	reviewCmd.Stdout = io.MultiWriter(os.Stdout, &reviewBuf)
	reviewCmd.Stderr = io.MultiWriter(os.Stderr, &reviewBuf)

	env := os.Environ()
	if msg := claudeExtractCommitMessage(data.OriginalTokens); msg != "" {
		env = append(env, "LRC_INITIAL_MESSAGE="+msg)
	}
	reviewCmd.Env = env

	reviewErr := reviewCmd.Run()
	reviewLog := reviewBuf.String()

	exitCode := 0
	if reviewErr != nil {
		var exitErr *exec.ExitError
		if errors.As(reviewErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "LiveReview: blocking review failed: %v\n", reviewErr)
			os.Exit(1)
		}
	}

	switch exitCode {
	case 0, 2:
		claudeRunOriginalCommit(data.OriginalTokens, true)
	case 1:
		switch {
		case strings.Contains(reviewLog, "attestation already present for current tree"):
			fmt.Fprintln(os.Stderr, "LiveReview: current tree is already reviewed; proceeding with git commit.")
			claudeRunOriginalCommit(data.OriginalTokens, true)
		case strings.Contains(reviewLog, "Commit aborted by user"):
			fmt.Fprintln(os.Stderr, "LiveReview: commit intentionally aborted in the browser; git commit was not run.")
			os.Exit(0)
		default:
			fmt.Fprintln(os.Stderr, "LiveReview: blocking review exited with code 1 before git commit could continue")
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "LiveReview: blocking review failed before git commit could continue")
		os.Exit(1)
	}

	return nil
}

// claudeExtractCommitMessage extracts the -m / --message value from git commit tokens.
func claudeExtractCommitMessage(tokens []string) string {
	for i, t := range tokens {
		if (t == "-m" || t == "--message") && i+1 < len(tokens) {
			return tokens[i+1]
		}
		if strings.HasPrefix(t, "--message=") {
			return strings.TrimPrefix(t, "--message=")
		}
		if strings.HasPrefix(t, "-m") && len(t) > 2 {
			return t[2:]
		}
	}
	return ""
}

func claudeRunOriginalCommit(tokens []string, markHandled bool) {
	env := os.Environ()
	if markHandled {
		env = append(env, "LRC_CLAUDE_REVIEW_HANDLED=1")
	}
	// Safe: re-executes the original git invocation intercepted by this hook, which was
	// validated to start with "git commit" before reaching here.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(tokens[0], tokens[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.ExitCode())
		}
		fmt.Fprintf(os.Stderr, "LiveReview: git commit failed: %v\n", err)
		os.Exit(1)
	}
}

func claudeGetGitDir() string {
	out, err := exec.Command("git", "rev-parse", "--git-dir").Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return ".git"
	}
	return strings.TrimSpace(string(out))
}

func claudeFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
