// Package linewrap breaks cue lines to a limit of visible characters per line and lines per cue, and joins them
// again. Each dialogue turn keeps lines of its own. Chinese and Japanese text breaks between characters where
// JIS X 4051 allows it.
package linewrap

import (
	"math"
	"regexp"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

var wordTokenRegexp = regexp.MustCompile(`(` + markupcore.TagPattern + `)|(` + markupcore.EntityPattern + `)| `)

// Word is a run of text between single spaces outside tags, with its visible length.
type Word struct {
	Text   string
	Length int
}

// MeasuredWords splits text at the ASCII spaces outside tags. A tag counts 0 characters, an entity such as &amp;
// counts 1, and other text counts its grapheme clusters, or its bytes for invalid UTF-8.
func MeasuredWords(text string) []Word {
	var words []Word
	start, length := -1, 0
	addText := func(from, to int) {
		if start < 0 {
			start = from
		}
		length += grapheme.Count(text[from:to])
	}
	at := 0
	for _, m := range wordTokenRegexp.FindAllStringSubmatchIndex(text, -1) {
		if at < m[0] {
			addText(at, m[0])
		}
		switch {
		case m[2] >= 0, m[4] >= 0:
			if start < 0 {
				start = m[0]
			}
			if m[4] >= 0 {
				length++
			}
		case start >= 0:
			words = append(words, Word{Text: text[start:m[0]], Length: length})
			start, length = -1, 0
		}
		at = m[1]
	}
	if at < len(text) {
		addText(at, len(text))
	}
	if start >= 0 {
		words = append(words, Word{Text: text[start:], Length: length})
	}
	return words
}

// Length returns the length of the words joined with 1 space.
func Length(words []Word) int {
	n := max(0, len(words)-1)
	for _, w := range words {
		n += w.Length
	}
	return n
}

// Fits reports whether the lines are at most maxLines and each line, measured as its words joined with 1 space,
// is at most cpl characters long.
func Fits(lines []string, cpl, maxLines int) bool {
	if len(lines) > maxLines {
		return false
	}
	for _, line := range lines {
		if Length(MeasuredWords(line)) > cpl {
			return false
		}
	}
	return true
}

// WrapToFit returns Wrap with keepDialogueLines, and false when the result does not fit cpl and maxLines.
func WrapToFit(lines []string, cpl, maxLines int) ([]string, bool) {
	wrapped := Wrap(lines, cpl, maxLines, true)
	if !Fits(wrapped, cpl, maxLines) {
		return nil, false
	}
	return wrapped, true
}

// FewestLines returns the line count of Wrap(lines, cpl, math.MaxInt, false) in linear time. It fills each line
// with as many parts as fit. When a part is longer than cpl, Wrap puts each part on a line of its own.
func FewestLines(lines []string, cpl int) int {
	var text string
	if segments := segments(lines, false); len(segments) > 0 {
		text = segments[0]
	}
	parts := breakableParts(text)
	count, length := 0, 0
	for _, p := range parts {
		if p.length > cpl {
			return len(parts)
		}
		joined := length + p.length
		if p.spaceBefore {
			joined++
		}
		if count > 0 && joined <= cpl {
			length = joined
		} else {
			count++
			length = p.length
		}
	}
	return count
}

// VisibleCharacters returns the sum of the visible lengths of the lines.
func VisibleCharacters(lines []string) int {
	n := 0
	for _, line := range lines {
		n += markupcore.VisibleLength(line)
	}
	return n
}

// VisibleLineLengths returns the visible length of each line that has at least 1 visible character.
func VisibleLineLengths(lines []string) []int {
	var lengths []int
	for _, line := range lines {
		if n := markupcore.VisibleLength(line); n > 0 {
			lengths = append(lengths, n)
		}
	}
	return lengths
}

// CharactersPerSecond returns characters divided by the duration in seconds, or +Inf for a duration of 0 or less.
func CharactersPerSecond(characters int, duration time.Duration) float64 {
	if duration <= 0 {
		return math.Inf(1)
	}
	return float64(characters) / duration.Seconds()
}

// FitsCharactersPerSecond reports whether the visible characters of the lines per second of duration are at most
// maxCPS. A maxCPS of 0 means no limit. Lines without visible characters always fit.
func FitsCharactersPerSecond(lines []string, duration time.Duration, maxCPS float64) bool {
	if maxCPS == 0 {
		return true
	}
	n := VisibleCharacters(lines)
	return n == 0 || CharactersPerSecond(n, duration) <= maxCPS
}
