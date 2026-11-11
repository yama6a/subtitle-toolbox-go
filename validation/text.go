package validation

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

var (
	// tagRegexp matches the tags that noUnbalancedTags reads, in lower case text.
	tagRegexp   = regexp.MustCompile(`<(/?)(b|i|u|s|font|v|c|lang|ruby|rt)(?:>|[\t\n\v\f\r .][^<>]*>)`)
	voiceRegexp = regexp.MustCompile(markupcore.VoiceTagPattern)
	bracketSpan = regexp.MustCompile(`\[[^\]]*\]|\([^)]*\)`)
)

// textRules applies the text and word rules of one cue. raw holds the lines with markup and text holds the plain
// lines that have text.
func (c *checker) textRules(i int, raw, text []string, words int, secs float64) error {
	r := c.rules
	if r.NoDoubleSpaces {
		n := 0
		for _, line := range text {
			n += doubleSpaces(line)
		}
		c.count(i, RuleNoDoubleSpaces, n)
	}
	if r.NoLeadingOrTrailingSpaces {
		n := 0
		for _, line := range text {
			ch := newChars(line)
			if ch.horizontal(0) || ch.horizontal(len(ch.runes)-1) {
				n++
			}
		}
		c.count(i, RuleNoLeadingOrTrailingSpaces, n)
	}
	rawText := strings.Join(raw, "\n")
	if r.NoUnbalancedTags {
		c.count(i, RuleNoUnbalancedTags, unbalancedTags(rawText))
	}
	if r.DialogueDashStyle != nil {
		c.count(i, RuleDialogueDashStyle, wrongDashes(text, *r.DialogueDashStyle))
	}
	if r.MaxSpeakersPerCue > 0 {
		if n := speakers(rawText, text); n > r.MaxSpeakersPerCue {
			c.add(i, RuleMaxSpeakersPerCue, float64(n), limitOf(float64(r.MaxSpeakersPerCue)))
		}
	}
	if r.MaxWordsPerMinute > 0 && words > 0 {
		if v := perSecond(float64(words), secs) * 60; v > r.MaxWordsPerMinute {
			c.add(i, RuleMaxWordsPerMinute, v, limitOf(r.MaxWordsPerMinute))
		}
	}
	if limit := r.MinDurationPerWord; limit > 0 && words > 0 {
		need := math.Round(limit.Seconds() * float64(words) * 1000)
		if secs*1000 < need {
			c.add(i, RuleMinDurationPerWord, secs/float64(words), limitOf(limit.Seconds()))
		}
	}
	if r.AllowedCharacters != "" {
		n := 0
		for _, line := range text {
			k, err := c.disallowed(line)
			if err != nil {
				return err
			}
			n += k
		}
		c.count(i, RuleAllowedCharacters, n)
	}
	if r.NoAllCapsLines {
		n := 0
		for _, line := range text {
			if isAllCaps(line) {
				n++
			}
		}
		c.count(i, RuleNoAllCapsLines, n)
	}
	return nil
}

// chars holds the characters of a line: its code points, or its bytes when the line is not valid UTF-8.
type chars struct {
	runes []rune
	valid bool
}

func newChars(line string) chars {
	if utf8.ValidString(line) {
		return chars{runes: []rune(line), valid: true}
	}
	runes := make([]rune, len(line))
	for i := range len(line) {
		runes[i] = rune(line[i])
	}
	return chars{runes: runes}
}

func (c chars) space(i int) bool {
	r := c.runes[i]
	if !c.valid {
		return r == ' ' || r >= '\t' && r <= '\r'
	}
	return markupcore.IsSpace(r)
}

func (c chars) horizontal(i int) bool {
	if i < 0 || i >= len(c.runes) {
		return false
	}
	r := c.runes[i]
	if !c.valid {
		return r == ' ' || r == '\t'
	}
	switch {
	case r == ' ', r == '\t', r == 0xA0, r == 0x1680, r == 0x180E, r >= 0x2000 && r <= 0x200A,
		r == 0x202F, r == 0x205F, r == 0x3000:
		return true
	}
	return false
}

// doubleSpaces counts the runs of 2 or more horizontal spaces with a character that is not white space on both sides.
func doubleSpaces(line string) int {
	c := newChars(line)
	n := 0
	for i := 0; i < len(c.runes); {
		if !c.horizontal(i) {
			i++
			continue
		}
		start := i
		for c.horizontal(i) {
			i++
		}
		if i-start >= 2 && start > 0 && i < len(c.runes) && !c.space(start-1) && !c.space(i) {
			n++
		}
	}
	return n
}

