package lrc

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var wordTimestampRegexp = regexp.MustCompile(markupcore.WordTimestampPattern)

// Write writes s as LRC. It writes the ID tags, then one line per cue in start order with centisecond times,
// and a time tag without text at the end of a cue with the cue data endLine. The BOM is on by default.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	file, _ := s.FormatData(formatDataKey)
	spaced, _ := file.Bool("spacedWordTimes")
	comments := make([][]string, s.Len()+1)
	for _, c := range s.Comments() {
		at := min(c.BeforeCue, s.Len())
		comments[at] = append(comments[at], c.Text)
	}

	body := appendIDTags(nil, s, file)
	for i, c := range s.All() {
		body = appendComments(body, comments[i])
		body = append(body, '[')
		body = appendStamp(body, c.Start)
		body = append(body, "] "...)
		body = appendText(body, c.Lines, spaced)
		body = append(body, '\n')
		data, _ := c.FormatData(formatDataKey)
		if endLine, _ := data.Bool("endLine"); endLine {
			body = append(body, '[')
			body = appendStamp(body, c.End)
			body = append(body, "]\n"...)
		}
	}
	body = appendComments(body, comments[s.Len()])
	return writing.Write(w, model.LRC, body, common, true)
}

func appendIDTags(body []byte, s *model.Subtitle, file *model.Data) []byte {
	written := map[string]bool{}
	add := func(tag, value string) {
		if !written[tag] {
			written[tag] = true
			body = append(body, "["+tag+":"+markupcore.ToSingleLine(value)+"]\n"...)
		}
	}
	for _, t := range metadataTags {
		if value, ok := s.Metadata(t.key); ok {
			add(t.tag, value)
		}
	}
	stored, _ := file.Get("idTags")
	switch stored := stored.(type) {
	case *model.Data:
		for _, tag := range stored.Keys() {
			value, _ := stored.String(tag)
			add(tag, value)
		}
	case []any:
		// A JSON list reads into PHP as an array with the keys 0, 1 and so on.
		for i, item := range stored {
			value, _ := item.(string)
			add(strconv.Itoa(i), value)
		}
	}
	return body
}

func appendComments(body []byte, comments []string) []byte {
	for _, text := range comments {
		body = append(body, "[#:"+markupcore.ToSingleLine(text)+"]\n"...)
	}
	return body
}

func appendText(body []byte, lines []string, spaced bool) []byte {
	parts, times := splitAtWordTimestamps(markupcore.RubyAsText(strings.Join(lines, " ")))
	start := len(body)
	body = append(body, parts[0]...)
	for i, d := range times {
		next := parts[i+1]
		if spaced && i == len(times)-1 && next == "" && len(body) > start && !isTrimByte(body[len(body)-1]) {
			body = append(body, ' ')
		}
		body = append(body, '<')
		body = appendStamp(body, d)
		body = append(body, '>')
		if spaced && next != "" && !isTrimByte(next[0]) {
			body = append(body, ' ')
		}
		body = append(body, next...)
	}
	return body
}

// splitAtWordTimestamps leaves a word timestamp that time.Duration cannot hold in the text, so PlainText drops it.
func splitAtWordTimestamps(text string) (parts []string, times []time.Duration) {
	copied := 0
	for _, loc := range wordTimestampRegexp.FindAllStringIndex(text, -1) {
		if d, ok := markupcore.WordTimestamp(text[loc[0]:loc[1]]); ok {
			parts = append(parts, markupcore.PlainText(text[copied:loc[0]]))
			times = append(times, d)
			copied = loc[1]
		}
	}
	return append(parts, markupcore.PlainText(text[copied:])), times
}

func isTrimByte(b byte) bool {
	return strings.IndexByte(markupcore.PHPTrimSet, b) >= 0
}

func appendStamp(body []byte, d time.Duration) []byte {
	cs := (max(d, 0).Round(time.Millisecond).Milliseconds() + 5) / 10
	return fmt.Appendf(body, "%02d:%02d.%02d", cs/6000, cs/100%60, cs%100)
}
