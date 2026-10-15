package main

import (
	"bytes"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
