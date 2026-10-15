package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	t.Parallel()

	const usage = "usage: stbox --version\n"
	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantStderr string
		wantCode   int
	}{
		{"long version flag", []string{"--version"}, buildVersion() + "\n", "", 0},
		{"short version flag", []string{"-V"}, buildVersion() + "\n", "", 0},
		{"no arguments", nil, "", usage, 2},
		{"unknown argument", []string{"convert"}, "", usage, 2},
		{"version flag with extra argument", []string{"--version", "x"}, "", usage, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tt.args, &stdout, &stderr)
			assert.Equal(t, tt.wantCode, code)
			assert.Equal(t, tt.wantStdout, stdout.String())
			assert.Equal(t, tt.wantStderr, stderr.String())
		})
	}
}
