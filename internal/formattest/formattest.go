// Package formattest holds the test helpers that the format packages share: the read checks of the fuzz targets,
// the fuzz seeds, a writer that always fails and the subtitle builder.
package formattest

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// CheckRead fails tb when a read breaks the fuzz rules of CLAUDE.md: err must hold a *model.ParseError and its text must not end in a period, the cues
// of s are in start order, each start and end is whole milliseconds above -timecode.MaxTime and below
// timecode.MaxTime, and each warning is Skipped or Repaired. A time may be negative, as model.Cue allows. It
// returns false when err is not nil, so the caller skips the write checks.
func CheckRead(tb testing.TB, s *model.Subtitle, warnings []model.ParseWarning, err error) bool {
	tb.Helper()

	if err != nil {
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			tb.Fatalf("read returned %v, which is no *model.ParseError", err)
		}
		if strings.HasSuffix(err.Error(), ".") {
			tb.Fatalf("the error text %q ends in a period", err.Error())
		}
		return false
	}
	checkCues(tb, s)
	for _, w := range warnings {
		if w.Action != model.Skipped && w.Action != model.Repaired {
			tb.Fatalf("the warning %q has no action", w.Message)
		}
	}
	return true
}

// AddSeeds adds to f every file that a pattern matches. A pattern is a filepath.Match pattern relative to the
// testdata folder, such as "mpl2/real/*.txt". Folders and SOURCES.md files are skipped. A pattern that
// matches no file fails f, so a moved testdata folder cannot drop the seeds unnoticed.
func AddSeeds(f *testing.F, patterns ...string) {
	f.Helper()

	root := testfiles.Root(f)
	for _, pattern := range patterns {
		paths, err := filepath.Glob(testfiles.Path(f, pattern))
		if err != nil {
			f.Fatal(err)
		}
		var added int
		for _, p := range paths {
			if filepath.Base(p) == "SOURCES.md" {
				continue
			}
			if info, err := os.Stat(p); err != nil || info.IsDir() {
				continue
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				f.Fatal(err)
			}
			f.Add(testfiles.Read(f, filepath.ToSlash(rel)))
			added++
		}
		if added == 0 {
			f.Fatalf("the seed pattern %q matches no file", pattern)
		}
	}
}

func checkCues(tb testing.TB, s *model.Subtitle) {
	tb.Helper()

	var previous time.Duration
	for i, c := range s.All() {
		if i > 0 && c.Start < previous {
			tb.Fatalf("cue %d starts before cue %d", i, i-1)
		}
		previous = c.Start
		for _, d := range []time.Duration{c.Start, c.End} {
			if d%time.Millisecond != 0 || d <= -timecode.MaxTime || d >= timecode.MaxTime {
				tb.Fatalf("cue %d has the time %v", i, d)
			}
		}
	}
}

// ErrWrite is the error of every write to ErrWriter.
var ErrWrite = errors.New("disk full")

// ErrWriter is an io.Writer whose every write fails with ErrWrite.
type ErrWriter struct{}

func (ErrWriter) Write([]byte) (int, error) { return 0, ErrWrite }

// Subtitle returns a new subtitle that holds cues. It panics when a cue fails validation.
func Subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	if err := s.AddCues(cues...); err != nil {
		panic(err)
	}
	return s
}
