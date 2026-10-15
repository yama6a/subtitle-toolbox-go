package model

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Cue is one timed text block. Its fields are plain data. A Subtitle rounds and cleans a cue when it takes it.
type Cue struct { //nolint:recvcheck // the API reads a copy with value receivers and changes the cue with pointer receivers
	// Start is the time the cue appears. It may be negative. Writers clamp it to 0.
	Start time.Duration
	// End is the time the cue disappears. Nothing checks that it is at or after Start.
	End time.Duration
	// Lines holds the text lines in core markup. Nil is a cue without text.
	Lines []string
	// ID is the identifier of the cue, such as a WebVTT cue identifier. The empty string means none.
	ID string
	// Alignment is the place of the cue on the screen.
	Alignment Alignment
	// Forced marks a forced narrative, which a player shows also when the viewer turned subtitles off.
	Forced bool
}

// Text returns the lines joined with LF.
func (c Cue) Text() string {
	return strings.Join(c.Lines, "\n")
}

// SetText sets the lines to the parts of text between LF characters, cleaned, without the empty ones.
func (c *Cue) SetText(text string) {
	c.Lines = cleanLines(strings.Split(text, "\n"))
}

// AddLine cleans line and appends it. An empty result is ignored.
func (c *Cue) AddLine(line string) {
	if line = cleanLine(line); line != "" {
		c.Lines = append(c.Lines, line)
	}
}

// MapWordTimestamps replaces the time of each word timestamp in the lines, such as <00:00:02.000>, with fn of it.
// The result is rounded to whole milliseconds, and a time below 0 becomes 0.
func (c *Cue) MapWordTimestamps(fn func(time.Duration) time.Duration) {
	for i, line := range c.Lines {
		if strings.Contains(line, "<") {
			c.Lines[i] = mapWordTimestamps(line, fn)
		}
	}
}

// Clone returns a copy of c that shares no slice with c.
func (c Cue) Clone() Cue {
	c.Lines = slices.Clone(c.Lines)
	return c
}

func (c Cue) normalized() Cue {
	c.Start = c.Start.Round(time.Millisecond)
	c.End = c.End.Round(time.Millisecond)
	c.Lines = cleanLines(c.Lines)
	return c
}

func cleanLines(lines []string) []string {
	var out []string
	for _, l := range lines {
		if l = cleanLine(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

func cleanLine(s string) string {
	return strings.Trim(collapseRuns(normalizeLineEnds(s)), phpTrimSet)
}

// normalizeLineEnds turns each run of CR characters followed by LF into one LF. A CRLF file gets 2 CR
// characters before each LF from a second text-mode conversion. A lone CR becomes LF too.
func normalizeLineEnds(s string) string {
	if !strings.Contains(s, "\r") {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\r' {
			out = append(out, s[i])
			continue
		}
		j := i + 1
		for j < len(s) && s[j] == '\r' {
			j++
		}
		if j < len(s) && s[j] == '\n' {
			i = j
		}
		out = append(out, '\n')
	}
	return string(out)
}

func collapseRuns(s string) string {
	if !strings.ContainsAny(s, "\t\n") && !strings.Contains(s, "  ") {
		return s
	}
	out := make([]byte, 0, len(s))
	for i := range len(s) {
		ch := s[i]
		if ch == '\t' {
			ch = ' '
		}
		if (ch == ' ' || ch == '\n') && len(out) > 0 && out[len(out)-1] == ch {
			continue
		}
		out = append(out, ch)
	}
	return string(out)
}

// phpTrimSet holds the characters of PHP trim. strings.TrimSpace would also remove U+00A0.
const phpTrimSet = " \t\n\r\x00\x0b"

var wordTimestamp = regexp.MustCompile(`<(\d{2,}):([0-5]\d):([0-5]\d)\.(\d{3})>`)

const maxWordTimestampHours = int64(time.Duration(1<<63-1) / time.Hour)

func mapWordTimestamps(line string, fn func(time.Duration) time.Duration) string {
	return wordTimestamp.ReplaceAllStringFunc(line, func(tag string) string {
		m := wordTimestamp.FindStringSubmatch(tag)
		hours, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil || hours >= maxWordTimestampHours {
			return tag
		}
		minutes, _ := strconv.Atoi(m[2])
		seconds, _ := strconv.Atoi(m[3])
		millis, _ := strconv.Atoi(m[4])
		d := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute +
			time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond
		d = max(0, fn(d).Round(time.Millisecond))
		ms := d.Milliseconds()
		return fmt.Sprintf("<%02d:%02d:%02d.%03d>", ms/3_600_000, ms/60_000%60, ms/1000%60, ms%1000)
	})
}
