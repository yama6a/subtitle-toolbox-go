// Package validation checks a subtitle against reading speed, line length, timing and text rules.
package validation

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/dlclark/regexp2/v2"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// Rule names one check. The value is the name in the CLI output and the JSON output.
type Rule string

// The 19 rules of Check and the 2 chapter rules of CheckYouTubeChapters. The comment of each constant names the unit of the value and the limit of its violations.
const (
	// RuleMaxCharactersPerSecond reports a cue that reads faster than the limit, in characters per second.
	RuleMaxCharactersPerSecond Rule = "maxCharactersPerSecond"
	// RuleMaxCharactersPerLine reports a line that is longer than the limit, in visible characters.
	RuleMaxCharactersPerLine Rule = "maxCharactersPerLine"
	// RuleMaxLinesPerCue reports a cue with more lines than the limit. The value is the line count.
	RuleMaxLinesPerCue Rule = "maxLinesPerCue"
	// RuleMinDuration reports a cue that is shorter than the limit. Value and limit are in seconds.
	RuleMinDuration Rule = "minDuration"
	// RuleMaxDuration reports a cue that is longer than the limit. Value and limit are in seconds.
	RuleMaxDuration Rule = "maxDuration"
	// RuleMinGap reports a gap that is shorter than the limit, but not negative. Value and limit are in seconds.
	RuleMinGap Rule = "minGap"
	// RuleNoOverlap reports a cue that starts before the latest end of the earlier cues. The value is the overlap in seconds.
	RuleNoOverlap Rule = "noOverlap"
	// RuleNoEmptyCues reports a cue without visible characters. The value is 0.
	RuleNoEmptyCues Rule = "noEmptyCues"
	// RuleNoDoubleSpaces reports runs of 2 or more spaces between words. The value is the number of runs.
	RuleNoDoubleSpaces Rule = "noDoubleSpaces"
	// RuleNoLeadingOrTrailingSpaces reports lines that start or end with a space. The value is the number of lines.
	RuleNoLeadingOrTrailingSpaces Rule = "noLeadingOrTrailingSpaces"
	// RuleNoUnbalancedTags reports tags without a partner tag. The value is the number of tags.
	RuleNoUnbalancedTags Rule = "noUnbalancedTags"
	// RuleDialogueDashStyle reports dialogue dashes in another style. The value is the number of lines.
	RuleDialogueDashStyle Rule = "dialogueDashStyle"
	// RuleMaxSpeakersPerCue reports a cue with more speakers than the limit. The value is the speaker count.
	RuleMaxSpeakersPerCue Rule = "maxSpeakersPerCue"
	// RuleMaxWordsPerMinute reports a cue that reads faster than the limit, in words per minute.
	RuleMaxWordsPerMinute Rule = "maxWordsPerMinute"
	// RuleMinDurationPerWord reports a cue with less time per word than the limit. Value and limit are in seconds per word.
	RuleMinDurationPerWord Rule = "minSecondsPerWord"
	// RuleAllowedCharacters reports characters that are not allowed. The value is the number of characters.
	RuleAllowedCharacters Rule = "allowedCharacters"
	// RuleNoAllCapsLines reports lines in upper case only. The value is the number of lines.
	RuleNoAllCapsLines Rule = "noAllCapsLines"
	// RuleRequireCues reports a subtitle without cues. The value is 0.
	RuleRequireCues Rule = "requireCues"
	// RuleNoNegativeDuration reports a cue that ends before it starts. The value is the negative duration in seconds.
	RuleNoNegativeDuration Rule = "noNegativeDuration"
	// RuleMinChapters reports a chapter list with fewer chapters than the limit. CheckYouTubeChapters reports it.
	// Rules has no field for it.
	RuleMinChapters Rule = "minChapters"
	// RuleFirstChapterAtZero reports a chapter list whose first chapter starts after 0 s. CheckYouTubeChapters
	// reports it. Rules has no field for it.
	RuleFirstChapterAtZero Rule = "firstChapterAtZero"
)

// String returns the name of the rule.
func (r Rule) String() string { return string(r) }

