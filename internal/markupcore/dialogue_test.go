package markupcore_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

func TestStartsDialogueTurn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want bool
	}{
		{name: "hyphen and space", text: "- Hi", want: true},
		{name: "hyphen without space", text: "-Hi", want: true},
		{name: "unicode hyphen", text: "\u2010 Hi", want: true},
		{name: "en dash", text: "\u2013 Hi", want: true},
		{name: "em dash", text: "\u2014Hi", want: true},
		{name: "tab and no-break space after the dash", text: "-\t\u00a0Hi", want: true},
		{name: "punctuation after the dash", text: "- \"Hi\"", want: true},
		{name: "minus sign before a digit", text: "-20 degrees"},
		{name: "space then a digit", text: "- 20 degrees"},
		{name: "double dash", text: "--"},
		{name: "en dash after a hyphen", text: "-\u2013 Hi"},
		{name: "lone dash", text: "-"},
		{name: "dash and spaces only", text: "-  "},
		{name: "dash before a line break", text: "-\nHi"},
		{name: "dash before an ideographic space", text: "-\u3000Hi"},
		{name: "no dash", text: "Hi - there"},
		{name: "leading space", text: " - Hi"},
		{name: "invalid UTF-8", text: "- Hi \xff"},
		{name: "empty", text: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.StartsDialogueTurn(tt.text))
		})
	}
}

func TestIsDialogueDash(t *testing.T) {
	t.Parallel()

	for _, r := range []rune{'-', '\u2010', '\u2013', '\u2014'} {
		assert.True(t, markupcore.IsDialogueDash(r), "%U", r)
	}
	for _, r := range []rune{'\u2011', '\u2012', '\u2015', '\u2212', '_', '~'} {
		assert.False(t, markupcore.IsDialogueDash(r), "%U", r)
	}
}

func TestSplitLeadingBidiControls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		text               string
		controls, wantRest string
	}{
		{name: "embedding", text: "\u202b- Hi\u202c", controls: "\u202b", wantRest: "- Hi\u202c"},
		{name: "every control", text: "\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069x", controls: "\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069", wantRest: "x"},
		{name: "none", text: "- Hi", wantRest: "- Hi"},
		{name: "control later", text: "a\u200f", wantRest: "a\u200f"},
		{name: "no-break space is no control", text: "\u00a0- Hi", wantRest: "\u00a0- Hi"},
		{name: "invalid UTF-8 has none", text: "\u202b\xff", wantRest: "\u202b\xff"},
		{name: "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			controls, rest := markupcore.SplitLeadingBidiControls(tt.text)
			assert.Equal(t, tt.controls, controls)
			assert.Equal(t, tt.wantRest, rest)
		})
	}
}

func TestRemoveBidiControls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, text, want string
	}{
		{name: "override", text: "Hello \u202edlrow", want: "Hello dlrow"},
		{name: "isolates", text: "\u2066a\u2069 \u2067b\u2069 \u2068c\u2069 d\u202ae\u202d", want: "a b c de"},
		{name: "marks", text: "\u200fx\u200e", want: "x"},
		{name: "no-break space stays", text: "a\u00a0b", want: "a\u00a0b"},
		{name: "invalid UTF-8 keeps its bytes", text: "\xff\u200fa\xe9", want: "\xffa\xe9"},
		{name: "nothing to remove", text: "plain", want: "plain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, markupcore.RemoveBidiControls(tt.text))
		})
	}
}

func TestUpperCaseEnglishI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		runs  []string
		ascii bool
		want  []string
	}{
		{
			name: "pronoun and contractions",
			runs: []string{"well, i'm here and i'll stay. i\u2019ve, i'd, i-i, hi, pi, 3i, i's, i.e."},
			want: []string{"well, I'm here and I'll stay. I\u2019ve, I'd, I-I, hi, pi, 3i, i's, i.e."},
		},
		{name: "start and end", runs: []string{"i"}, want: []string{"I"}},
		{name: "after a letter and a dot", runs: []string{"e.i"}, want: []string{"e.i"}},
		{name: "before a dot and a letter", runs: []string{"i.e"}, want: []string{"i.e"}},
		{name: "before a dot at the end", runs: []string{"so do i."}, want: []string{"so do I."}},
		{name: "after a digit and a dot", runs: []string{"3.i"}, want: []string{"3.I"}},
		{name: "contraction before a letter", runs: []string{"i'mx"}, want: []string{"i'mx"}},
		{name: "upper case contraction stays", runs: []string{"i'M"}, want: []string{"i'M"}},
		{name: "letter of another script", runs: []string{"\u00e9i i\u00e9"}, want: []string{"\u00e9i i\u00e9"}},
		{name: "across runs", runs: []string{"w", "i", "ndow and ", "i"}, want: []string{"w", "i", "ndow and ", "I"}},
		{name: "run boundary inside a contraction", runs: []string{"i", "'ll go"}, want: []string{"I", "'ll go"}},
		{name: "invalid UTF-8 without fallback", runs: []string{"caf\xe9 i think"}, want: []string{"caf\xe9 i think"}},
		{name: "invalid UTF-8 with fallback", runs: []string{"caf\xe9 i think, i'd\xe9"}, ascii: true, want: []string{"caf\xe9 I think, I'd\xe9"}},
		{name: "fallback reads U+2019 as no apostrophe", runs: []string{"\xff i\u2019ve"}, ascii: true, want: []string{"\xff I\u2019ve"}},
		{name: "no runs", runs: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := append([]string(nil), tt.runs...)
			assert.Equal(t, tt.want, markupcore.UpperCaseEnglishI(tt.runs, tt.ascii))
			assert.Equal(t, in, tt.runs, "the input stays unchanged")
		})
	}
}
