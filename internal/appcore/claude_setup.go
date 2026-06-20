package appcore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/flock"
	"github.com/urfave/cli/v2"
)

const setupKeyPrompt = "Paste your Gemini API key:"

type setupStatusPayload struct {
	State     string `json:"state"`
	Message   string `json:"message,omitempty"`
	UpdatedAt int64  `json:"updated_at"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	LogTail   string `json:"log_tail,omitempty"`
}

func setupSessionDir() (string, error) {
	root := os.Getenv("LRC_PLUGIN_DATA_DIR")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not determine home directory: %w", err)
		}
		root = filepath.Join(home, ".claude", "plugins", "data", "lrc")
	}
	return filepath.Join(root, "setup-session"), nil
}

func setupStatusFile(dir string) string { return filepath.Join(dir, "status.json") }
func setupLogFile(dir string) string    { return filepath.Join(dir, "output.log") }
func setupKeyFile(dir string) string    { return filepath.Join(dir, "gemini-key.txt") }
func setupLockFile(dir string) string   { return filepath.Join(dir, "worker.lock") }

func readSetupStatus(dir string) setupStatusPayload {
	data, err := os.ReadFile(setupStatusFile(dir))
	if err != nil {
		return setupStatusPayload{State: "idle"}
	}
	var s setupStatusPayload
	if err := json.Unmarshal(data, &s); err != nil {
		return setupStatusPayload{State: "unknown"}
	}
	return s
}

func writeSetupStatus(dir string, s setupStatusPayload) {
	_ = os.MkdirAll(dir, 0755)
	s.UpdatedAt = time.Now().Unix()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(setupStatusFile(dir), data, 0644)
}

func readLogTailStr(dir string, maxBytes int) string {
	data, err := os.ReadFile(setupLogFile(dir))
	if err != nil {
		return ""
	}
	if len(data) <= maxBytes {
		return string(data)
	}
	return string(data[len(data)-maxBytes:])
}

// setupCommandEnv returns an environment with common lrc install dirs prepended
// to PATH so that the spawned lrc setup subprocess can find itself.
func setupCommandEnv() []string {
	env := os.Environ()
	var extra []string
	if home, err := os.UserHomeDir(); err == nil {
		extra = append(extra, filepath.Join(home, ".local", "bin"))
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		extra = append(extra, filepath.Join(la, "Programs", "lrc"))
	}
	if len(extra) == 0 {
		return env
	}
	prefix := strings.Join(extra, string(os.PathListSeparator))
	for i, e := range env {
		if idx := strings.IndexByte(e, '='); idx != -1 && strings.EqualFold(e[:idx], "PATH") {
			env[i] = "PATH=" + prefix + string(os.PathListSeparator) + e[idx+1:]
			return env
		}
	}
	return append(env, "PATH="+prefix)
}

// ── start ─────────────────────────────────────────────────────────────────────

func runInternalClaudeSetupStart(_ *cli.Context) error {
	dir, err := setupSessionDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup start: %v\n", err)
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup start: cannot create session dir: %v\n", err)
		return nil
	}

	fl := flock.New(setupLockFile(dir))
	got, err := fl.TryLock()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup start: lock error: %v\n", err)
		return nil
	}
	if !got {
		// Worker already running — just report its current status.
		s := readSetupStatus(dir)
		fmt.Println(s.Message)
		return nil
	}
	// We acquired the lock, meaning no worker is running. Release it so the
	// worker process can acquire it.
	_ = fl.Unlock()

	lrcExe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup start: cannot find lrc executable: %v\n", err)
		return nil
	}

	writeSetupStatus(dir, setupStatusPayload{
		State:   "starting",
		Message: "launching lrc setup",
	})

	devNull, _ := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	// Safe: re-invokes this same lrc binary (os.Executable) with a fixed subcommand.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	workerCmd := exec.Command(lrcExe, "internal", "claude", "setup", "worker")
	if devNull != nil {
		workerCmd.Stdin = devNull
		workerCmd.Stdout = devNull
		workerCmd.Stderr = devNull
		defer devNull.Close()
	}
	setupSessionDetach(workerCmd)

	if err := workerCmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup start: failed to spawn worker: %v\n", err)
		return nil
	}
	go func() { _ = workerCmd.Wait() }()

	// Poll up to 3 s for the worker to report a meaningful state.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := readSetupStatus(dir)
		switch s.State {
		case "running", "awaiting_key", "completed":
			fmt.Println(s.Message)
			return nil
		case "failed":
			fmt.Fprintln(os.Stderr, readLogTailStr(dir, 1200))
			os.Exit(1)
		}
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println(readSetupStatus(dir).Message)
	return nil
}

// ── worker ────────────────────────────────────────────────────────────────────

func runInternalClaudeSetupWorker(_ *cli.Context) error {
	dir, err := setupSessionDir()
	if err != nil {
		return fmt.Errorf("lrc setup worker: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("lrc setup worker: cannot create session dir: %w", err)
	}

	fl := flock.New(setupLockFile(dir))
	got, err := fl.TryLock()
	if err != nil {
		return fmt.Errorf("lrc setup worker: lock error: %w", err)
	}
	if !got {
		return fmt.Errorf("lrc setup worker: another worker is already running")
	}
	defer fl.Unlock()

	// Reset session files.
	_ = os.WriteFile(setupLogFile(dir), nil, 0644)
	_ = os.Remove(setupKeyFile(dir))

	lrcExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("lrc setup worker: cannot find lrc executable: %w", err)
	}

	// Build a combined stdout+stderr pipe so the worker captures all output.
	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("lrc setup worker: pipe: %w", err)
	}

	// Safe: re-invokes this same lrc binary (os.Executable) with a fixed subcommand.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	proc := exec.Command(lrcExe, "setup")
	proc.Env = setupCommandEnv()
	proc.Stdout = pw
	proc.Stderr = pw

	stdinPipe, err := proc.StdinPipe()
	if err != nil {
		pw.Close()
		pr.Close()
		return fmt.Errorf("lrc setup worker: stdin pipe: %w", err)
	}

	if err := proc.Start(); err != nil {
		pw.Close()
		pr.Close()
		stdinPipe.Close()
		writeSetupStatus(dir, setupStatusPayload{
			State:   "failed",
			Message: "failed to start lrc setup",
		})
		return nil
	}
	pw.Close() // Close write end in parent — subprocess holds the only write reference.

	// Collect exit code in a goroutine so we can poll without calling Wait twice.
	var (
		exitMu   sync.Mutex
		exitCode int
	)
	doneCh := make(chan struct{})
	go func() {
		if werr := proc.Wait(); werr != nil {
			if exitErr, ok := werr.(*exec.ExitError); ok {
				exitMu.Lock()
				exitCode = exitErr.ExitCode()
				exitMu.Unlock()
			}
		}
		close(doneCh)
	}()

	writeSetupStatus(dir, setupStatusPayload{
		State:   "running",
		Message: "lrc setup started",
	})

	logFile, err := os.OpenFile(setupLogFile(dir), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		_ = proc.Process.Kill()
		<-doneCh
		return fmt.Errorf("lrc setup worker: open log: %w", err)
	}
	defer logFile.Close()

	rolling := make([]byte, 0, len(setupKeyPrompt)+128)
	keySent := false
	reader := bufio.NewReader(pr)

outer:
	for {
		b, rerr := reader.ReadByte()
		if rerr != nil {
			break
		}
		_, _ = logFile.Write([]byte{b})
		rolling = append(rolling, b)
		if excess := len(rolling) - (len(setupKeyPrompt) + 128); excess > 0 {
			rolling = rolling[excess:]
		}

		if keySent || !bytes.Contains(rolling, []byte(setupKeyPrompt)) {
			continue
		}

		writeSetupStatus(dir, setupStatusPayload{
			State:   "awaiting_key",
			Message: "Paste the Gemini API key in chat to continue setup",
		})

		// Pause stdout reading while waiting for the key file.
		// In practice lrc setup waits for stdin at this point, so the pipe won't fill.
		for {
			keyData, kerr := os.ReadFile(setupKeyFile(dir))
			if kerr == nil {
				if key := strings.TrimSpace(string(keyData)); key != "" {
					_, _ = fmt.Fprintln(stdinPipe, key)
					stdinPipe.Close()
					keySent = true
					writeSetupStatus(dir, setupStatusPayload{
						State:   "key_sent",
						Message: "Gemini API key submitted to setup",
					})
					continue outer
				}
			}
			// Stop if the process exited while we were waiting.
			select {
			case <-doneCh:
				break outer
			default:
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	// Drain any remaining output so the subprocess can exit cleanly.
	_, _ = io.Copy(logFile, reader)
	pr.Close()

	<-doneCh // Wait for Wait() goroutine to finish.
	exitMu.Lock()
	code := exitCode
	exitMu.Unlock()

	if code == 0 {
		writeSetupStatus(dir, setupStatusPayload{
			State:    "completed",
			Message:  "lrc setup completed",
			ExitCode: &code,
		})
	} else {
		logTail := readLogTailStr(dir, 1200)
		writeSetupStatus(dir, setupStatusPayload{
			State:    "failed",
			Message:  "lrc setup failed",
			ExitCode: &code,
			LogTail:  logTail,
		})
	}
	return nil
}

// ── submit-key ────────────────────────────────────────────────────────────────

func runInternalClaudeSetupSubmitKey(c *cli.Context) error {
	key := strings.TrimSpace(c.String("key"))
	if key == "" {
		fmt.Fprintln(os.Stderr, "Gemini API key input was empty")
		os.Exit(1)
	}

	dir, err := setupSessionDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup submit-key: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup submit-key: cannot create session dir: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(setupKeyFile(dir), []byte(key+"\n"), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup submit-key: cannot write key file: %v\n", err)
		os.Exit(1)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		s := readSetupStatus(dir)
		switch s.State {
		case "completed":
			fmt.Println(s.Message)
			return nil
		case "failed":
			fmt.Fprintln(os.Stderr, readLogTailStr(dir, 1200))
			os.Exit(1)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// Timed out — print last known status message.
	fmt.Println(readSetupStatus(dir).Message)
	return nil
}

// ── status ────────────────────────────────────────────────────────────────────

func runInternalClaudeSetupStatus(_ *cli.Context) error {
	dir, err := setupSessionDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lrc setup status: %v\n", err)
		return nil
	}

	s := readSetupStatus(dir)
	data, _ := json.MarshalIndent(s, "", "  ")
	fmt.Println(string(data))

	if tail := readLogTailStr(dir, 400); tail != "" {
		fmt.Print("\n--- log tail ---\n")
		fmt.Print(tail)
	}
	return nil
}
