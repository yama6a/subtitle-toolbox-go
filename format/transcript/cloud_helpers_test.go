package transcript_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// cloudFormat is one of the cloud speech-to-text formats with its testdata folder.
type cloudFormat struct {
	folder string
	read   func(io.Reader, model.CommonReadOptions, transcript.ReadOptions) (*model.Subtitle, []model.ParseWarning, error)
	write  func(io.Writer, *model.Subtitle, model.CommonWriteOptions, transcript.WriteOptions) error
}

func (f cloudFormat) parse(t *testing.T, content string, opts transcript.ReadOptions) *model.Subtitle {
	t.Helper()

	s, _, err := f.read(strings.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(t, err)
	return s
}

func (f cloudFormat) parseFile(t *testing.T, name string, opts transcript.ReadOptions) *model.Subtitle {
	t.Helper()

	return f.parse(t, string(testfiles.Read(t, f.folder+"/real/"+name)), opts)
}

// output writes s and returns the decoded JSON.
func (f cloudFormat) output(t *testing.T, s *model.Subtitle) orderedjson.Value {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, f.write(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{}))
	v, err := orderedjson.Decode(out.Bytes())
	require.NoError(t, err)
	return v
}

func (f cloudFormat) realFiles(tb testing.TB) []string {
	tb.Helper()

	entries, err := os.ReadDir(testfiles.Path(tb, f.folder+"/real"))
	require.NoError(tb, err)
	var files []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".json" {
			files = append(files, e.Name())
		}
	}
	return files
}

// testRoundTrip checks that each real file reads and writes back to the same JSON data, with and without word times.
// The reader makes no cue of an empty text, so the test does not need those entries.
func (f cloudFormat) testRoundTrip(t *testing.T) {
	t.Helper()

	for _, file := range f.realFiles(t) {
		for name, opts := range map[string]transcript.ReadOptions{"default": {}, "words": withWords} {
			t.Run(file+"/"+name, func(t *testing.T) {
				t.Parallel()

				source, err := orderedjson.Decode(testfiles.Read(t, f.folder+"/real/"+file))
				require.NoError(t, err)
				got := f.output(t, f.parseFile(t, file, opts))
				assert.Equal(t, orderedjson.Block(orderedjson.Canonical(source)), orderedjson.Block(orderedjson.Canonical(got)))
			})
		}
	}
}

// cloudWarnings returns the message and the block index of each warning.
func cloudWarnings(w []model.ParseWarning) []string {
	out := make([]string, len(w))
	for i, x := range w {
		out[i] = fmt.Sprintf("%s|%d|%s", x.Message, x.Block, x.Action)
	}
	return out
}

// cloudBroken is one row of the table of a word with a bad field. Strict mode returns the message. Lenient mode
// skips the item and keeps the rest.
type cloudBroken struct {
	name    string
	content string
	message string
	cues    []string
	block   int
}

func (f cloudFormat) testBroken(t *testing.T, tests []cloudBroken) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := f.read(strings.NewReader(tt.content), model.CommonReadOptions{}, transcript.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.message, perr.Msg)

			s, warnings, err := f.read(strings.NewReader(tt.content), model.CommonReadOptions{Lenient: true}, transcript.ReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.cues, describe(s))
			assert.Equal(t, []string{fmt.Sprintf("%s|%d|%s", tt.message, tt.block, model.Skipped)}, cloudWarnings(warnings))
		})
	}
}

// cloudFuzz runs the fuzz target of f. The seeds are the real files of f and the inputs in seeds.
func cloudFuzz(f *testing.F, cf cloudFormat, seeds ...string) {
	f.Helper()

	for _, name := range cf.realFiles(f) {
		f.Add(testfiles.Read(f, cf.folder+"/real/"+name))
	}
	for _, seed := range append(seeds, `[1]`, `{`, ``) {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			common := model.CommonReadOptions{Lenient: lenient}
			for _, opts := range []transcript.ReadOptions{{}, withWords} {
				s, warnings, err := cf.read(bytes.NewReader(content), common, opts)
				if err != nil {
					var perr *model.ParseError
					if !errors.As(err, &perr) {
						t.Fatalf("the reader returned %v, which is no *ParseError", err)
					}
					continue
				}
				checkCues(t, s)
				for _, w := range warnings {
					if w.Action != model.Skipped && w.Action != model.Repaired {
						t.Fatalf("the warning %q has no action", w.Message)
					}
				}
				var out bytes.Buffer
				if err := cf.write(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil && !errors.Is(err, model.ErrUnwritable) {
					t.Fatalf("the writer returned %v, which does not match ErrUnwritable", err)
				}
			}
		}
	})
}

// cloudBench reads and writes content, which holds about 1 MB. The benchmarks have no time limit, so a rise from
// linear to quadratic cost shows in the numbers of the bench run.
func cloudBench(b *testing.B, f cloudFormat, name, content string) {
	b.Helper()

	b.Run("read "+name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(content)))
		for b.Loop() {
			if _, _, err := f.read(strings.NewReader(content), model.CommonReadOptions{}, withWords); err != nil {
				b.Fatal(err)
			}
		}
	})
	s, _, err := f.read(strings.NewReader(content), model.CommonReadOptions{}, withWords)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("write "+name, func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(content)))
		for b.Loop() {
			if err := f.write(io.Discard, s, model.CommonWriteOptions{}, transcript.WriteOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

// cloudHasPath reports whether the path exists in v. A string in path is a key and an int is an index.
func cloudHasPath(v orderedjson.Value, path ...any) bool {
	for _, p := range path {
		switch p := p.(type) {
		case string:
			var ok bool
			if v, ok = v.Get(p); !ok {
				return false
			}
		case int:
			if p >= len(v.Items()) {
				return false
			}
			v = v.Items()[p]
		}
	}
	return true
}
