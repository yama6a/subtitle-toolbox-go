// Package fixing fixes common errors in cue text: spacing, punctuation, dialogue dashes, unbalanced tags and the
// OCR errors of image subtitles, such as lt's for It's. docs/text.md has the rules.
package fixing

import (
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Rule names one fix. The value is the CLI name.
type Rule string

// The rules, in run order.
const (
	// RuleAdCues removes a cue whose visible text names a subtitle download site. Off by default.
	RuleAdCues Rule = "adCues"
	// RuleUnbalancedTags removes closing style tags without an opening tag, and closes style tags still open at
	// the end of the cue.
	RuleUnbalancedTags Rule = "unbalancedTags"
	// RuleEmptyTags removes style tag pairs that hold only white space.
	RuleEmptyTags Rule = "emptyTags"
	// RuleDoubleApostrophes writes 2 apostrophes as a double quote. Off by default.
	RuleDoubleApostrophes Rule = "doubleApostrophes"
	// RuleOCRPipe reads a pipe as I or l.
	RuleOCRPipe Rule = "ocrPipe"
	// RuleOCRZeroInWords reads a 0 in a word of letters as o or O.
	RuleOCRZeroInWords Rule = "ocrZeroInWords"
	// RuleOCRLowercaseL reads an l that stands for I as I.
	RuleOCRLowercaseL Rule = "ocrLowercaseL"
	// RuleLoneLowercaseI writes the English pronoun i as I. Off by default, and English only.
	RuleLoneLowercaseI Rule = "loneLowercaseI"
	// RuleEllipsis writes ". . ." and "...." in the style of Options.Ellipsis.
	RuleEllipsis Rule = "ellipsis"
	// RuleUnneededPeriods removes a period after a question mark or an exclamation mark. Off by default.
	RuleUnneededPeriods Rule = "unneededPeriods"
	// RuleDoubleSpaces writes 2 or more spaces as 1.
	RuleDoubleSpaces Rule = "doubleSpaces"
	// RuleSpaceBeforePunctuation removes spaces before punctuation.
	RuleSpaceBeforePunctuation Rule = "spaceBeforePunctuation"
	// RuleMissingSpaceAfterPunctuation adds a space after punctuation between words.
	RuleMissingSpaceAfterPunctuation Rule = "missingSpaceAfterPunctuation"
	// RuleDialogueOnOneLine splits "- Hi. - Hello." into 2 dialogue lines. Off by default.
	RuleDialogueOnOneLine Rule = "dialogueOnOneLine"
	// RuleDialogueDashes writes each dialogue dash in the style of Options.DialogueDashStyle.
	RuleDialogueDashes Rule = "dialogueDashes"
	// RuleMusicNotes writes a # or * at a line edge as U+266A. Off by default.
	RuleMusicNotes Rule = "musicNotes"
	// RuleSentenceStartCase writes the first letter of a line that starts a sentence in upper case. Off by default.
	RuleSentenceStartCase Rule = "sentenceStartCase"
)

// String returns the CLI name of r, such as "ocrLowercaseL".
func (r Rule) String() string {
	return string(r)
}

// Rules returns every rule in run order.
func Rules() []Rule {
	return []Rule{
		RuleAdCues, RuleUnbalancedTags, RuleEmptyTags, RuleDoubleApostrophes, RuleOCRPipe, RuleOCRZeroInWords,
		RuleOCRLowercaseL, RuleLoneLowercaseI, RuleEllipsis, RuleUnneededPeriods, RuleDoubleSpaces,
		RuleSpaceBeforePunctuation, RuleMissingSpaceAfterPunctuation, RuleDialogueOnOneLine, RuleDialogueDashes,
		RuleMusicNotes, RuleSentenceStartCase,
	}
}

// OptionalRules returns the rules that are off by default, in run order.
func OptionalRules() []Rule {
	var out []Rule
	for _, r := range Rules() {
		if !r.Default() {
			out = append(out, r)
		}
	}
	return out
}

// Default reports whether r runs without Options.Enable. It is false for a name that is no rule.
func (r Rule) Default() bool {
	switch r {
	case RuleUnbalancedTags, RuleEmptyTags, RuleOCRPipe, RuleOCRZeroInWords, RuleOCRLowercaseL, RuleEllipsis,
		RuleDoubleSpaces, RuleSpaceBeforePunctuation, RuleMissingSpaceAfterPunctuation, RuleDialogueDashes:
		return true
	case RuleAdCues, RuleDoubleApostrophes, RuleLoneLowercaseI, RuleUnneededPeriods, RuleDialogueOnOneLine,
		RuleMusicNotes, RuleSentenceStartCase:
	}
	return false
}

// EllipsisStyle is the ellipsis that the ellipsis rule writes.
type EllipsisStyle uint8

// The ellipsis styles. EllipsisThreeDots is the zero value.
const (
	// EllipsisThreeDots writes "...".
	EllipsisThreeDots EllipsisStyle = iota
	// EllipsisUnicode writes U+2026. It also replaces "...".
	EllipsisUnicode
)

// String returns the constant name, such as "EllipsisThreeDots".
func (e EllipsisStyle) String() string {
	switch e {
	case EllipsisThreeDots:
		return "EllipsisThreeDots"
	case EllipsisUnicode:
		return "EllipsisUnicode"
	}
	return "EllipsisStyle(" + strconv.Itoa(int(e)) + ")"
}

// Options holds the settings of Apply and Preview. The zero value runs the default rules with the language
// metadata of the subtitle.
type Options struct {
	// Language is a code such as "en", "de-AT" or "fra". The empty string takes the language metadata.
	Language string
	// Enable lists rules that are off by default to run too. Nil runs the default rules only.
	Enable []Rule
	// Disable lists rules that are on by default to skip. Nil skips none.
	Disable []Rule
	// DialogueDashStyle is the dash that the dialogueDashes and dialogueOnOneLine rules write. The zero value is
	// a hyphen-minus and a space.
	DialogueDashStyle model.DialogueDashStyle
	// Ellipsis is the ellipsis that the ellipsis rule writes. The zero value writes "...".
	Ellipsis EllipsisStyle
}

// Validate returns an error that matches model.ErrInvalidArgument for an unknown rule name, a rule in both
// Enable and Disable, or a style outside its constants.
func (o Options) Validate() error {
	for _, list := range []struct {
		name  string
		rules []Rule
	}{{"Enable", o.Enable}, {"Disable", o.Disable}} {
		for _, r := range list.rules {
			if !slices.Contains(Rules(), r) {
				return invalid(list.name, `Unknown rule "`+string(r)+`".`)
			}
		}
	}
	for _, r := range o.Enable {
		if slices.Contains(o.Disable, r) {
			return invalid("Enable", "The rule "+string(r)+" is in both Enable and Disable.")
		}
	}
	if o.DialogueDashStyle.Text() == "" {
		return invalid("DialogueDashStyle", "The style "+o.DialogueDashStyle.String()+" is no dialogue dash style.")
	}
	if o.Ellipsis > EllipsisUnicode {
		return invalid("Ellipsis", "The style "+o.Ellipsis.String()+" is not EllipsisThreeDots or EllipsisUnicode.")
	}
	return nil
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "fixing.Options." + field, Msg: msg}
}

