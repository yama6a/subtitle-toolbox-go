package subviewer_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v5/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/formattest"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func FuzzRead(f *testing.F) {
	formattest.AddSeeds(f, "subviewer/real/*.sub", "lenient/*_time_*.sub", "lenient/*loose*.sub", "lenient/*sixty*.sub", "lenient/*absurd*.sub")
	for _, seed := range []string{
		"[INFORMATION]\n[TITLE]x\n[END INFORMATION]\n[SUBTITLE]\n[COLF]&H0\n00:00:01.50,00:00:04.00\nHello[br]world\n",
		"00:00:05.00,00:00:03.00\nA\r\n\r\n99999:59:59.999,99999:59:59.999\nB",
		"99999:59:59.9996,0:0:1\nA\n0:0:1.5,00:00:02\nB\n",
		"[DELAY]\n-99999999999999999999\n******** START SCRIPT ********\n[00:00:01]\nA\n[00:00:02]\n",
		"[DELAY]359999990\n******** START SCRIPT ********\n[00:00:12]\nA|B||\n[00:00:13]\n\n[00:00:14]\nC\n[00:00:13]\n",
		"[TITLE]\n[x]y\n\xff\x00\n******** START SCRIPT ********\n[00:00:01]\n\xff\n",
		"[a]\n[a]b\nc\n00:00:01.00,00:00:02.00\nA\n[SIZE]\nB\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		for _, lenient := range []bool{false, true} {
			s, warnings, err := subviewer.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, subviewer.ReadOptions{})
			if !formattest.CheckRead(t, s, warnings, err) {
				continue
			}
			checkEnds(t, s)
			for _, v := range []subviewer.Version{subviewer.V1, subviewer.V2} {
				checkWrite(t, s, v)
			}
		}
	})
}

func checkEnds(t *testing.T, s *model.Subtitle) {
	t.Helper()

	for i, c := range s.All() {
		if c.End < c.Start {
			t.Fatalf("cue %d is %v", i, c)
		}
	}
}

// checkWrite writes s in version v and reads the output back. A header with [, | or * can read back as other tags,
// so it only has to read without an error in lenient mode. Otherwise the cues, lines and header stay. A line may
// gain a word joiner and turn | into U+00A6, as the writer escapes text that would read as structure.
func checkWrite(t *testing.T, s *model.Subtitle, v subviewer.Version) {
	t.Helper()

	var out bytes.Buffer
	if _, err := subviewer.Write(&out, s, model.CommonWriteOptions{}, subviewer.WriteOptions{Version: v}); err != nil {
		t.Fatalf("Write returned %v", err)
	}
	if _, _, err := subviewer.Read(bytes.NewReader(out.Bytes()), model.CommonReadOptions{Lenient: true}, subviewer.ReadOptions{}); err != nil {
		t.Fatalf("lenient Read of the written output returned %v", err)
	}
	if hasMarkup(s) {
		return
	}
	back, _, err := subviewer.Read(bytes.NewReader(out.Bytes()), model.CommonReadOptions{}, subviewer.ReadOptions{})
	if err != nil {
		t.Fatalf("Read of the written output returned %v", err)
	}
	var want []model.Cue
	for _, c := range s.All() {
		if lines := markupcore.PlainLines(c.Lines); len(lines) > 0 {
			c.Lines = lines
			want = append(want, c)
		}
	}
	if back.Len() != len(want) {
		t.Fatalf("the written output reads as %d cues, want %d", back.Len(), len(want))
	}
	tolerance := 10 * time.Millisecond
	if v == subviewer.V1 {
		tolerance = time.Second
	}
	for i, c := range back.Cues() {
		w := want[i]
		for _, d := range []time.Duration{c.Start - max(w.Start, 0), c.End - max(w.End, 0)} {
			if d < -tolerance || d > tolerance {
				t.Fatalf("cue %d is %v to %v, want %v to %v", i, c.Start, c.End, w.Start, w.End)
			}
		}
		if got := markupcore.PlainLines(c.Lines); unescaped(got) != unescaped(w.Lines) {
			t.Fatalf("cue %d has the lines %q, want %q", i, got, w.Lines)
		}
	}
	for _, k := range []string{model.MetadataTitle, model.MetadataAuthor} {
		a, _ := s.Metadata(k)
		b, _ := back.Metadata(k)
		if strings.Trim(a, markupcore.TrimSet) != b {
			t.Fatalf("the %s is %q, want %q", k, b, a)
		}
	}
}

// hasMarkup reports whether the header or the metadata of s has a character that the writer cannot keep apart from
// the syntax of the format.
func hasMarkup(s *model.Subtitle) bool {
	d, _ := s.FormatData("subviewer")
	for _, v := range plain(d).(map[string]any) {
		if strings.ContainsAny(strings.Join(flatten(v), ""), "[|*") {
			return true
		}
	}
	for _, v := range s.AllMetadata() {
		if strings.ContainsAny(v, "[|*") {
			return true
		}
	}
	return false
}

func unescaped(lines []string) string {
	return strings.NewReplacer("\u2060", "", "\u00a6", "|").Replace(strings.Join(lines, "\n"))
}

func flatten(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case map[string]any:
		var out []string
		for k, item := range v {
			out = append(out, k)
			out = append(out, flatten(item)...)
		}
		return out
	}
	return nil
}
