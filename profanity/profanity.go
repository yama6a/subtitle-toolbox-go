// Package profanity masks words in cue text and returns the time ranges of the matches, so that a player can
// mute the audio there. docs/text.md has the rules.
package profanity

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dlclark/regexp2/v2"

	"github.com/yama6a/subtitle-toolbox-go/internal/grapheme"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Mask is how Apply writes a match. The values are the values of the CLI option --mask.
type Mask string

// The masks. The zero value "" acts as MaskStars.
const (
	MaskStars       Mask = "stars"        // one star per character
	MaskFirstLetter Mask = "first-letter" // the first character, then one star per other character
	MaskRemove      Mask = "remove"       // removes the word
	MaskNone        Mask = "none"         // keeps the word, so only the report holds the ranges
)

// Options holds the settings of Apply.
type Options struct {
	// Words holds one word or phrase each. A * at the end matches any ending, so damn* matches damned.
	Words []string
	// Mask is how Apply writes a match. "" acts as MaskStars.
	Mask Mask
	// MaskFunc writes each match when it is set, and Mask does not count. Apply escapes its result as text.
	MaskFunc func(word string) string
	// Padding widens each mute range on both sides.
	Padding time.Duration
}

// MuteRange is a span of the audio that holds a match.
type MuteRange struct {
	Start, End time.Duration
}

// Report holds the mute ranges of Apply, sorted and joined.
type Report struct {
	MuteRanges []MuteRange
}

// wordChar is a character that continues a word. A match must not have one on either side.
const wordChar = `[\p{L}\p{M}\p{N}_]`

// matchTimeout limits each match, because regexp2 backtracks and the words are user input.
const matchTimeout = time.Second

// Validate returns an error that matches model.ErrInvalidArgument when a word is not valid, the word list is
// empty, Padding is negative or Mask is unknown.
func (o Options) Validate() error {
	for _, w := range o.Words {
		if !validWord(w) {
			return invalid("Words", "A profanity word must be a non-empty UTF-8 string with a * only at the end, got "+
				phpQuote(w)+".")
		}
	}
	if len(o.Words) == 0 {
		return invalid("Words", "The profanity word list is empty.")
	}
	if o.Padding < 0 {
		return invalid("Padding", "The padding must not be negative, got "+timecode.Seconds(o.Padding)+".")
	}
	switch o.Mask {
	case "", MaskStars, MaskFirstLetter, MaskRemove, MaskNone:
		return nil
	default:
		return invalid("Mask", "The mask "+strconv.Quote(string(o.Mask))+" is not stars, first-letter, remove or none.")
	}
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "profanity.Options." + field, Msg: msg}
}

// validWord checks the PHP pattern ^[^*\s](?:[^*]*[^*\s])?\*?$ with \s as Unicode white space.
func validWord(w string) bool {
	if !utf8.ValidString(w) {
		return false
	}
	w = strings.TrimSuffix(w, "*")
	if w == "" || strings.Contains(w, "*") {
		return false
	}
	first, _ := utf8.DecodeRuneInString(w)
	last, _ := utf8.DecodeLastRuneInString(w)
	return !markupcore.IsSpace(first) && !markupcore.IsSpace(last)
}

// phpQuote writes s as PHP var_export writes a string.
func phpQuote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// Apply masks the words of opts in the text runs of the cues of s and returns the mute ranges. It removes each
// cue that has no visible text left, and the comments of a removed cue move to the next cue. Apply returns the
// error of Validate, or an error that matches model.ErrInvalidArgument when a match takes longer than 1 s. Then
// it changes nothing.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	return apply(s, opts, matchTimeout)
}

func apply(s *model.Subtitle, opts Options, timeout time.Duration) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	f := filter{re: compile(opts.Words, timeout), mask: opts.maskFunc()}
	edited := make([][]string, s.Len())
	var ranges []MuteRange
	for i, c := range s.All() {
		if len(c.Lines) == 0 {
			continue
		}
		lines, found, ok := f.cue(c)
		if !ok {
			return Report{}, invalid("Words", "The word match took longer than 1 s on cue "+strconv.Itoa(i+1)+".")
		}
		edited[i] = lines
		ranges = append(ranges, found...)
	}
	s.EditLines(func(i int, _ model.Cue) ([]string, bool) { return edited[i], edited[i] != nil }, nil)
	return Report{MuteRanges: join(ranges, opts.Padding)}, nil
}

// compile builds the pattern of the words: the longest word first, so a phrase wins over a word that it starts
// with.
func compile(words []string, timeout time.Duration) *regexp2.Regexp {
	var unique []string
	seen := make(map[string]bool, len(words))
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			unique = append(unique, w)
		}
	}
	slices.SortStableFunc(unique, func(a, b string) int { return cmp.Compare(len(b), len(a)) })
	alternatives := make([]string, len(unique))
	for i, w := range unique {
		if prefix, ok := strings.CutSuffix(w, "*"); ok {
			alternatives[i] = quoteMeta(prefix) + wordChar + "*"
		} else {
			alternatives[i] = quoteMeta(w)
		}
	}
	pattern := "(?<!" + wordChar + ")(?:" + strings.Join(alternatives, "|") + ")(?!" + wordChar + ")"
	re := regexp2.MustCompile(pattern, regexp2.None)
	re.MatchTimeout = timeout
	return re
}

