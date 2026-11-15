package qttext_test

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/qttext"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "qttext/*.txt", "qttext/real/*.txt", "lenient/*.txt")
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
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			var out bytes.Buffer
			if _, err := qttext.Write(&out, s, model.CommonWriteOptions{}, qttext.WriteOptions{}); err != nil {
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