// Rules selects the rules that Check applies. The zero value checks nothing.
type Rules struct {
	// MaxCharactersPerSecond is the limit of visible characters per second. 0 turns the rule off, +Inf checks nothing.
	MaxCharactersPerSecond float64
	// MaxCharactersPerLine is the limit of visible characters of one line. 0 turns the rule off.
	MaxCharactersPerLine int
	// MaxLinesPerCue is the limit of lines with visible text. 0 turns the rule off.
	MaxLinesPerCue int
	// MinDuration is the least duration of a cue. 0 turns the rule off.
	MinDuration time.Duration
	// MaxDuration is the greatest duration of a cue. 0 turns the rule off.
	MaxDuration time.Duration
	// MinGap is the least gap to the latest end of the earlier cues. 0 turns the rule off.
	MinGap time.Duration
	// NoOverlap reports a cue that starts before the latest end of the earlier cues.
	NoOverlap bool
	// NoEmptyCues reports a cue without visible characters.
	NoEmptyCues bool
	// NoDoubleSpaces reports 2 or more spaces between words.
	NoDoubleSpaces bool
	// NoLeadingOrTrailingSpaces reports a line that starts or ends with a space.
	NoLeadingOrTrailingSpaces bool
	// NoUnbalancedTags reports a formatting tag without a partner tag.
	NoUnbalancedTags bool
	// DialogueDashStyle is the only dash style a dialogue line may use. nil turns the rule off.
	DialogueDashStyle *model.DialogueDashStyle
	// MaxSpeakersPerCue is the limit of speakers of one cue. 0 turns the rule off.
	MaxSpeakersPerCue int
	// MaxWordsPerMinute is the limit of words per minute. 0 turns the rule off, +Inf checks nothing.
	MaxWordsPerMinute float64
	// MinDurationPerWord is the least duration per word of a cue. 0 turns the rule off.
	MinDurationPerWord time.Duration
	// AllowedCharacters is a list such as "Cafab" or a class such as "[A-Za-z0-9 .,!?]". "" turns the rule off.
	AllowedCharacters string
	// NoAllCapsLines reports a line in upper case only.
	NoAllCapsLines bool
	// RequireCues reports a subtitle without cues.
	RequireCues bool
	// NoNegativeDuration reports a cue that ends before it starts.
	NoNegativeDuration bool
}

// matchTimeout limits each match of the allowed character class, because regexp2 can backtrack without end.
const matchTimeout = time.Second

// Validate returns an error that matches model.ErrInvalidArgument when a limit is negative or NaN, the dialogue
// dash style is unknown, or the allowed characters are a class that regexp2 rejects.
func (r Rules) Validate() error {
	_, err := r.prepare()
	return err
}

// prepare checks the rules and compiles the class of AllowedCharacters. The result is nil for a list.
func (r Rules) prepare() (*regexp2.Regexp, error) {
	for _, f := range [...]struct {
		rule  Rule
		field string
		value float64
	}{
		{RuleMaxCharactersPerSecond, "MaxCharactersPerSecond", r.MaxCharactersPerSecond},
		{RuleMaxCharactersPerLine, "MaxCharactersPerLine", float64(r.MaxCharactersPerLine)},
		{RuleMaxLinesPerCue, "MaxLinesPerCue", float64(r.MaxLinesPerCue)},
		{RuleMaxSpeakersPerCue, "MaxSpeakersPerCue", float64(r.MaxSpeakersPerCue)},
		{RuleMaxWordsPerMinute, "MaxWordsPerMinute", r.MaxWordsPerMinute},
	} {
		if f.value < 0 || math.IsNaN(f.value) {
			return nil, limitError(f.rule, f.field, timecode.Number(f.value))
		}
	}
	for _, f := range [...]struct {
		rule  Rule
		field string
		value time.Duration
	}{
		{RuleMinDuration, "MinDuration", r.MinDuration},
		{RuleMaxDuration, "MaxDuration", r.MaxDuration},
		{RuleMinGap, "MinGap", r.MinGap},
		{RuleMinDurationPerWord, "MinDurationPerWord", r.MinDurationPerWord},
	} {
		if f.value < 0 {
			return nil, limitError(f.rule, f.field, timecode.Seconds(f.value))
		}
	}
	if s := r.DialogueDashStyle; s != nil && s.Text() == "" {
		return nil, &model.InvalidArgumentError{
			Option: "Rules.DialogueDashStyle",
			Msg:    "The dialogue dash style " + strconv.Itoa(int(*s)) + " is not one of the 8 styles.",
		}
	}
	if !isClass(r.AllowedCharacters) {
		return nil, nil //nolint:nilnil // nil means a list, which needs no pattern
	}
	return compileClass(r.AllowedCharacters)
}

func limitError(rule Rule, field, got string) error {
	return &model.InvalidArgumentError{
		Option: "Rules." + field,
		Msg:    "The limit " + string(rule) + " must be 0 or more, got " + got + ".",
	}
}

func isClass(allowed string) bool {
	return len(allowed) >= 2 && strings.HasPrefix(allowed, "[") && strings.HasSuffix(allowed, "]")
}

func compileClass(class string) (*regexp2.Regexp, error) {
	if strings.Contains(class, "[:") {
		return nil, &model.InvalidArgumentError{
			Option: "Rules.AllowedCharacters",
			Msg: `The allowed characters "` + class + `" hold a POSIX class such as [:alpha:]. ` +
				`Use a Unicode class such as \p{L}.`,
		}
	}
	re, err := regexp2.Compile("^(?:"+class+")$", regexp2.None)
	if err != nil {
		reason, _, _ := strings.Cut(strings.TrimPrefix(err.Error(), "error parsing regexp: "), " in `")
		return nil, &model.InvalidArgumentError{
			Option: "Rules.AllowedCharacters",
			Msg: `The allowed characters "` + class + `" are not a valid character class: ` +
				strings.TrimSuffix(reason, ".") + ".",
		}
	}
	re.MatchTimeout = matchTimeout
	return re, nil
}
