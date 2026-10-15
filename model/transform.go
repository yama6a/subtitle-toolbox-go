package model

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/markup"
)

// regexTimeout limits each match of a user pattern, because regexp2 backtracks and can run without end.
const regexTimeout = time.Second

var wordTimestampRegexp = regexp.MustCompile(markupcore.WordTimestampPattern)

// ReplaceOptions holds the settings of ReplaceText.
type ReplaceOptions struct {
	// Regex reads search as a regular expression in the syntax of github.com/dlclark/regexp2, without delimiters.
	// False matches search as plain text.
	Regex bool
	// IgnoreCase matches search in any case. False matches the case of search.
	IgnoreCase bool
}

// StripOptions holds the settings of StripFormatting.
type StripOptions struct {
	// KeepTags names the tags to keep, compared in lower case, for example "i". Nil removes every tag.
	KeepTags []string
	// RemoveWordTimestamps also removes the <hh:mm:ss.mmm> word timestamps. False keeps them.
	RemoveWordTimestamps bool
}

// EditLines calls fn for each cue with its index and a copy, and sets the cleaned lines that fn returns when fn
// returns true. Then it removes each cue that had text before and has none after, by hasText, or by
// markup.HasVisibleText when hasText is nil. A comment before a removed cue moves to the next cue.
// A cue without text before stays. EditLines returns the old indexes of the removed cues in ascending order.
// Fn must not change s.
func (s *Subtitle) EditLines(fn func(i int, c Cue) (lines []string, ok bool), hasText func(lines []string) bool) []int {
	if hasText == nil {
		hasText = markup.HasVisibleText
	}
	var removed []int
	for i := range s.cues {
		lines, ok := fn(i, s.cues[i].Clone())
		if !ok {
			continue
		}
		before := s.cues[i].Lines
		s.cues[i].Lines = cleanLines(lines)
		if hasText(before) && !hasText(s.cues[i].Lines) {
			removed = append(removed, i)
		}
	}
	if len(removed) > 0 {
		gone := make(map[int]bool, len(removed))
		for _, i := range removed {
			gone[i] = true
		}
		s.removeWhere(func(i int) bool { return gone[i] })
	}
	return removed
}

// MapText calls fn for each text run between tags, with &lt;, &gt; and &amp; decoded, and a copy of its cue.
// A changed run is escaped again in the style of its raw form, so fn cannot add tags. A run that comes back
// unchanged keeps its bytes. A cue that loses all its text goes, as EditLines says.
func (s *Subtitle) MapText(fn func(text string, c Cue) string) {
	s.EditLines(func(_ int, c Cue) ([]string, bool) {
		return markupcore.MapTextRuns(c.Lines, func(text string, _, _ bool) string { return fn(text, c) }), true
	}, nil)
}

// MapLines calls fn for each line with its tags and entities, and a copy of its cue. Use it to change tags.
// A cue that loses all its text goes, as EditLines says.
func (s *Subtitle) MapLines(fn func(line string, c Cue) string) {
	s.EditLines(func(_ int, c Cue) ([]string, bool) {
		lines := make([]string, len(c.Lines))
		for i, line := range c.Lines {
			lines[i] = fn(line, c)
		}
		return lines, true
	}, nil)
}

// ReplaceText replaces each match of search with replace in the text runs between tags, as MapText does.
// A match cannot cross a tag. A regex replace uses the regexp2 substitutions $1, ${1}, ${name} and $$, and a plain
// replace takes replace as it is. A regex keeps a run of invalid UTF-8 unchanged.
// ReplaceText returns an error that matches ErrInvalidArgument for an empty search, an invalid pattern or
// replacement, and a match that takes longer than 1 s. On an error the subtitle stays unchanged.
func (s *Subtitle) ReplaceText(search, replace string, opts ReplaceOptions) error {
	if search == "" {
		return invalidArgument("The search text must not be empty.")
	}
	replaceRun, err := newReplacer(search, replace, opts)
	if err != nil {
		return err
	}

	edited := make([][]string, len(s.cues))
	for i, c := range s.cues {
		var runErr error
		edited[i] = markupcore.MapTextRuns(c.Lines, func(text string, _, _ bool) string {
			if runErr != nil {
				return text
			}
			out, err := replaceRun(text)
			if err != nil {
				runErr = err
				return text
			}
			return out
		})
		if runErr != nil {
			return invalidArgument(fmt.Sprintf("The regular expression %s took longer than 1 s on cue %d.", quote(search), i))
		}
	}
	s.EditLines(func(i int, _ Cue) ([]string, bool) { return edited[i], true }, nil)
	return nil
}

