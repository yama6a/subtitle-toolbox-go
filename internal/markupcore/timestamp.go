package markupcore

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	wordTimestampRegexp      = regexp.MustCompile(WordTimestampPattern)
	exactWordTimestampRegexp = regexp.MustCompile(`^<(\d{2,}):([0-5]\d):([0-5]\d)\.(\d{3})>$`)
)

const maxWordTimestampHours = math.MaxInt64 / int64(time.Hour)

// TimedWord is one word and the time it is spoken, for InsertWordTimestamps. A nil Start gives no timestamp.
type TimedWord struct {
	Text  string
	Start *time.Duration
}

// CoreTimestamp formats d as the body of a word timestamp, for example "00:01:02.500" for 62.5 s.
// It rounds to whole milliseconds, writes at least 2 hour digits, and writes a negative time as 0.
func CoreTimestamp(d time.Duration) string {
	ms := max(0, d.Round(time.Millisecond)).Milliseconds()
	return fmt.Sprintf("%02d:%02d:%02d.%03d", ms/3_600_000, ms/60_000%60, ms/1000%60, ms%1000)
}

// WordTimestamp returns the time of a word timestamp such as <00:01:02.500>, or false for other text.
// It also returns false for 2562047 hours or more, which time.Duration cannot hold.
func WordTimestamp(tag string) (time.Duration, bool) {
	m := exactWordTimestampRegexp.FindStringSubmatch(tag)
	if m == nil {
		return 0, false
	}
	hours, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || hours >= maxWordTimestampHours {
		return 0, false
	}
	minutes, _ := strconv.Atoi(m[2])
	seconds, _ := strconv.Atoi(m[3])
	millis, _ := strconv.Atoi(m[4])
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond, true
}

// MapWordTimestamps replaces the time of each word timestamp in text with fn of it, rounded to whole milliseconds.
// A time below 0 becomes 0. A word timestamp of 2562047 hours or more stays as written.
func MapWordTimestamps(text string, fn func(time.Duration) time.Duration) string {
	if !strings.Contains(text, "<") {
		return text
	}
	return wordTimestampRegexp.ReplaceAllStringFunc(text, func(tag string) string {
		d, ok := WordTimestamp(tag)
		if !ok {
			return tag
		}
		return "<" + CoreTimestamp(fn(d)) + ">"
	})
}

// InsertWordTimestamps escapes text and writes a word timestamp before each word, searching from the end of the
// word before it. A word that is empty, has no start or is not in the rest of the text gets no timestamp.
func InsertWordTimestamps(text string, words []TimedWord) string {
	var b strings.Builder
	copied, searchAt := 0, 0
	for _, w := range words {
		if w.Text == "" || w.Start == nil {
			continue
		}
		i := strings.Index(text[searchAt:], w.Text)
		if i < 0 {
			continue
		}
		i += searchAt
		b.WriteString(EscapeText(text[copied:i]))
		b.WriteString("<" + CoreTimestamp(*w.Start) + ">")
		copied, searchAt = i, i+len(w.Text)
	}
	b.WriteString(EscapeText(text[copied:]))
	return b.String()
}