// Fix is one change of one rule on one cue.
type Fix struct {
	// CueIndex is the index of the cue before any cue removal.
	CueIndex int
	// Rule is the rule that made the change.
	Rule Rule
	// Before is the lines before the change, joined with LF.
	Before string
	// After is the lines after the change, joined with LF.
	After string
}

// Report lists the fixes in cue order, then in rule order.
type Report struct {
	// Fixes holds one entry for each rule that changed a cue.
	Fixes []Fix
}

// Apply runs the rules that opts turns on over each cue of s, in run order, and returns every change. A cue left
// without visible text goes, and its comments move to the next cue. Apply returns the error of Validate and
// changes nothing when opts is not valid.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	return run(s, opts, true)
}

// Preview returns the report that Apply would return, and changes nothing.
func Preview(s *model.Subtitle, opts Options) (Report, error) {
	return run(s, opts, false)
}

func run(s *model.Subtitle, opts Options, change bool) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	f := newFixer(s, opts)
	var ends []bool
	if slices.Contains(f.rules, RuleSentenceStartCase) {
		for _, c := range s.All() {
			ends = append(ends, endsSentence(markupcore.VisibleText(c.Text())))
		}
	}
	edit := func(i int, c model.Cue) ([]string, bool) {
		if len(c.Lines) == 0 {
			return nil, false
		}
		f.startsSentence = i == 0 || ends != nil && ends[i-1]
		lines := f.fixCue(i, c.Lines)
		return lines, change && !slices.Equal(lines, c.Lines)
	}
	if change {
		s.EditLines(edit, func(lines []string) bool { return len(markupcore.PlainLines(lines)) > 0 })
	} else {
		for i, c := range s.All() {
			edit(i, c)
		}
	}
	return Report{Fixes: f.fixes}, nil
}

// ruleLanguage returns "en", "de", "fr" or "es" for a code such as "en-US" or "deu", and "" for other languages.
func ruleLanguage(code string) string {
	switch markupcore.PrimaryLanguage(code) {
	case "en", "eng":
		return "en"
	case "de", "deu", "ger":
		return "de"
	case "fr", "fra", "fre":
		return "fr"
	case "es", "spa":
		return "es"
	}
	return ""
}

type fixer struct {
	opts     Options
	language string
	turkic   bool
	rules    []Rule
	fixes    []Fix
	// startsSentence tells whether the cue that fixCue fixes starts a sentence.
	startsSentence bool
}

func newFixer(s *model.Subtitle, opts Options) *fixer {
	code := opts.Language
	if code == "" {
		code, _ = s.Metadata(model.MetadataLanguage)
	}
	f := &fixer{opts: opts, language: ruleLanguage(code), turkic: markupcore.IsTurkic(code)}
	for _, r := range Rules() {
		on := slices.Contains(opts.Enable, r) || r.Default() && !slices.Contains(opts.Disable, r)
		if on && (r != RuleLoneLowercaseI || f.language == "en") {
			f.rules = append(f.rules, r)
		}
	}
	return f
}

// fixCue runs the rules on the lines of the cue i and records each change.
func (f *fixer) fixCue(i int, lines []string) []string {
	for _, r := range f.rules {
		if len(lines) == 0 {
			break
		}
		fixed := fixes[r](f, lines)
		if !slices.Equal(fixed, lines) {
			f.fixes = append(f.fixes, Fix{CueIndex: i, Rule: r, Before: strings.Join(lines, "\n"), After: strings.Join(fixed, "\n")})
			lines = fixed
		}
	}
	return lines
}
