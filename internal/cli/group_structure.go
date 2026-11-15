package cli

import (
	"cmp"
	"strconv"
	"time"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
	"github.com/yama6a/subtitle-toolbox-go/v5/edit/resegment"
)

func structureGroup() group {
	return group{
		name:    "structure",
		summary: "Rebuild, join, split and wrap cues.",
		options: []option{
			flagOption("structure-resegment", "Build new cues from the word timestamps. Each cue holds one sentence, or what fits "+
				"--structure-max-cpl and --structure-max-lines."),
			valueOption("structure-max-word-gap", "SECONDS", "--structure-resegment ends a cue at a pause of this length. Default: "+
				timecode.Seconds(resegment.DefaultMaxWordGap)+"."),
			flagOption("structure-unwrap",
				"Join the lines of each cue with a space. Each dialogue dash line starts a line of its own."),
			flagOption("structure-merge-short", "Join cues shorter than "+timecode.Seconds(model.DefaultMinDuration)+
				" s with a neighbor at most "+timecode.Seconds(model.DefaultMergeMaxGap)+" s away. The joined cue must fit "+
				timecode.Seconds(model.DefaultMaxDuration)+" s, --structure-max-cpl and --structure-max-lines."),
			flagOption("structure-split-long", "Split cues longer than "+timecode.Seconds(model.DefaultMaxDuration)+
				" s, or longer than --structure-max-lines lines of --structure-max-cpl characters. "+
				"Split at sentence ends, clause ends or spaces."),
			flagOption("structure-wrap", "Break lines longer than --structure-max-cpl characters."),
			valueOption("structure-max-cpl", "CHARS", "Maximum characters per line for --structure-wrap, "+
				"--structure-resegment, --structure-merge-short and --structure-split-long. Default: "+strconv.Itoa(model.DefaultMaxCharactersPerLine)+"."),
			valueOption("structure-max-lines", "LINES", "Maximum number of lines per cue for --structure-wrap, "+
				"--structure-resegment, --structure-merge-short and --structure-split-long. Default: "+strconv.Itoa(model.DefaultMaxLinesPerCue)+"."),
			flagOption("structure-merge-duplicates", "Join same-text cues that overlap or touch."),
			flagOption("structure-merge-same-time",
				"Join cues with the same start and end into one cue with the lines of all of them."),
		},
		build: buildStructure,
	}
}

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildStructure(a *args) (edit, error) {
	if err := needs(a, "structure-resegment", "structure-max-word-gap"); err != nil {
		return nil, err
	}
	limited := []string{"structure-wrap", "structure-resegment", "structure-merge-short", "structure-split-long"}
	for _, opt := range []string{"structure-max-cpl", "structure-max-lines"} {
		if err := needsOneOf(a, limited, opt); err != nil {
			return nil, err
		}
	}
	gap, hasGap, err := a.PositiveTime("structure-max-word-gap")
	if err != nil {
		return nil, err
	}
	cpl, _, err := a.PositiveInt("structure-max-cpl")
	if err != nil {
		return nil, err
	}
	lines, _, err := a.PositiveInt("structure-max-lines")
	if err != nil {
		return nil, err
	}
	// 0 for an option that the call does not set makes the library default apply.
	limits := model.CueLimits{MaxCharactersPerLine: int(cpl), MaxLinesPerCue: int(lines)}
	wrapCPL := cmp.Or(int(cpl), model.DefaultMaxCharactersPerLine)
	wrapLines := cmp.Or(int(lines), model.DefaultMaxLinesPerCue)
	var wordGap *time.Duration
	if hasGap {
		wordGap = &gap
	}
	// The run order of docs/cli.md, which is not the order of the help.
	return stepsEdit(flagSteps(a, []flagStep{
		{"structure-resegment", func(s *stbox.Subtitle) error {
			_, err := resegment.ByWords(s, resegment.Options{Limits: limits, MaxWordGap: wordGap})
			return err
		}},
		{"structure-unwrap", func(s *stbox.Subtitle) error { s.UnwrapLines(); return nil }},
		{"structure-merge-same-time", func(s *stbox.Subtitle) error { return s.MergeSameTimeCues(0) }},
		{"structure-merge-short", func(s *stbox.Subtitle) error {
			return s.MergeShortCues(model.MergeShortCuesOptions{Limits: limits})
		}},
		{"structure-split-long", func(s *stbox.Subtitle) error {
			_, err := resegment.SplitLong(s, resegment.Options{Limits: limits})
			return err
		}},
		{"structure-wrap", func(s *stbox.Subtitle) error { return s.WrapLines(wrapCPL, wrapLines) }},
		{"structure-merge-duplicates", func(s *stbox.Subtitle) error { return s.RemoveDuplicateCues(0) }},
	})), nil
}