func newReplacer(search, replace string, opts ReplaceOptions) (func(string) (string, error), error) {
	switch {
	case opts.Regex:
		return newRegexReplacer(search, replace, opts.IgnoreCase)
	case opts.IgnoreCase:
		return newFoldReplacer(search, replace)
	}
	return func(text string) (string, error) { return strings.ReplaceAll(text, search, replace), nil }, nil
}

func newRegexReplacer(search, replace string, ignoreCase bool) (func(string) (string, error), error) {
	options := regexp2.None
	if ignoreCase {
		options = regexp2.IgnoreCase
	}
	re, err := regexp2.Compile(search, options)
	if err != nil {
		return nil, invalidArgument(fmt.Sprintf("The regular expression %s is not valid: %v.", quote(search), err))
	}
	re.MatchTimeout = regexTimeout
	if _, err := re.Replace("", replace, -1, -1); err != nil {
		return nil, invalidArgument(fmt.Sprintf("The replacement %s is not valid: %v.", quote(replace), err))
	}
	return func(text string) (string, error) {
		if !utf8.ValidString(text) {
			return text, nil
		}
		return re.Replace(text, replace, -1, -1)
	}, nil
}

// newFoldReplacer matches search in any case with the case folding of package regexp. A search or a text run of
// invalid UTF-8 falls back to the case of ASCII letters, as PHP str_ireplace does.
func newFoldReplacer(search, replace string) (func(string) (string, error), error) {
	if !utf8.ValidString(search) {
		return func(text string) (string, error) { return replaceASCIIFold(text, search, replace), nil }, nil
	}
	re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(search))
	if err != nil {
		return nil, invalidArgument(fmt.Sprintf("The search text %s is not valid: %v.", quote(search), err))
	}
	return func(text string) (string, error) {
		if !utf8.ValidString(text) {
			return replaceASCIIFold(text, search, replace), nil
		}
		return re.ReplaceAllLiteralString(text, replace), nil
	}, nil
}

// replaceASCIIFold replaces search in text with a match that ignores the case of the ASCII letters only.
func replaceASCIIFold(text, search, replace string) string {
	lowerText, lowerSearch := asciiLower(text), asciiLower(search)
	var b strings.Builder
	copied := 0
	for {
		i := strings.Index(lowerText[copied:], lowerSearch)
		if i < 0 {
			break
		}
		b.WriteString(text[copied : copied+i])
		b.WriteString(replace)
		copied += i + len(search)
	}
	if copied == 0 {
		return text
	}
	b.WriteString(text[copied:])
	return b.String()
}

// StripFormatting removes each tag of each line whose name is not in opts.KeepTags. It keeps the word timestamps
// unless opts.RemoveWordTimestamps is set. ASS override blocks such as {\an8} are no tags and stay.
// A cue that loses all its text goes, as EditLines says.
func (s *Subtitle) StripFormatting(opts StripOptions) {
	strip := func(text string) string { return markupcore.KeepTags(text, opts.KeepTags...) }
	s.MapLines(func(line string, _ Cue) string {
		if opts.RemoveWordTimestamps {
			return strip(line)
		}
		var b strings.Builder
		copied := 0
		for _, loc := range wordTimestampRegexp.FindAllStringIndex(line, -1) {
			b.WriteString(strip(line[copied:loc[0]]))
			b.WriteString(line[loc[0]:loc[1]])
			copied = loc[1]
		}
		b.WriteString(strip(line[copied:]))
		return b.String()
	})
}

// FixRightToLeft wraps each line with a right-to-left letter in U+202B and U+202C, inside its leading and trailing
// tags, and moves sentence-end punctuation from the start of its text to the end. A player with a left-to-right
// base direction then shows the punctuation of Arabic or Hebrew at the right end. A second call changes nothing.
func (s *Subtitle) FixRightToLeft() {
	s.MapLines(func(line string, _ Cue) string { return markupcore.EmbedRightToLeft(line) })
}

// RemoveBidiControls removes U+200E, U+200F, U+202A to U+202E and U+2066 to U+2069 from the text runs.
// It keeps U+00A0 and moves no punctuation back. A cue that loses all its text goes, as EditLines says.
func (s *Subtitle) RemoveBidiControls() {
	s.MapText(func(text string, _ Cue) string { return markupcore.RemoveBidiControls(text) })
}

// quote writes text in double quotes as it is, so a pattern such as \.{4,} keeps its single backslash.
func quote(text string) string {
	return `"` + text + `"`
}
