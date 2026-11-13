package model_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestCueText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"no lines", nil, ""},
		{"one line", []string{"Hello"}, "Hello"},
		{"two lines", []string{"this is a line", "with a linebreak"}, "this is a line\nwith a linebreak"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, model.Cue{Lines: tt.lines}.Text())
		})
	}
}

func TestCueSetText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{"one line per LF", "this is a line\nwith a linebreak", []string{"this is a line", "with a linebreak"}},
		{
			"CRLF, double spaces and tabs",
			"this is a       line\r\n\r\nwith a \t\t   linebreak",
			[]string{"this is a line", "with a linebreak"},
		},
		{"empty", "", nil},
		{"blank lines only", "\n \n\t", nil},
		{"lone CR stays inside one line", "a\rb", []string{"a\nb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{Lines: []string{"old"}}
			c.SetText(tt.text)
			assert.Equal(t, tt.want, c.Lines)
		})
	}
}

func TestCueAddLine(t *testing.T) {
	t.Parallel()

	var c model.Cue
	c.AddLine("first line")
	c.AddLine("  ")
	c.AddLine("second  \tline ")

	assert.Equal(t, []string{"first line", "second line"}, c.Lines)
}

func TestCueClone(t *testing.T) {
	t.Parallel()

	c := model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"a", "b"}, ID: "intro", Position: model.Position{Alignment: model.TopLeft}, Forced: true}
	clone := c.Clone()
	assert.Equal(t, c, clone)

	clone.Lines[0] = "changed"
	assert.Equal(t, []string{"a", "b"}, c.Lines)
	assert.Nil(t, model.Cue{}.Clone().Lines)

	c.Position.Offset = &model.Offset{X: 50, Y: 10}
	clone = c.Clone()
	clone.Position.Offset.Y = 90
	assert.InDelta(t, 10.0, c.Position.Offset.Y, 0)
}

func word(text string, start, end float64) model.Word {
	return model.Word{Text: text, Start: sec(start), End: sec(end)}
}

func TestExtractWordTags(t *testing.T) {
	t.Parallel()

	const (
		youtube = "the<00:05:04.199><c> train</c><c.colorE5E5E5><00:05:04.379><c> leaves</c></c>"
		hasVTT  = true
	)
	tests := []struct {
		name      string
		in        []string
		vtt       bool
		wantLines []string
		speaker   string
		words     []model.Word
		class     string
		end       bool
		rule      []string // TaggedLines of a cue without the source, nil for the input
	}{
		{name: "tag at the start", in: []string{"<00:00:01.000>Hi there"}, wantLines: []string{"Hi there"}, words: []model.Word{word("Hi there", 1, 5)}},
		{name: "tag in the middle", in: []string{"Hi <00:00:02.000>there"}, wantLines: []string{"Hi there"}, words: []model.Word{word("there", 2, 5)}},
		{name: "inside a tag", in: []string{"<00:00:01.000><c> beach</c>"}, wantLines: []string{"<c> beach</c>"}, words: []model.Word{word("beach", 1, 5)}},
		{name: "syllables", in: []string{"<00:00:01.000>Hel<00:00:01.400>lo"}, wantLines: []string{"Hello"}, words: []model.Word{word("Hel", 1, 1.4), word("lo", 1.4, 5)}},
		{name: "text before the first tag", in: []string{"Well, <00:00:02.000>yes"}, wantLines: []string{"Well, yes"}, words: []model.Word{word("yes", 2, 5)}},
		{name: "whole-cue voice", in: []string{"<v Anna>Good <00:00:01.000>morning."}, wantLines: []string{"Good morning."}, speaker: "Anna", words: []model.Word{word("morning.", 1, 5)}},
		{name: "2 voices", in: []string{"<v Anna>Hi.</v>", "<v Ben>Hello.</v>"}, wantLines: []string{"<v Anna>Hi.</v>", "<v Ben>Hello.</v>"}},
		{name: "empty word", in: []string{"<00:00:01.000>Hi", "<00:00:04.000>"}, wantLines: []string{"Hi"}, words: []model.Word{word("Hi", 1, 4)}, rule: []string{"<00:00:01.000>Hi"}},
		{
			name: "start goes back", in: []string{"<00:00:03.000>a <00:00:02.000>b <00:00:04.000>c"}, wantLines: []string{"a b c"},
			words: []model.Word{word("a b", 3, 3), word("c", 4, 5)}, rule: []string{"<00:00:03.000>a b <00:00:04.000>c"},
		},
		{name: "class with vtt data", in: []string{"<v.loud Anna>Hi</v>"}, vtt: hasVTT, wantLines: []string{"Hi"}, speaker: "Anna", class: ".loud", end: true},
		{name: "class without vtt data", in: []string{"<v.loud Anna>Hi"}, wantLines: []string{"<v.loud Anna>Hi"}},
		{
			name: "one voice twice", in: []string{"<v Guide>Please keep your hands", "<v Guide>inside the boat."},
			wantLines: []string{"Please keep your hands", "inside the boat."}, speaker: "Guide", rule: []string{"<v Guide>Please keep your hands", "inside the boat."},
		},
		{name: "text after the end of the voice", in: []string{"<v Anna>Hi</v> there"}, wantLines: []string{"<v Anna>Hi</v> there"}},
		{
			name: "YouTube line", in: []string{youtube}, wantLines: []string{"the<c> train</c><c.colorE5E5E5><c> leaves</c></c>"},
			words: []model.Word{word("train", 304.199, 304.379), word("leaves", 304.379, 304.379)},
			rule:  []string{"the<00:05:04.199><c> train</c><00:05:04.379><c.colorE5E5E5><c> leaves</c></c>"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{Start: 0, End: 5 * time.Second, Lines: tt.in}
			if tt.vtt {
				require.NoError(t, c.SetFormatData("vtt", model.NewData(model.Field{Key: "align", Value: "start"})))
			}
			c.ExtractWordTags()
			assert.Equal(t, tt.wantLines, c.Lines)
			assert.Equal(t, tt.speaker, c.Speaker)
			assert.Equal(t, tt.words, c.Words)
			assert.Equal(t, tt.in, c.TaggedLines())
			if tt.vtt {
				d, _ := c.FormatData("vtt")
				class, _ := d.String("voiceClass")
				end, _ := d.Bool("voiceEnd")
				assert.Equal(t, tt.class, class)
				assert.Equal(t, tt.end, end)
			}

			fresh := model.Cue{Lines: c.Lines, Speaker: c.Speaker, Words: c.Words}
			if fd, ok := c.FormatData("vtt"); ok {
				require.NoError(t, fresh.SetFormatData("vtt", fd))
			}
			want := tt.rule
			if want == nil {
				want = tt.in
			}
			assert.Equal(t, want, fresh.TaggedLines())
		})
	}
}

