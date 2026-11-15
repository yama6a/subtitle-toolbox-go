package validation

import (
	"strings"
	"time"

	"github.com/dlclark/regexp2/v2"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/inspect/internal/readingspeed"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
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
// change s. It skips the image cues without lines. It calls rules.Validate first.
func Check(s *model.Subtitle, rules Rules) ([]Violation, error) {
	c, err := newChecker(rules)
	if err != nil {
		return nil, err
	}
	if rules.RequireCues && s.Len() == 0 {
		c.add(NoCue, RuleRequireCues, 0, nil)
	}
	var latestEnd time.Duration
	seen := false
	for i, cue := range s.Cues() {
		if cue.IsImageWithoutText() {
			continue
		}
		if err := c.cue(i, cue); err != nil {
			return nil, err
		}
		if seen {
			c.gap(i, cue.Start-latestEnd)
		}
		if !seen || cue.End > latestEnd {
			latestEnd = cue.End
		}
		seen = true
	}
	return c.out, nil
}

func newChecker(rules Rules) (*checker, error) {
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
	return c, nil
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

// measures holds what the rules read of one cue.
type measures struct {
	readingspeed.Measures
	visibleLines int      // lines with a visible length of 1 or more
	raw          string   // the lines with markup, joined with LF
	speaker      string   // Speaker of the cue
	text         []string // the plain lines that have text
}

func measure(cue model.Cue) measures {
	m := measures{Measures: readingspeed.Measure(cue), raw: strings.Join(cue.Lines, "\n"), speaker: cue.Speaker}
	for k, n := range m.LineLengths {
		if n > 0 {
			m.visibleLines++
		}
		if strings.Trim(m.PlainLines[k], markupcore.TrimSet) != "" {
			m.text = append(m.text, m.PlainLines[k])
		}
	}
	return m
}

// cue applies the rules of one cue except the gap rules.
func (c *checker) cue(i int, cue model.Cue) error {
	m := measure(cue)
	c.lengthRules(i, m)
	c.speedRules(i, m)
	c.spaceRules(i, m)
	c.tagAndSpeakerRules(i, m)
	c.wordRules(i, m)
	return c.characterRules(i, m)
}

func (c *checker) lengthRules(i int, m measures) {
	r := c.rules
	if r.NoNegativeDuration && m.Duration < 0 {
		c.add(i, RuleNoNegativeDuration, m.Seconds, nil)
	}
	if r.NoEmptyCues && m.Characters == 0 {
		c.add(i, RuleNoEmptyCues, 0, nil)
	}
	if r.MaxCharactersPerLine > 0 {
		for _, n := range m.LineLengths {
			if n > r.MaxCharactersPerLine {
				c.add(i, RuleMaxCharactersPerLine, float64(n), limitOf(float64(r.MaxCharactersPerLine)))
			}
		}
	}
	if r.MaxLinesPerCue > 0 && m.visibleLines > r.MaxLinesPerCue {
		c.add(i, RuleMaxLinesPerCue, float64(m.visibleLines), limitOf(float64(r.MaxLinesPerCue)))
	}
}

func (c *checker) speedRules(i int, m measures) {
	r := c.rules
	if r.MaxCharactersPerSecond > 0 && m.Characters > 0 {
		if v := m.CharactersPerSecond(); v > r.MaxCharactersPerSecond {
			c.add(i, RuleMaxCharactersPerSecond, v, limitOf(r.MaxCharactersPerSecond))
		}
	}
	if r.MinDuration > 0 && m.Duration < r.MinDuration.Round(time.Millisecond) {
		c.add(i, RuleMinDuration, m.Seconds, limitOf(r.MinDuration.Seconds()))
	}
	if r.MaxDuration > 0 && m.Duration > r.MaxDuration.Round(time.Millisecond) {
		c.add(i, RuleMaxDuration, m.Seconds, limitOf(r.MaxDuration.Seconds()))
	}
}

// gap applies the overlap and gap rules to the gap before cue i. A negative gap is an overlap.
func (c *checker) gap(i int, gap time.Duration) {
	if c.rules.NoOverlap && gap < 0 {
		c.add(i, RuleNoOverlap, -timecode.FloatSeconds(gap), nil)
	}
	if limit := c.rules.MinGap; limit > 0 && gap >= 0 && gap < limit.Round(time.Millisecond) {
		c.add(i, RuleMinGap, timecode.FloatSeconds(gap), limitOf(limit.Seconds()))
	}
}