// quoteMeta escapes the metacharacters of regexp2 and writes each rune that has other cases as a class of its
// unicode.SimpleFold orbit, such as U+01C4 U+01C5 U+01C6 for U+01C5. So the pattern folds case as PCRE does.
// regexp2.IgnoreCase folds only Lu and Ll runes. regexp2.Escape writes a rune outside the printable ones as \u with
// fewer than 4 hex digits, such as \u600 for U+0600.
func quoteMeta(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.SimpleFold(r) != r {
			b.WriteByte('[')
			for f := r; ; {
				b.WriteRune(f)
				if f = unicode.SimpleFold(f); f == r {
					break
				}
			}
			b.WriteByte(']')
			continue
		}
		if strings.ContainsRune(`\.+*?()|[]{}^$# `, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (o Options) maskFunc() func(string) string {
	if o.MaskFunc != nil {
		return o.MaskFunc
	}
	switch o.Mask {
	case MaskFirstLetter:
		return func(w string) string {
			for c := range grapheme.Clusters(w) {
				return c + strings.Repeat("*", clusters(w)-1)
			}
			return ""
		}
	case MaskRemove:
		return func(string) string { return "" }
	case MaskNone:
		return func(w string) string { return w }
	case "", MaskStars:
	}
	return func(w string) string { return strings.Repeat("*", clusters(w)) }
}

func clusters(s string) int {
	n := 0
	for range grapheme.Clusters(s) {
		n++
	}
	return n
}

type filter struct {
	re   *regexp2.Regexp
	mask func(string) string
}

func (f filter) cue(c model.Cue) ([]string, []MuteRange, bool) {
	tokens := tokenize(c)
	var ranges []MuteRange
	changed := false
	for i, tok := range tokens {
		if tok.tag || tok.text == "" {
			continue
		}
		masked, found, ok := f.run(tok.text)
		if !ok {
			return nil, nil, false
		}
		if !found {
			continue
		}
		// FixOverlaps can end a cue before a word timestamp, and players skip a range that ends before it starts.
		ranges = append(ranges, MuteRange{Start: min(tok.start, tok.end), End: max(tok.start, tok.end)})
		if masked != tok.text {
			tokens[i].text = masked
			changed = true
		}
	}
	if !changed {
		return nil, ranges, true
	}
	lines := make([]string, len(c.Lines))
	for _, tok := range tokens {
		lines[tok.line] += tok.text
	}
	return lines, ranges, true
}

// token is a text run or a tag of a cue line. A text run starts at the last word timestamp before it in the cue,
// or at the cue start, and ends at the first word timestamp after it, or at the cue end.
type token struct {
	line       int
	text       string
	tag        bool
	start, end time.Duration
}

func tokenize(c model.Cue) []token {
	var tokens []token
	t := c.Start
	for i, line := range c.Lines {
		for k, part := range markupcore.SplitTags(line) {
			if d, ok := markupcore.WordTimestamp(part); ok {
				t = d
			}
			tokens = append(tokens, token{line: i, text: part, tag: k%2 == 1, start: t})
		}
	}
	t = c.End
	for i := range slices.Backward(tokens) {
		tokens[i].end = t
		if d, ok := markupcore.WordTimestamp(tokens[i].text); ok {
			t = d
		}
	}
	return tokens
}

func (f filter) run(raw string) (masked string, found, ok bool) {
	text := markupcore.UnescapeText(raw)
	if !utf8.ValidString(text) {
		return raw, false, true
	}
	n := 0
	out, err := f.re.ReplaceFunc(text, func(m regexp2.Match) string {
		n++
		return f.mask(m.String())
	}, -1, -1)
	if err != nil || n == 0 {
		return raw, false, err == nil
	}
	if out == text {
		return raw, true, true
	}
	return markupcore.EscapeTextLike(out, raw), true, true
}

func join(ranges []MuteRange, padding time.Duration) []MuteRange {
	for i, r := range ranges {
		end := r.End + padding
		if end < r.End {
			end = math.MaxInt64
		}
		ranges[i] = MuteRange{Start: max(0, r.Start-padding).Round(time.Millisecond), End: end.Round(time.Millisecond)}
	}
	slices.SortFunc(ranges, func(a, b MuteRange) int {
		return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End))
	})
	var out []MuteRange
	for _, r := range ranges {
		if n := len(out); n > 0 && r.Start <= out[n-1].End {
			out[n-1].End = max(out[n-1].End, r.End)
			continue
		}
		out = append(out, r)
	}
	return out
}

// EDL writes the ranges as an MPlayer and Kodi edit decision list, one range per line with action 1, mute:
// "62.380 62.900 1" and LF. No ranges give "".
func EDL(ranges []MuteRange) string {
	var b strings.Builder
	for _, r := range ranges {
		b.WriteString(seconds(r.Start) + " " + seconds(r.End) + " 1\n")
	}
	return b.String()
}

// FFmpegVolumeFilter writes an FFmpeg audio filter that mutes the ranges, such as
// "volume=enable='between(t,62.380,62.900)':volume=0", or "" for no ranges.
func FFmpegVolumeFilter(ranges []MuteRange) string {
	if len(ranges) == 0 {
		return ""
	}
	parts := make([]string, len(ranges))
	for i, r := range ranges {
		parts[i] = "between(t," + seconds(r.Start) + "," + seconds(r.End) + ")"
	}
	return "volume=enable='" + strings.Join(parts, "+") + "':volume=0"
}

// seconds writes d in seconds with 3 decimals from its whole milliseconds, so no float rounding takes part.
func seconds(d time.Duration) string {
	ms := d.Round(time.Millisecond).Milliseconds()
	sign := ""
	if ms < 0 {
		sign, ms = "-", -ms
	}
	frac := strconv.FormatInt(ms%1000, 10)
	return sign + strconv.FormatInt(ms/1000, 10) + "." + strings.Repeat("0", 3-len(frac)) + frac
}
