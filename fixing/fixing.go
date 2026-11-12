// Package fixing fixes common errors in cue text: spacing, punctuation, dialogue dashes, unbalanced tags and the
// OCR errors of image subtitles, such as lt's for It's. docs/text.md has the rules.
package fixing

import (
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

// Rule names one fix. The value is the CLI name.
type Rule string

// The rules, in run order.
const (
	// RuleAdCues removes a cue whose visible text names a subtitle download site. An image cue stays, with no lines. Off by default.
	RuleAdCues Rule = "adCues"
	// RuleReplaceList applies Options.ReplaceList. It runs when the list is not nil and Disable does not name it.
	RuleReplaceList Rule = "replaceList"
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
		RuleAdCues, RuleReplaceList, RuleUnbalancedTags, RuleEmptyTags, RuleDoubleApostrophes, RuleOCRPipe, RuleOCRZeroInWords,
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

// Default reports whether r runs without Options.Enable. RuleReplaceList counts as default, and it still needs
// Options.ReplaceList. It is false for a name that is no rule.
func (r Rule) Default() bool {
	switch r {
	case RuleReplaceList, RuleUnbalancedTags, RuleEmptyTags, RuleOCRPipe, RuleOCRZeroInWords, RuleOCRLowercaseL, RuleEllipsis,
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
	// ReplaceList holds the OCR replace list for the replaceList rule. Nil turns the rule off.
	ReplaceList *ReplaceList
}

// Validate returns an error that matches model.ErrInvalidArgument for an unknown rule name, a rule in both
// Enable and Disable, replaceList in Enable without a list, a regular expression that does not compile, or a style outside its constants.
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
	if err := o.validateReplaceList(); err != nil {
		return err
	}
	if o.DialogueDashStyle.Text() == "" {
		return invalid("DialogueDashStyle", "The style "+o.DialogueDashStyle.String()+" is no dialogue dash style.")
	}
	if o.Ellipsis > EllipsisUnicode {
		return invalid("Ellipsis", "The style "+o.Ellipsis.String()+" is not EllipsisThreeDots or EllipsisUnicode.")
	}
	return nil
}

func (o Options) validateReplaceList() error {
	if o.ReplaceList == nil {
		if slices.Contains(o.Enable, RuleReplaceList) {
			return invalid("Enable", "The rule replaceList needs a replace list.")
		}
		return nil
	}
	return o.ReplaceList.Validate()
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
	cues := s.Cues()
	var ends []bool
	if slices.Contains(f.rules, RuleSentenceStartCase) {
		for _, c := range cues {
			ends = append(ends, endsSentence(markupcore.VisibleText(c.Text())))
		}
	}
	edited, changed, err := f.fixCues(cues, ends)
	if err != nil {
		return Report{}, err
	}
	if change {
		s.EditLines(func(i int, _ model.Cue) ([]string, bool) { return edited[i], changed[i] },
			func(lines []string) bool { return len(markupcore.PlainLines(lines)) > 0 })
	}
	return Report{Fixes: f.fixes}, nil
}

// fixCues returns the lines of each cue after the rules, and whether they differ from the lines before.
func (f *fixer) fixCues(cues []model.Cue, ends []bool) (edited [][]string, changed []bool, err error) {
	edited = make([][]string, len(cues))
	changed = make([]bool, len(cues))
	for i, c := range cues {
		if len(c.Lines) == 0 {
			continue
		}
		f.startsSentence = i == 0 || ends != nil && ends[i-1]
		f.cue = i
		f.continues = f.runner != nil && continues(cues, i)
		edited[i] = f.fixCue(i, c.Lines)
		if f.err != nil {
			return nil, nil, f.err
		}
		changed[i] = !slices.Equal(edited[i], c.Lines)
	}
	return edited, changed, nil
}

// continuationGap is the longest gap between 2 cues that still holds one sentence.
const continuationGap = 600 * time.Millisecond

// continues reports whether the cue i runs on in the next cue: the next cue starts at most continuationGap after
// it ends, and its text starts with a lower case letter.
func continues(cues []model.Cue, i int) bool {
	if i+1 >= len(cues) || cues[i+1].Start-cues[i].End > continuationGap {
		return false
	}
	return startsWith(strings.Join(markupcore.PlainLines(cues[i+1].Lines), "\n"), unicode.IsLower)
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
	runner   *listRunner
	// cue and continues describe the cue that fixCue fixes, for the replaceList rule.
	cue       int
	continues bool
	// err is the first failure of a rule. It stops the run.
	err error
	// startsSentence tells whether the cue that fixCue fixes starts a sentence.
	startsSentence bool
}

// runs reports whether the options turn the rule r on, before the language check.
func (o Options) runs(r Rule) bool {
	if r == RuleReplaceList {
		return o.ReplaceList != nil && !slices.Contains(o.Disable, r)
	}
	return slices.Contains(o.Enable, r) || r.Default() && !slices.Contains(o.Disable, r)
}

func newFixer(s *model.Subtitle, opts Options) *fixer {
	code := opts.Language
	if code == "" {
		code, _ = s.Metadata(model.MetadataLanguage)
	}
	f := &fixer{opts: opts, language: ruleLanguage(code), turkic: markupcore.IsTurkic(code)}
	for _, r := range Rules() {
		if opts.runs(r) && (r != RuleLoneLowercaseI || f.language == "en") {
			f.rules = append(f.rules, r)
		}
	}
	if opts.ReplaceList != nil {
		f.runner = newListRunner(opts.ReplaceList)
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
		if f.err != nil {
			return nil
		}
		if !slices.Equal(fixed, lines) {
			f.fixes = append(f.fixes, Fix{CueIndex: i, Rule: r, Before: strings.Join(lines, "\n"), After: strings.Join(fixed, "\n")})
			lines = fixed
		}
	}
	return lines
}