func TestTaggedLinesAfterAnEdit(t *testing.T) {
	t.Parallel()

	c := model.Cue{End: 5 * time.Second, Lines: []string{"<00:00:01.000>Hi <00:00:02.000>there"}}
	c.ExtractWordTags()
	c.Words[0].End = 1500 * time.Millisecond
	assert.Equal(t, []string{"<00:00:01.000>Hi <00:00:02.000>there"}, c.TaggedLines(), "an end change keeps the source")

	c.Words[1].Start = 3 * time.Second
	assert.Equal(t, []string{"<00:00:01.000>Hi <00:00:03.000>there"}, c.TaggedLines())
}

func TestPlainText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"tags", []string{"<i>Hi</i> <c.x>there</c>"}, "Hi there"},
		{"entity", []string{"Fish &amp; Chips"}, "Fish & Chips"},
		{"tab and 2 spaces", []string{"a\t b  c"}, "a b c"},
		{"2 lines", []string{"Hi", "there"}, "Hi there"},
		{"white space at the ends", []string{" Hi ", " there "}, "Hi there"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, model.Cue{Lines: tt.lines}.PlainText())
		})
	}
}

func TestValidateWords(t *testing.T) {
	t.Parallel()

	conf := func(v float64) *float64 { return &v }
	lines := []string{"The beach house"}
	tests := []struct {
		name   string
		lines  []string
		words  []model.Word
		option string
		msg    string
	}{
		{"valid", lines, []model.Word{word("The", 0, 1), word("beach", 1, 2)}, "", ""},
		{"empty text", lines, []model.Word{word("The", 0, 1), word("beach", 1, 2), word("", 2, 3)}, "Words[2].Text", "The word text must not be empty."},
		{"white space", lines, []model.Word{word("The", 0, 1), word("house", 1, 2), word(" beach", 2, 3)}, "Words[2].Text", `The word text must not start or end with white space, got " beach".`},
		{"start below 0", lines, []model.Word{word("The", -1, 1)}, "Words[0].Start", "The word start must be 0 s or more and below 100000 hours, got -1 s."},
		{"end too large", lines, []model.Word{word("The", 0, 360000000)}, "Words[0].End", "The word end must be 0 s or more and below 100000 hours, got 360000000 s."},
		{"end before start", lines, []model.Word{word("The", 2, 1)}, "Words[0].End", "The word end 1 s must not be before the word start 2 s."},
		{"start goes back", lines, []model.Word{word("The", 4, 5), word("beach", 3, 6)}, "Words[1].Start", "The word start 3 s must not be before the start 4 s of the word before it."},
		{"confidence", lines, []model.Word{{Text: "The", Confidence: conf(1.5)}}, "Words[0].Confidence", "The confidence must be from 0 to 1, got 1.5."},
		{"not in the text", lines, []model.Word{word("The", 0, 1), word("beach", 1, 2), word("The", 2, 3)}, "Words[2].Text", `The word "The" is not in the text of the cue after the word before it.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := model.Cue{Lines: tt.lines, Words: tt.words}.ValidateWords()
			if tt.msg == "" {
				require.NoError(t, err)
				return
			}
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, tt.option, invalid.Option)
			assert.Equal(t, tt.msg, invalid.Msg)
		})
	}
}

func TestSyncWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		lines       []string
		words       []model.Word
		newLines    []string
		want        []model.Word
		wantDropped int
	}{
		{
			"case change",
			[]string{"Hello there"},
			[]model.Word{word("Hello", 0, 1), word("there", 1, 2)},
			[]string{"HELLO THERE"},
			[]model.Word{word("HELLO", 0, 1), word("THERE", 1, 2)},
			0,
		},
		{
			"sentence case",
			[]string{"hello. how are you"},
			[]model.Word{word("hello.", 0, 1), word("how", 1, 2), word("you", 2, 3)},
			[]string{"Hello. How are you"},
			[]model.Word{word("Hello.", 0, 1), word("How", 1, 2), word("you", 2, 3)},
			0,
		},
		{
			"wrap",
			[]string{"one two three"},
			[]model.Word{word("one", 0, 1), word("three", 2, 3)},
			[]string{"one two", "three"},
			[]model.Word{word("one", 0, 1), word("three", 2, 3)},
			0,
		},
		{
			"unwrap",
			[]string{"one two", "three"},
			[]model.Word{word("one", 0, 1), word("three", 2, 3)},
			[]string{"one two three"},
			[]model.Word{word("one", 0, 1), word("three", 2, 3)},
			0,
		},
		{
			"masked word",
			[]string{"damn it"},
			[]model.Word{word("damn", 0, 1), word("it", 1, 2)},
			[]string{"d*** it"},
			[]model.Word{word("it", 1, 2)},
			1,
		},
		{"replacement across 2 words", []string{"New York"}, []model.Word{word("New", 0, 1), word("York", 1, 2)}, []string{"NYC"}, nil, 2},
		{
			"removed word",
			[]string{"Hi there you"},
			[]model.Word{word("Hi", 0, 1), word("there", 1, 2), word("you", 2, 3)},
			[]string{"Hi you"},
			[]model.Word{word("Hi", 0, 1), word("you", 2, 3)},
			1,
		},
		{
			"unchanged text",
			[]string{"Hi there"},
			[]model.Word{word("Hi", 0, 1), word("there", 1, 2)},
			[]string{"Hi there"},
			[]model.Word{word("Hi", 0, 1), word("there", 1, 2)},
			0,
		},
		{"Kelvin sign", []string{"Key"}, []model.Word{word("Key", 0, 1)}, []string{"Key"}, []model.Word{word("Key", 0, 1)}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := model.Cue{Lines: tt.lines, Words: tt.words}
			require.NoError(t, c.ValidateWords())
			c.Lines = tt.newLines
			assert.Equal(t, tt.wantDropped, c.SyncWords())
			assert.Equal(t, tt.want, c.Words)
		})
	}
}

func TestCueCloneWords(t *testing.T) {
	t.Parallel()

	v := 0.5
	c := model.Cue{Lines: []string{"a"}, Words: []model.Word{{Text: "a", Confidence: &v}}}
	clone := c.Clone()
	clone.Words[0].Text = "b"
	*clone.Words[0].Confidence = 0.9
	assert.Equal(t, "a", c.Words[0].Text)
	assert.InDelta(t, 0.5, *c.Words[0].Confidence, 0)
}
