package golden_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/golden"
)

// recorder records failures instead of failing the test.
type recorder struct {
	testing.TB
	failed bool
}

func (r *recorder) Helper() {}

func (r *recorder) Errorf(string, ...any) { r.failed = true }

func (r *recorder) Fatalf(string, ...any) { r.failed = true }

func TestEqual(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		stored     string
		got        string
		update     bool
		wantFailed bool
		wantFile   string
	}{
		{name: "match", stored: "a\nb\n", got: "a\nb\n", wantFile: "a\nb\n"},
		{name: "differ", stored: "a\nb\n", got: "a\nc\n", wantFailed: true, wantFile: "a\nb\n"},
		{name: "update writes", stored: "old\n", got: "new\n", update: true, wantFile: "new\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "a.out.srt")
			require.NoError(t, os.WriteFile(path, []byte(tt.stored), 0o600))
			r := &recorder{TB: t}
			golden.Equal(r, path, []byte(tt.got), tt.update)
			assert.Equal(t, tt.wantFailed, r.failed)
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, tt.wantFile, string(got))
		})
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		stored     *string
		got        string
		update     bool
		wantFailed bool
		wantGolden *string
	}{
		{name: "no golden, equal to the input", input: "a\n", got: "a\n"},
		{name: "no golden, BOM and CR LF in the input", input: "\ufeffa\r\nb\r\n", got: "a\nb\n"},
		{name: "no golden, differs", input: "a\n", got: "b\n", wantFailed: true},
		{name: "golden matches", input: "a\n", stored: new("b\n"), got: "b\n", wantGolden: new("b\n")},
		{name: "golden differs", input: "a\n", stored: new("b\n"), got: "c\n", wantFailed: true, wantGolden: new("b\n")},
		{name: "update writes a golden", input: "a\n", got: "b\n", update: true, wantGolden: new("b\n")},
		{name: "update deletes a golden equal to the input", input: "a\r\n", stored: new("b\n"), got: "a\n", update: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			input := filepath.Join(dir, "a.srt")
			path := filepath.Join(dir, "a.srt.out.srt")
			require.NoError(t, os.WriteFile(input, []byte(tt.input), 0o600))
			if tt.stored != nil {
				require.NoError(t, os.WriteFile(path, []byte(*tt.stored), 0o600))
			}
			r := &recorder{TB: t}
			golden.RoundTrip(r, input, path, []byte(tt.got), tt.update)
			assert.Equal(t, tt.wantFailed, r.failed)
			got, err := os.ReadFile(path)
			if tt.wantGolden == nil {
				require.ErrorIs(t, err, os.ErrNotExist)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, *tt.wantGolden, string(got))
		})
	}
}

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{"a\nb\n", "a\nb\n"},
		{"\ufeffa\r\nb\r\n", "a\nb\n"},
		{"a\rb\r\n\r", "a\nb\n\n"},
		{"\ufeff\ufeffa", "\ufeffa"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.in), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, string(golden.Normalize([]byte(tt.in))))
		})
	}
}
