package validation

import (
	"math"
	"strings"
	"time"

	"github.com/dlclark/regexp2/v2"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/markup"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// NoCue is the cue index of a violation that is about the whole subtitle.
const NoCue = -1

// Violation is one broken rule.
type Violation struct {
	Cue   int      // 0-based cue index, NoCue for a rule about the whole subtitle
	Rule  Rule     // the broken rule
	Value float64  // the measured value in the unit of the rule. +Inf for a cue with text and no duration
	Limit *float64 // the limit in the same unit, nil for a rule without a number limit
}

// Check returns one violation per broken rule, in the order of the rule table of docs/validation.md. It does not
// change s. It calls rules.Validate first.
func Check(s *model.Subtitle, rules Rules) ([]Violation, error) {
	class, err := rules.prepare()
	if err != nil {
		return nil, err
	}
	c := &checker{rules: rules, class: class}
	if rules.AllowedCharacters != "" && class == nil {
		c.list = make(map[string]bool)
		for _, ch := range markupcore.Characters(rules.AllowedCharacters) {
			c.list[ch] = true
		}
	}
	if rules.RequireCues && s.Len() == 0 {
		c.add(NoCue, RuleRequireCues, 0, nil)
	}
	var latestEnd time.Duration
	for i, cue := range s.Cues() {
		if err := c.cue(i, cue); err != nil {
			return nil, err
		}
		if i > 0 {
			c.gap(i, cue.Start-latestEnd)
		}
		if i == 0 {
			latestEnd = cue.End
		}
		latestEnd = max(latestEnd, cue.End)
	}
	return c.out, nil
}

type checker struct {
	rules Rules
	class *regexp2.Regexp
	// list holds the allowed characters of a list, nil for a class.
	list  map[string]bool
	cache map[string]bool
	out   []Violation
}

func (c *checker) add(cue int, rule Rule, value float64, limit *float64) {
	c.out = append(c.out, Violation{Cue: cue, Rule: rule, Value: value, Limit: limit})
}

func (c *checker) count(cue int, rule Rule, n int) {
	if n > 0 {
		c.add(cue, rule, float64(n), nil)
	}
}

func limitOf(v float64) *float64 { return &v }

func seconds(d time.Duration) float64 { return float64(d.Milliseconds()) / 1000 }

// cue applies the rules of one cue except the gap rules.
func (c *checker) cue(i int, cue model.Cue) error {
	r := c.rules
	duration := cue.End - cue.Start
	secs := seconds(duration)
	characters, visibleLines := 0, 0
	lengths := make([]int, len(cue.Lines))
	for k, line := range cue.Lines {
		lengths[k] = markup.VisibleLength(line)
		characters += lengths[k]
		if lengths[k] > 0 {
			visibleLines++
		}
	}
	plain := make([]string, len(cue.Lines))
	var text []string
	for k, line := range cue.Lines {
		plain[k] = markup.PlainText(line)
		if strings.Trim(plain[k], markupcore.PHPTrimSet) != "" {
			text = append(text, plain[k])
		}
	}
	words := len(markupcore.Words(strings.Join(plain, "\n")))

	if r.NoNegativeDuration && duration < 0 {
		c.add(i, RuleNoNegativeDuration, secs, nil)
	}
	if r.NoEmptyCues && characters == 0 {
		c.add(i, RuleNoEmptyCues, 0, nil)
	}
	if r.MaxCharactersPerLine > 0 {
		for _, n := range lengths {
			if n > r.MaxCharactersPerLine {
				c.add(i, RuleMaxCharactersPerLine, float64(n), limitOf(float64(r.MaxCharactersPerLine)))
			}
		}
	}
	if r.MaxLinesPerCue > 0 && visibleLines > r.MaxLinesPerCue {
		c.add(i, RuleMaxLinesPerCue, float64(visibleLines), limitOf(float64(r.MaxLinesPerCue)))
	}
	if r.MaxCharactersPerSecond > 0 && characters > 0 {
		if v := perSecond(float64(characters), secs); v > r.MaxCharactersPerSecond {
			c.add(i, RuleMaxCharactersPerSecond, v, limitOf(r.MaxCharactersPerSecond))
		}
	}
	if r.MinDuration > 0 && duration < r.MinDuration.Round(time.Millisecond) {
		c.add(i, RuleMinDuration, secs, limitOf(r.MinDuration.Seconds()))
	}
	if r.MaxDuration > 0 && duration > r.MaxDuration.Round(time.Millisecond) {
		c.add(i, RuleMaxDuration, secs, limitOf(r.MaxDuration.Seconds()))
	}
	return c.textRules(i, cue.Lines, text, words, secs)
}

// perSecond returns n per second, or +Inf for a duration of 0 or less.
func perSecond(n, secs float64) float64 {
	if secs <= 0 {
		return math.Inf(1)
	}
	return n / secs
}

// gap applies the overlap and gap rules to the gap before cue i. A negative gap is an overlap.
func (c *checker) gap(i int, gap time.Duration) {
	if c.rules.NoOverlap && gap < 0 {
		c.add(i, RuleNoOverlap, -seconds(gap), nil)
	}
	if limit := c.rules.MinGap; limit > 0 && gap >= 0 && gap < limit.Round(time.Millisecond) {
		c.add(i, RuleMinGap, seconds(gap), limitOf(limit.Seconds()))
	}
}
