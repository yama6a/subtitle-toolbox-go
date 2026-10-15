package webvtt_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports tests/Parsers/LenientWebVttTest.php of subtitle-toolbox-php. The 13 WebVTT fixtures of testdata/lenient
// run in lenient_test.go of the root package.
func TestReadLenient(t *testing.T) {
	t.Parallel()

	t.Run("a damaged signature keeps the skipped lines in the warning", func(t *testing.T) {
		t.Parallel()

		s, warnings := parse(t, fixture(t, filepath.Join(lenDir, "damaged_signature.vtt")), true)
		assert.Equal(t, []string{"WEBVTS", "Kind: captions"}, warnings[0].Lines)
		assert.Equal(t, []model.Comment{{Text: "closing time", BeforeCue: 1}}, s.Comments())
		assert.Equal(t, vttData(model.Field{Key: "line", Value: "0"}, model.Field{Key: "alignment", Value: int64(8)}), cueData(t, firstCue(t, s)))
	})
	t.Run("a missing signature keeps the cue identifiers", func(t *testing.T) {
		t.Parallel()

		s, _ := parse(t, fixture(t, filepath.Join(lenDir, "missing_signature.vtt")), true)
		var ids []string
		for _, c := range s.All() {
			ids = append(ids, c.ID)
		}
		assert.Equal(t, []string{"1", "2"}, ids)
	})
	t.Run("line numbers count the empty lines before the signature", func(t *testing.T) {
		t.Parallel()

		_, warnings := parse(t, "\n\nWEBVTT\n\nbroken\n\n00:00:01.000 --> 00:00:02.000\ntext\n", true)
		assert.Equal(t, []model.ParseWarning{{
			Message: `Block #1 is not a WebVTT cue, comment, style or region. The line is "broken".`,
			Line:    5, Block: 1, Lines: []string{"broken"}, Action: model.Skipped,
		}}, warnings)
	})
	t.Run("a NOTE after an empty line stays apart from the cue", func(t *testing.T) {
		t.Parallel()

		s, warnings := parse(t, "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n\nThe ferry leaves.\n\nNOTE checked\n\nSTYLE\n::cue { color: red }\n", true)
		assert.Equal(t, []string{"The ferry leaves."}, cueTexts(s))
		assert.Equal(t, "checked", s.Comments()[0].Text)
		assert.Equal(t, []model.ParseWarning{
			{
				Message: "Block #1 has an empty line before line 5 inside the cue. The parser kept the text after it in the cue.",
				Line:    5, Block: 1, Lines: []string{"The ferry leaves."}, Action: model.Repaired,
			},
			{
				Message: "Block #3 is a STYLE block after a cue. The spec allows it only before the first cue, so the parser skipped it.",
				Line:    9, Block: 3, Lines: []string{"STYLE", "::cue { color: red }"}, Action: model.Skipped,
			},
		}, warnings)
	})
	t.Run("the header lines before the split stay", func(t *testing.T) {
		t.Parallel()

		s, _ := parse(t, fixture(t, filepath.Join(lenDir, "missing_empty_line.vtt")), true)
		d, _ := s.FormatData("vtt")
		assert.Equal(t, vttData(model.Field{Key: "headerLines", Value: []any{"Kind: captions", "Language: en"}}), d)
	})
	t.Run("joined files skip the second header and the STYLE block", func(t *testing.T) {
		t.Parallel()

		s, warnings := parse(t, fixture(t, filepath.Join(lenDir, "joined_files.vtt")), true)
		assert.Equal(t, []string{"One", "Two"}, cueTexts(s))
		assert.Len(t, warnings, 2)
		assert.Equal(t, []string{"WEBVTT", "Kind: captions"}, warnings[0].Lines)
		assert.Equal(t, []string{"STYLE", "::cue { color: red }"}, warnings[1].Lines)
	})
	t.Run("joined files fail in strict mode", func(t *testing.T) {
		t.Parallel()

		perr := parseError(t, fixture(t, filepath.Join(lenDir, "joined_files.vtt")), false)
		assert.Equal(t, `Block #2 is not a WebVTT cue, comment, style or region. The line is "WEBVTT".`, perr.Msg)
		assert.Equal(t, 6, perr.Line)
	})
}

func TestRepairSignature(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		texts   []string
		ids     []string
		warning model.ParseWarning
	}{
		{
			name:    "no signature and no text before the cue",
			content: "00:00:01.000 --> 00:00:02.000\nHi\n",
			texts:   []string{"Hi"}, ids: []string{""},
			warning: model.ParseWarning{Message: noWebVTT + " The parser read the cues without it.", Line: 1, Block: 0, Action: model.Repaired},
		},
		{
			name:    "the line above the timing line is the identifier",
			content: "\n\n  \nhello\n1\n00:00:01.000 --> 00:00:02.000 align:left\nHi\n",
			texts:   []string{"Hi"}, ids: []string{"1"},
			warning: model.ParseWarning{Message: noWebVTT + " The parser skipped the lines before line 5.", Line: 4, Block: 0, Lines: []string{"hello"}, Action: model.Repaired},
		},
		{
			name:    "a signature after text",
			content: "converted by a tool\n\nWEBVTT Title\n\n00:01.000 --> 00:02.000\nHi\n",
			texts:   []string{"Hi"}, ids: []string{""},
			warning: model.ParseWarning{Message: noWebVTT + " The parser skipped the lines before line 3.", Line: 1, Block: 0, Lines: []string{"converted by a tool"}, Action: model.Repaired},
		},
		{
			name:    "WEBVTTX is no signature",
			content: "WEBVTTX\n\n00:01.000 --> 00:02.000\nHi\n",
			texts:   []string{"Hi"}, ids: []string{""},
			warning: model.ParseWarning{Message: noWebVTT + " The parser skipped the lines before line 3.", Line: 1, Block: 0, Lines: []string{"WEBVTTX"}, Action: model.Repaired},
		},
		{
			name:    "settings after the timing line",
			content: "00:01.000-->00:02.000 line:0\nHi\n",
			texts:   []string{"Hi"}, ids: []string{""},
			warning: model.ParseWarning{Message: noWebVTT + " The parser read the cues without it.", Line: 1, Block: 0, Action: model.Repaired},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := parse(t, tt.content, true)
			assert.Equal(t, tt.texts, cueTexts(s))
			var ids []string
			for _, c := range s.All() {
				ids = append(ids, c.ID)
			}
			assert.Equal(t, tt.ids, ids)
			assert.Equal(t, []model.ParseWarning{tt.warning}, warnings)
		})
	}
}

func TestReadLenientSwap(t *testing.T) {
	t.Parallel()

	s, warnings := parse(t, "WEBVTT\n\n00:05.000 --> 00:01.000\nswapped\n\n00:45.000 --> 00:01.000\ntoo long\n", true)
	assert.Equal(t, []string{"swapped"}, cueTexts(s))
	assert.Equal(t, []model.ParseWarning{
		{
			Message: "The cue ends at 1 s, before it starts at 5 s. The parser swapped the times.",
			Line:    3, Block: 1, Lines: []string{"00:05.000 --> 00:01.000", "swapped"}, Action: model.Repaired,
		},
		{
			Message: "The cue ends at 1 s, before it starts at 45 s.",
			Line:    6, Block: 2, Lines: []string{"00:45.000 --> 00:01.000", "too long"}, Action: model.Skipped,
		},
	}, warnings)
}
