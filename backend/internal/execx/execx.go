package execx

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CmdError carries the captured stderr so callers can surface it verbatim in the UI.
type CmdError struct {
	Cmd    string
	Stderr string
	Err    error
}

func (e *CmdError) Error() string {
	if strings.TrimSpace(e.Stderr) != "" {
		return fmt.Sprintf("%s: %v: %s", e.Cmd, e.Err, strings.TrimSpace(e.Stderr))
	}
	return fmt.Sprintf("%s: %v", e.Cmd, e.Err)
}

func (e *CmdError) Unwrap() error { return e.Err }

// Run executes name with args and returns stdout. On failure, the returned error
// is a *CmdError containing captured stderr.
func Run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), &CmdError{
			Cmd:    name + " " + strings.Join(args, " "),
			Stderr: stderr.String(),
			Err:    err,
		}
	}
	return stdout.String(), nil
}

// RunLong is Run without the 30s timeout, for commands that may legitimately take longer.
func RunLong(ctx context.Context, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return stdout.String(), &CmdError{
			Cmd:    name + " " + strings.Join(args, " "),
			Stderr: stderr.String(),
			Err:    err,
		}
	}
	return stdout.String(), nil
}