// unbalancedTags counts the tags of raw that have no partner. An open <v> needs no </v>. </ruby> and the next <rt>
// close an open <rt>.
func unbalancedTags(raw string) int {
	var open []string
	n := 0
	for _, m := range tagRegexp.FindAllStringSubmatch(markupcore.ASCIILower(raw), -1) {
		name := m[2]
		if m[1] == "" {
			if name == "rt" {
				if len(open) > 0 && open[len(open)-1] == "rt" {
					open = open[:len(open)-1]
				}
				if !slices.Contains(open, "ruby") {
					n++
				}
			}
			open = append(open, name)
			continue
		}
		at := lastIndex(open, name)
		if at < 0 {
			n++
			continue
		}
		inner := open[at+1:]
		if name == "ruby" && len(inner) > 0 && inner[0] == "rt" {
			inner = inner[1:]
		}
		n += unclosed(inner)
		open = open[:at]
	}
	return n + unclosed(open)
}

func unclosed(names []string) int {
	n := 0
	for _, name := range names {
		if name != "v" {
			n++
		}
	}
	return n
}

func lastIndex(names []string, name string) int {
	for k := len(names) - 1; k >= 0; k-- {
		if names[k] == name {
			return k
		}
	}
	return -1
}

// dashStart returns the line without the spaces and bidi controls before its first character.
func dashStart(line string) string {
	_, rest := markupcore.SplitLeadingBidiControls(strings.TrimLeft(line, markupcore.PHPTrimSet))
	return strings.TrimLeft(rest, markupcore.PHPTrimSet)
}

// wrongDashes counts the lines that open a dialogue turn in another style than style.
func wrongDashes(text []string, style model.DialogueDashStyle) int {
	n := 0
	for _, line := range text {
		start := dashStart(line)
		if !markupcore.StartsDialogueTurn(start) {
			continue
		}
		rest, ok := strings.CutPrefix(start, style.Text())
		if r, _ := utf8.DecodeRuneInString(rest); !ok || rest == "" || markupcore.IsSpace(r) {
			n++
		}
	}
	return n
}

// speakers returns the largest of the dash lines, the distinct <v> names and 1 for a cue with text.
func speakers(raw string, text []string) int {
	dashes := 0
	for _, line := range text {
		if markupcore.StartsDialogueTurn(dashStart(line)) {
			dashes++
		}
	}
	names := make(map[string]bool)
	for _, m := range voiceRegexp.FindAllStringSubmatch(raw, -1) {
		names[strings.Trim(m[2], markupcore.PHPTrimSet)] = true
	}
	return max(dashes, len(names), min(len(text), 1))
}

// isAllCaps reports whether a line has 2 or more upper case letters and no lower case letter. Text in brackets
// and parentheses does not count.
func isAllCaps(line string) bool {
	line = bracketSpan.ReplaceAllString(line, "")
	upper, lower := 0, 0
	c := newChars(line)
	for _, r := range c.runes {
		switch {
		case !c.valid && r >= utf8.RuneSelf:
		case unicode.IsUpper(r):
			upper++
		case unicode.IsLower(r):
			lower++
		}
	}
	return upper >= 2 && lower == 0
}

// disallowed counts the characters of line that the allowed characters do not hold. U+0020 is always allowed.
func (c *checker) disallowed(line string) (int, error) {
	valid := utf8.ValidString(line)
	n := 0
	for _, ch := range markupcore.Characters(line) {
		if ch == " " {
			continue
		}
		if c.class == nil {
			if !c.list[ch] {
				n++
			}
			continue
		}
		if !valid {
			ch = string(rune(ch[0]))
		}
		ok, err := c.classMatches(ch)
		if err != nil {
			return 0, err
		}
		if !ok {
			n++
		}
	}
	return n, nil
}

func (c *checker) classMatches(ch string) (bool, error) {
	if ok, seen := c.cache[ch]; seen {
		return ok, nil
	}
	ok, err := c.class.MatchString(ch)
	if err != nil {
		return false, &model.InvalidArgumentError{
			Option: "Rules.AllowedCharacters",
			Msg:    `The match of the allowed characters "` + c.rules.AllowedCharacters + `" took more than ` + matchTimeout.String() + ".",
		}
	}
	if c.cache == nil {
		c.cache = make(map[string]bool)
	}
	c.cache[ch] = ok
	return ok, nil
}
