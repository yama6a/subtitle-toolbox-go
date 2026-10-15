package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/cli"
)

func TestBinaryPrintsWhatRunPrints(t *testing.T) {
	t.Parallel()

	bin := filepath.Join(t.TempDir(), "stbox")
	build := exec.CommandContext(t.Context(), "go", "build", "-ldflags", "-X main.version=1.2.3", "-o", bin, ".")
	out, err := build.CombinedOutput()
	require.NoError(t, err, "%s", out)

	tests := []struct {
		name  string
		words []string
	}{
		{"version", []string{"--version"}},
		{"short version", []string{"-V"}},
		{"help", []string{"help"}},
		{"unknown command", []string{"merge"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			cmd := exec.CommandContext(t.Context(), bin, tt.words...) //nolint:gosec // the test runs the binary it built
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			code := 0
			var exitErr *exec.ExitError
			if err := cmd.Run(); errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				require.NoError(t, err)
			}

			var wantOut, wantErr bytes.Buffer
			runErr := cli.Run(t.Context(), tt.words, cli.Env{
				Stdin: strings.NewReader(""), Stdout: &wantOut, Stderr: &wantErr, Version: "1.2.3",
			})
			assert.Equal(t, cli.ExitCode(runErr), code)
			assert.Equal(t, wantOut.String(), stdout.String())
			assert.Equal(t, wantErr.String(), stderr.String())
		})
	}
}

// TestASecondSignalEndsTheProcess runs the test binary again as a process that blocks on standard input after
// the first signal, as a read of "-" does.
func TestASecondSignalEndsTheProcess(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_SIGNAL_HELPER") == "1" {
		ctx, stop := notifyContext()
		defer stop()
		fmt.Println("ready")
		<-ctx.Done()
		fmt.Println("stopped")
		_, _ = io.ReadAll(os.Stdin)
		return
	}
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no SIGINT for a child process")
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestASecondSignalEndsTheProcess$") //nolint:gosec // the test runs its own binary
	cmd.Env = append(os.Environ(), "STBOX_SIGNAL_HELPER=1")
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	lines := bufio.NewScanner(stdout)
	require.True(t, lines.Scan())
	require.Equal(t, "ready", lines.Text())
	require.NoError(t, cmd.Process.Signal(os.Interrupt))
	require.True(t, lines.Scan())
	require.Equal(t, "stopped", lines.Text())

	// The default handling returns in the goroutine of context.AfterFunc, a moment after ctx ends.
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	var err2 error
	for done := false; !done; {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case err2 = <-waited:
			done = true
		case <-time.After(50 * time.Millisecond):
		}
	}
	var exitErr *exec.ExitError
	require.ErrorAs(t, err2, &exitErr)
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	assert.Equal(t, syscall.SIGINT, status.Signal())
}
