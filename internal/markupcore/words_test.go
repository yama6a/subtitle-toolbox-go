package markupcore_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
)

func TestTagWords(t *testing.T) {
	t.Parallel()

	s := time.Second
	type tt = markupcore.TimedText
	tests := []struct {
		name  string
		lines []string
		words []markupcore.TimedText
		voice markupcore.Voice
		want  []string
	}{
		{
			"first word",
			[]string{"The beach"},
			[]tt{{"The", 0}, {"beach", 240 * time.Millisecond}},
			markupcore.Voice{},
			[]string{"<00:00:00.000>The <00:00:00.240>beach"},
		},
		{
			"before an opening tag",
			[]string{"<c> beach</c>"},
			[]tt{{"beach", s}},
			markupcore.Voice{},
			[]string{"<00:00:01.000><c> beach</c>"},
		},
		{
			"after the space before a tag",
			[]string{"at <i>dusk</i>"},
			[]tt{{"dusk", 6500 * time.Millisecond}},
			markupcore.Voice{},
			[]string{"at <00:00:06.500><i>dusk</i>"},
		},
		{
			"after a closing tag",
			[]string{"Hello</i> <i>world"},
			[]tt{{"Hello", s}, {"world", 2 * s}},
			markupcore.Voice{},
			[]string{"<00:00:01.000>Hello</i> <00:00:02.000><i>world"},
		},
		{
			"inside a word",
			[]string{"Hello"},
			[]tt{{"Hel", s}, {"lo", 1400 * time.Millisecond}},
			markupcore.Voice{},
			[]string{"<00:00:01.000>Hel<00:00:01.400>lo"},
		},
		{
			"entity",
			[]string{"Fish &amp; Chips"},
			[]tt{{"&", s}},
			markupcore.Voice{},
			[]string{"Fish <00:00:01.000>&amp; Chips"},
		},
		{
			"2 lines",
			[]string{"Please keep", "your hands"},
			[]tt{{"Please", 0}, {"your", s}},
			markupcore.Voice{},
			[]string{"<00:00:00.000>Please keep", "<00:00:01.000>your hands"},
		},
		{
			"missing word",
			[]string{"Hi there"},
			[]tt{{"Hi", 0}, {"xyz", s}, {"there", 2 * s}},
			markupcore.Voice{},
			[]string{"<00:00:00.000>Hi there"},
		},
		{
			"voice with class and close",
			[]string{"Hi", "there"},
			[]tt{{"Hi", s}},
			markupcore.Voice{Name: "Anna", Class: ".loud", Close: true},
			[]string{"<v.loud Anna><00:00:01.000>Hi", "there</v>"},
		},
		{
			"voice name escaped",
			[]string{"Hi"},
			nil,
			markupcore.Voice{Name: "Ann & Bo"},
			[]string{"<v Ann &amp; Bo>Hi"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.TagWords(tt.lines, tt.words, tt.voice))
		})
	}
}

func TestCuePlainText(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Fish & Chips a b", markupcore.CuePlainText([]string{" <i>Fish &amp;\t Chips</i> ", "", "a  b "}))
}

func TestSearchWords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		text  string
		words []string
		fold  bool
		want  []markupcore.Match
	}{
		{"exact", "a bc a", []string{"bc", "a"}, false, []markupcore.Match{{2, 4}, {5, 6}}},
		{"case differs without fold", "Hello", []string{"hello"}, false, []markupcore.Match{{-1, -1}}},
		{"case folds", "HELLO there", []string{"hello", "THERE"}, true, []markupcore.Match{{0, 5}, {6, 11}}},
		{"Kelvin sign", "Key", []string{"key"}, true, []markupcore.Match{{0, 5}}},
		{
			"missing word keeps the place", "a b",
			[]string{"a", "x", "b", "x"},
			true,
			[]markupcore.Match{{0, 1}, {-1, -1}, {2, 3}, {-1, -1}},
		},
		{"empty word", "a", []string{""}, true, []markupcore.Match{{-1, -1}}},
		{"invalid byte", "a\xffb", []string{"b"}, true, []markupcore.Match{{2, 3}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.SearchWords(tt.text, tt.words, tt.fold))
		})
	}
}

func TestFindWordsStopsAtTheFirstMiss(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []markupcore.Match{{0, 2}}, markupcore.FindWords("Hi there", []string{"Hi", "xyz", "there"}))
}
