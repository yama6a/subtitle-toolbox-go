package cli

import (
	"unicode/utf8"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/hearingimpaired"
)

func sdhGroup() group {
	return group{
		name:    "sdh",
		summary: "Remove hearing-impaired annotations.",
		options: []option{
			flagOption("sdh", "Remove hearing-impaired annotations such as [DOOR SLAMS], (laughs) and JOHN:. "+
				"Remove a cue with no text left."),
			flagOption("sdh-keep-square-brackets", "Keep text in square brackets, such as [DOOR SLAMS]."),
			flagOption("sdh-keep-parentheses", "Keep text in parentheses, such as (laughs)."),
			flagOption("sdh-keep-speaker-labels", "Keep speaker labels, such as JOHN:."),
			flagOption("sdh-keep-music-lines", "Keep lines that hold only music symbols."),
			flagOption("sdh-any-case-labels", "Also remove speaker labels that are not upper case, such as Baker:."),
			flagOption("sdh-lyrics", "Remove text between two music symbols."),
			{
				name: "sdh-brackets", value: "PAIR", repeatable: true,
				help: `Also remove text between this pair of characters, for example "{}" or "**". Repeatable.`,
			},
		},
		build: buildSDH,
	}
}

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildSDH(a *args) (edit, error) {
	if err := needs(a, "sdh", "sdh-keep-square-brackets", "sdh-keep-parentheses", "sdh-keep-speaker-labels",
		"sdh-keep-music-lines", "sdh-any-case-labels", "sdh-lyrics", "sdh-brackets"); err != nil {
		return nil, err
	}
	if !a.Has("sdh") {
		return nil, nil //nolint:nilnil // a nil edit means the group is off
	}
	opts := hearingimpaired.Options{
		KeepSquareBrackets: a.Has("sdh-keep-square-brackets"),
		KeepParentheses:    a.Has("sdh-keep-parentheses"),
		KeepSpeakerLabels:  a.Has("sdh-keep-speaker-labels"),
		KeepMusicLines:     a.Has("sdh-keep-music-lines"),
		AnyCaseLabels:      a.Has("sdh-any-case-labels"),
		Lyrics:             a.Has("sdh-lyrics"),
	}
	for _, pair := range a.Values("sdh-brackets") {
		open, n := utf8.DecodeRuneInString(pair)
		if !utf8.ValidString(pair) || utf8.RuneCountInString(pair) != 2 {
			return nil, usage("The option --sdh-brackets needs two characters, an opening and a closing one, got " + quote(pair) + ".")
		}
		opts.CustomBrackets = append(opts.CustomBrackets, hearingimpaired.BracketPair{Open: string(open), Close: pair[n:]})
	}
	return steps{inPlace(func(s *stbox.Subtitle) error {
		_, err := hearingimpaired.Apply(s, opts)
		return err
	})}, nil
}
