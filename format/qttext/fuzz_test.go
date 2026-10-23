package qttext_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func FuzzRead(f *testing.F) {
	for _, dir := range []string{"qttext", realDir, "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".txt" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"{QTtext}{timeScale:600}{timeStamps:relative}\n[00:00:01.300]\nOne\n[00:00:02.000]\n[00:00:01.000]\nTwo\n[00:00:02.000]\n",
		"{QTtext}{timeScale:30}\n[00:00:01.45]\n{bold}One{plain} & <i>two\n[00:00:04.00]\n",
		"{QTtext}{timeScale:1}\n[99999:59:59.9999999999999999999]\nx\n",
		"{QTtext}{timeScale:99999999999999999999}\n[00:00:01.99999999999999999999]\nx\n",
		"{QTtext}{timeStamps:relative}\n[99999:00:00]\nx\n[99999:00:00]\ny\n",
		"{QTtext}{bold}\n[00:00:01.00][01:00:00.299]</i>\n[00:00:09.00]\n",
		"{QTtext}\n[00:00:01.00]\n{italic}a\n[00:00:02.00]\n\n[00:00:03.00]\nb\n[00:00:04.00]\n\n",
		"[00:00:01.00]\nOne\n[00:00:00.00]\nTwo\n",
		"{QTtext}\n[00:00:01.00]caf\xE9\x00\x0b {x:\n\t[100000:00:00.00]",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := qttext.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, qttext.ReadOptions{})
			if err != nil {
				var perr *model.ParseError
				if !errors.As(err, &perr) {
					t.Fatalf("Read returned %v, which is no *ParseError", err)
				}
				continue
			}
			var previous time.Duration
			for i, c := range s.All() {
				if i > 0 && c.Start < previous {
					t.Fatalf("cue %d starts before cue %d", i, i-1)
				}
				previous = c.Start
				for _, d := range []time.Duration{c.Start, c.End} {
					if d%time.Millisecond != 0 || d < 0 || d >= timecode.MaxTime {
						t.Fatalf("cue %d has the time %v", i, d)
					}
				}
			}
			for _, w := range warnings {
				if w.Action != model.Skipped && w.Action != model.Repaired {
					t.Fatalf("the warning %q has no action", w.Message)
				}
			}
			var out bytes.Buffer
			if err := qttext.Write(&out, s, model.CommonWriteOptions{}, qttext.WriteOptions{}); err != nil {
				t.Fatalf("Write returned %v", err)
			}
			if looksLikeATimestamp(s) {
				continue
			}
			back, _, err := qttext.Read(&out, model.CommonReadOptions{}, qttext.ReadOptions{})
			if err != nil {
				t.Fatalf("Read of the written output returned %v", err)
			}
			if got, want := cueTexts(back), cueTexts(s); !slices.Equal(got, want) {
				t.Fatalf("the written output reads as the cues %q, want %q", got, want)
			}
		}
	})
}

// cueTexts returns the text of each cue, sorted, because a relative file can move a cue and change the order.
func cueTexts(s *model.Subtitle) []string {
	texts := make([]string, 0, s.Len())
	for _, c := range s.All() {
		texts = append(texts, c.Text())
	}
	slices.Sort(texts)
	return texts
}

// looksLikeATimestamp reports whether the text of a cue line or of a stored source line reads as a timestamp line when written,
// which issue 190 covers.
func looksLikeATimestamp(s *model.Subtitle) bool {
	for _, c := range s.All() {
		data, _ := c.FormatData("qttext")
		source, _ := data.Strings("source")
		for _, line := range slices.Concat(c.Lines, source) {
			if timestampLine.MatchString(strings.Trim(markupcore.PlainText(line), " \t\n\r\x00\v")) {
				return true
			}
		}
	}
	return false
}

var timestampLine = regexp.MustCompile(`^\[\d+:[0-5]\d:[0-5]\d(\.\d+)?\]`)
