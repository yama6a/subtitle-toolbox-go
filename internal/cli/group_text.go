package cli

import (
	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
	"github.com/yama6a/subtitle-toolbox-go/v5/speakers"
)

func textGroup() group {
	return group{
		name:    "text",
		summary: "Convert speaker labels, change the case, remove tags, fix right-to-left text.",
		options: []option{
			valueOption("speakers", "MODE", "Convert <v> speaker tags: prefix (ANNA: Hi), dashes, colors, or from-prefix (ANNA: to <v Anna>)."),
			valueOption("case", "MODE", "Change the case of the text between tags: upper, lower or sentence."),
			flagOption("strip-tags", "Remove all formatting tags, such as <i> and <font>, from the cue text."),
			valueOption("rtl", "MODE", "Right-to-left text: fix wraps each Arabic or Hebrew line in Unicode embedding "+
				"marks, clean removes all bidi controls."),
		},
		build: buildText,
	}
}

//nolint:gochecknoglobals // lookup table of the --speakers values
var speakerOptions = map[string]speakers.Options{
	"prefix":      {Style: speakers.StylePrefix},
	"dashes":      {Style: speakers.StyleDialogueDashes},
	"colors":      {Style: speakers.StyleColors},
	"from-prefix": {FromLabels: true},
}

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildText(a *args) (edit, error) {
	mode, hasCase, err := a.Choice("case", string(model.CaseUpper), string(model.CaseLower), string(model.CaseSentence))
	if err != nil {
		return nil, err
	}
	speakerMode, hasSpeakers, err := a.Choice("speakers", "prefix", "dashes", "colors", "from-prefix")
	if err != nil {
		return nil, err
	}
	rtl, _, err := a.Choice("rtl", "fix", "clean")
	if err != nil {
		return nil, err
	}
	var st steps
	if hasSpeakers {
		opts := speakerOptions[speakerMode]
		st = append(st, inPlace(func(s *stbox.Subtitle) error {
			_, err := speakers.Apply(s, opts)
			return err
		}))
	}
	if hasCase {
		language, _ := a.Value("language")
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.ChangeCase(model.CaseMode(mode), language) }))
	}
	if a.Has("strip-tags") {
		st = append(st, inPlace(func(s *stbox.Subtitle) error { s.StripFormatting(model.StripOptions{}); return nil }))
	}
	switch rtl {
	case "fix":
		st = append(st, inPlace(func(s *stbox.Subtitle) error { s.FixRightToLeft(); return nil }))
	case "clean":
		st = append(st, inPlace(func(s *stbox.Subtitle) error { s.RemoveBidiControls(); return nil }))
	}
	return stepsEdit(st), nil
}
