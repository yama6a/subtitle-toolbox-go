// Package formattest holds the test helpers that the format packages share: the read checks of the fuzz targets and the fuzz seeds.
package formattest

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// CheckRead fails tb when a read breaks the fuzz rules of CLAUDE.md: err must hold a *model.ParseError, the cues
// of s are in start order, each start and end is whole milliseconds, 0 or more and below timecode.MaxTime, and
// each warning is Skipped or Repaired. It returns false when err is not nil, so the caller skips the write checks.
func CheckRead(tb testing.TB, s *model.Subtitle, warnings []model.ParseWarning, err error) bool {
	tb.Helper()

	if err != nil {
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			tb.Fatalf("read returned %v, which is no *model.ParseError", err)
		}
		return false
	}
	var previous time.Duration
	for i, c := range s.All() {
		if i > 0 && c.Start < previous {
			tb.Fatalf("cue %d starts before cue %d", i, i-1)
		}
		previous = c.Start
		for _, d := range []time.Duration{c.Start, c.End} {
			if d%time.Millisecond != 0 || d < 0 || d >= timecode.MaxTime {
				tb.Fatalf("cue %d has the time %v", i, d)
			}
		}
	}
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
