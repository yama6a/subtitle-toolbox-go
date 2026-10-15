package model_test

import (
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	opAddCue = iota
	opSetCue
	opRemoveCue
	opAddComment
	opRemoveCuesWhere
	opCount
)

// FuzzSubtitleOps reads each 3 bytes of the input as one operation and checks the invariants after each step.
func FuzzSubtitleOps(f *testing.F) {
	f.Add([]byte{opAddCue, 5, 1, opAddCue, 1, 1, opAddComment, 1, 0, opSetCue, 0, 9, opRemoveCue, 1, 0})
	f.Add([]byte{opAddComment, 3, 0, opAddCue, 2, 0, opAddCue, 2, 1, opRemoveCuesWhere, 2, 0, opAddComment, 0, 0})
	f.Add([]byte{opAddCue, 0, 0, opAddCue, 0, 0, opSetCue, 1, 0, opSetCue, 0, 255, opRemoveCuesWhere, 0, 1})

	f.Fuzz(func(t *testing.T, data []byte) {
		s := &model.Subtitle{}
		for step := 0; step+2 < len(data); step += 3 {
			op, a, b := data[step]%opCount, int(data[step+1]), int(data[step+2])
			at := time.Duration(a-64) * 100 * time.Millisecond
			switch op {
			case opAddCue:
				s.AddCue(model.Cue{Start: at, End: at + time.Duration(b)*time.Millisecond})
			case opSetCue:
				if s.Len() > 0 {
					_ = s.SetCue(b%s.Len(), model.Cue{Start: at, End: at + time.Second})
				}
			case opRemoveCue:
				_ = s.RemoveCue(a % (s.Len() + 1))
			case opAddComment:
				_ = s.AddComment("c", a-8)
			case opRemoveCuesWhere:
				s.RemoveCuesWhere(func(c model.Cue) bool { return c.Start/time.Second%time.Duration(b%4+2) == 0 })
			}

			cueStarts := starts(s)
			if !slices.IsSorted(cueStarts) {
				t.Fatalf("step %d: cues out of start order: %v", step/3, cueStarts)
			}
			comments := s.Comments()
			if !slices.IsSortedFunc(comments, func(x, y model.Comment) int { return x.BeforeCue - y.BeforeCue }) {
				t.Fatalf("step %d: comments out of order: %v", step/3, comments)
			}
			for _, c := range comments {
				if c.BeforeCue < 0 {
					t.Fatalf("step %d: comment with a negative index: %v", step/3, comments)
				}
			}
		}
	})
}

var (
	fuzzKeys = []string{
		"ass", "csv", "lrc", "microdvd", "mpsub", "qttext", "sami", "subviewer", "ttml", "vtt", "dcp", "itt",
		"mcc", "scc", "spruce-stl", "stl", "ffmeta-chapters", "srt", model.ImageKey, "whisper",
	}
	fuzzFields = []string{
		"header", "roles", "width", "delimiter", "timeFormat", "frameRate", "quotedColumns", "identifier", "text",
		"lines", "codes", "color", "tags", "otherCodes", "coordinates", "x1", "x2", "y1", "y2", "blocks", "comments",
		"attributes", "html", "paragraphs", "namespaces", "styles", "regions", "fields", "loadFonts", "timeBase",
		"columns", "dialect", "timeCodeRate", "firstSubtitleNumber", "styleFormat", "commentEvents", "png", "forced",
		"12", "0", "-5", "012", "1.5",
	}
	fuzzStrings = []string{"", "a", ",", "\t", "1/1000", "0/1", "interop", "hh:mm:ss:ff", "2.0", strings.Repeat("0aF", 85) + "b"}
)

// FuzzDataCheck builds a tree of format data from the input and runs the type check of every schema key on it.
func FuzzDataCheck(f *testing.F) {
	f.Add([]byte{6, 3, 0, 2, 5, 1, 1, 6, 4, 13, 2, 1, 14, 2, 2, 15, 2, 3, 16, 2, 4})
	f.Add([]byte{6, 2, 9, 5, 2, 6, 3, 10, 4, 0, 11, 0, 12, 5, 1, 4, 1, 3, 4, 8})
	f.Add([]byte{6, 1, 19, 5, 1, 4, 9})

	f.Fuzz(func(t *testing.T, in []byte) {
		b := &treeBuilder{in: in}
		d, _ := b.value(0).(*model.Data)
		if d == nil {
			d = model.NewData(model.Field{Key: "x", Value: b.value(0)})
		}
		for _, key := range fuzzKeys {
			var c model.Cue
			if c.SetFormatData(key, d) == nil {
				got, ok := c.FormatData(key)
				if ok != (d.Len() > 0) || (ok && !reflect.DeepEqual(d, got)) {
					t.Fatalf("cue %s: stored %#v, read %#v", key, d, got)
				}
			}
			_ = (&model.Subtitle{}).SetFormatData(key, d)
		}
	})
}

type treeBuilder struct {
	in  []byte
	pos int
}

func (b *treeBuilder) next() int {
	if b.pos >= len(b.in) {
		return 0
	}
	b.pos++
	return int(b.in[b.pos-1])
}

func (b *treeBuilder) value(depth int) any {
	const maxDepth = 8
	op := b.next() % 7
	if depth >= maxDepth && op >= 5 {
		op = 0
	}
	switch op {
	case 1:
		return b.next()%2 == 1
	case 2:
		return int64(b.next() - 128)
	case 3:
		return []float64{0.5, math.Inf(1), 1e300, -2}[b.next()%4]
	case 4:
		return fuzzStrings[b.next()%len(fuzzStrings)]
	case 5:
		list := []any{}
		for range b.next() % 4 {
			list = append(list, b.value(depth+1))
		}
		return list
	case 6:
		d := model.NewData()
		for range b.next() % 5 {
			d.Set(fuzzFields[b.next()%len(fuzzFields)], b.value(depth+1))
		}
		return d
	default:
		return nil
	}
}

// FuzzChangeCase runs the 3 case modes with and without the Turkic rules and checks that the tags stay in count
// and order.
func FuzzChangeCase(f *testing.F) {
	f.Add(`<i>STOP</i> tom &amp; <font color="#ff0000">jerry</font> &lt;3`)
	f.Add(">> I'M HERE. ETC. THEN i.e., NOW\u2026 W<b>I</b>NDOW")
	f.Add("caf\xe9 <i>NO\xebL</i> \u0130STANBUL \u00df \u03a3")
	f.Add("- WAIT &nbsp; <00:00:01.500>x <v O'Neil>[BELL]")

	f.Fuzz(func(t *testing.T, line string) {
		for _, mode := range []model.CaseMode{model.CaseUpper, model.CaseLower, model.CaseSentence} {
			for _, language := range []string{"", "tr"} {
				s := &model.Subtitle{}
				s.AddCues(model.Cue{End: time.Second, Lines: []string{line}}, model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{line}})
				before := tagsOf(s)
				if err := s.ChangeCase(mode, language); err != nil {
					t.Fatalf("ChangeCase(%q, %q) of %q: %v", mode, language, line, err)
				}
				if after := tagsOf(s); !slices.Equal(before, after) {
					t.Fatalf("ChangeCase(%q, %q) of %q changed the tags from %q to %q", mode, language, line, before, after)
				}
			}
		}
	})
}

func tagsOf(s *model.Subtitle) []string {
	var tags []string
	for _, c := range s.All() {
		for _, line := range c.Lines {
			for i, part := range markupcore.SplitTags(line) {
				if i%2 == 1 {
					tags = append(tags, part)
				}
			}
		}
	}
	return tags
}
