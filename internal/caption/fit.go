package caption

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
)

// FitAction is the kind of a change that a fit made. The values print as they are.
type FitAction string

// The fit actions.
const (
	Transliterated FitAction = "transliterated"
	Wrapped        FitAction = "wrapped"
	Delayed        FitAction = "delayed"
	Dropped        FitAction = "dropped"
)

// FitChange is one change that a fit made to a cue.
type FitChange struct {
	// CueIndex is the 0-based index of the cue in the subtitle.
	CueIndex int
	// Action is the kind of the change.
	Action FitAction
	// Message is the sentence that describes the change.
	Message string
}

// fitText replaces the characters that CEA-608 lacks, then wraps the lines again when they break the limits.
// It returns the changes with the new lines.
func fitText(lines []string, start time.Duration, index int) ([]string, []FitChange) {
	var changes []FitChange
	prefix := "Cue #" + strconv.Itoa(index) + " at " + timecode.Seconds(start) + " s"
	lines, replaced := transliterate(lines)
	if len(replaced) > 0 {
		changes = append(changes, FitChange{index, Transliterated, prefix + ": replaced " + strings.Join(replaced, ", ") + "."})
	}

	var lengths []int
	for _, line := range lines {
		if n := len(cea608.StyledCharacters(line)); n > 0 {
			lengths = append(lengths, n)
		}
	}
	if len(lengths) <= cea608.MaxLines && (len(lengths) == 0 || slices.Max(lengths) <= cea608.Columns) {
		return lines, changes
	}
	// Wrapping keeps every visible character but spaces, so more of them than the rows hold never fit.
	// The check skips a break search that takes seconds for a long CJK cue.
	if measuredLength(strings.Join(lines, " ")) > cea608.Columns*cea608.MaxLines {
		return lines, changes
	}
	wrapped, ok := linewrap.WrapToFit(lines, cea608.Columns, cea608.MaxLines)
	if !ok {
		wrapped = linewrap.Wrap(lines, cea608.Columns, cea608.MaxLines, false)
		if !linewrap.Fits(wrapped, cea608.Columns, cea608.MaxLines) {
			return lines, changes
		}
	}
	changes = append(changes, FitChange{index, Wrapped, prefix + ": wrapped the text into " + strconv.Itoa(len(wrapped)) +
		" lines of at most " + strconv.Itoa(cea608.Columns) + " characters."})
	return wrapped, changes
}

func measuredLength(text string) int {
	n := 0
	for _, w := range linewrap.MeasuredWords(text) {
		n += w.Length
	}
	return n
}

// transliterate replaces each character without a CEA-608 code that has a replacement. It decodes the entities of a
// text run that it changes and escapes the run again. It returns the replacements as `"X" with "Y"`, each
// character once, in first-seen order.
func transliterate(lines []string) ([]string, []string) {
	var replaced []string
	seen := map[rune]bool{}
	out := make([]string, len(lines))
	for i, line := range lines {
		parts := markupcore.SplitTags(line)
		for j := 0; j < len(parts); j += 2 {
			decoded := markupcore.DecodeEntities(parts[j])
			var b strings.Builder
			changed := false
			for _, r := range decoded {
				to, ok := cea608.Transliterate(r)
				if cea608.Encodable(r) || !ok {
					b.WriteRune(r)
					continue
				}
				changed = true
				b.WriteString(to)
				if !seen[r] {
					seen[r] = true
					replaced = append(replaced, `"`+string(r)+`" with "`+to+`"`)
				}
			}
			if changed {
				parts[j] = markupcore.EscapeText(b.String())
			}
		}
		out[i] = strings.Join(parts, "")
	}
	return out, replaced
}
