package subviewer

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const endScript = "******** END SCRIPT ********"

// tag is a header tag and its value.
type tag struct{ key, value string }

// defaultTags returns the tags that Subtitle Edit writes, for a subtitle that has no stored header.
func defaultTags(version Version) []tag {
	last := tag{"COMMENT", ""}
	if version == V1 {
		last = tag{"BEGIN", ""}
	}
	return []tag{{"SOURCE", ""}, {"PRG", ""}, {"FILEPATH", ""}, {"DELAY", "0"}, {"CD TRACK", "0"}, last}
}

// Write writes s as SubViewer 1 or 2. The header holds the title and the author, then the stored header tags, or the
// tags that Subtitle Edit writes when s has none. A CR or LF in a header value becomes a space, and a stored tag
// that the reader would read as another tag is not written, so the header always reads back. A cue without visible
// text is not written, because an empty line ends a cue.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	data, _ := s.FormatData(formatDataKey)
	version := opts.Version
	if version == VersionDefault {
		version = V2
		if n, ok := data.Number("version"); ok && n == 1 {
			version = V1
		}
	}
	tags := headerTags(s, data, version)
	var body []byte
	if version == V1 {
		body = appendVersion1(body, s, tags)
	} else {
		body = appendVersion2(body, s, tags, data)
	}
	return writing.Write(w, model.SubViewer, body, common, false)
}

// headerTags returns the title, the author, then the stored header or the default tags.
func headerTags(s *model.Subtitle, data *model.Data, version Version) []tag {
	title, _ := s.Metadata(model.MetadataTitle)
	author, _ := s.Metadata(model.MetadataAuthor)
	tags := []tag{{"TITLE", title}, {"AUTHOR", author}}
	stored, ok := storedHeader(data)
	if !ok {
		stored = defaultTags(version)
	}
	for _, t := range stored {
		if t.key != "TITLE" && t.key != "AUTHOR" {
			tags = append(tags, t)
		}
	}
	return tags
}

// storedHeader returns the stored header tags. A header stored as a list has the keys 0, 1 and so on, as in PHP.
func storedHeader(data *model.Data) ([]tag, bool) {
	v, ok := data.Get("header")
	if !ok {
		return nil, false
	}
	var tags []tag
	switch h := v.(type) {
	case *model.Data:
		for _, k := range h.Keys() {
			value, _ := h.String(k)
			tags = append(tags, tag{k, value})
		}
	case []any:
		for i, item := range h {
			value, _ := item.(string)
			tags = append(tags, tag{fmt.Sprint(i), value})
		}
	default:
		return nil, false
	}
	return tags, true
}

func appendVersion1(body []byte, s *model.Subtitle, tags []tag) []byte {
	for _, t := range tags {
		if !validKey(t.key) {
			continue
		}
		value := lineBreaks.Replace(t.value)
		if t.key == "DELAY" {
			value = "0" // the reader added the delay to the times
		}
		trimmed := strings.Trim(value, markupcore.PHPTrimSet)
		body = append(body, '[')
		body = append(body, t.key...)
		body = append(body, ']')
		switch {
		case value == "":
		case strings.HasPrefix(trimmed, "[") || trimmed == startScript:
			// The reader takes the value on the next line only when it does not start with [, and it
			// starts the script at a line that equals the marker.
			body = append(body, value...)
		default:
			body = append(body, '\n')
			body = append(body, value...)
		}
		body = append(body, '\n')
	}
	body = append(body, startScript+"\n"...)
	for _, c := range s.All() {
		lines := markupcore.PlainLines(c.Lines)
		if len(lines) == 0 {
			continue
		}
		body = appendTime1(body, c.Start)
		body = append(body, '\n')
		body = append(body, strings.Join(lines, "|")...)
		body = append(body, '\n')
		body = appendTime1(body, c.End)
		body = append(body, "\n\n"...)
	}
	return append(body, "[end]\n"+endScript+"\n"...)
}

func appendVersion2(body []byte, s *model.Subtitle, tags []tag, data *model.Data) []byte {
	body = append(body, "[INFORMATION]\n"...)
	for _, t := range tags {
		if !validKey(t.key) || isBlockTag(t.key) {
			continue
		}
		line := "[" + t.key + "]" + lineBreaks.Replace(t.value)
		if !readsAsHeader(line) {
			line = "[" + t.key + "]"
		}
		if readsAsHeader(line) {
			body = append(body, line...)
			body = append(body, '\n')
		}
	}
	body = append(body, "[END INFORMATION]\n[SUBTITLE]\n"...)
	if style, ok := data.String("style"); ok {
		style = strings.Trim(lineBreaks.Replace(style), markupcore.PHPTrimSet)
		if isStyleLine(style) && !looksLikeTimingLine(style) {
			body = append(body, style...)
			body = append(body, '\n')
		}
	}
	return appendBlocks2(body, s)
}

func appendBlocks2(body []byte, s *model.Subtitle) []byte {
	first := true
	for _, c := range s.All() {
		lines := markupcore.PlainLines(c.Lines)
		if len(lines) == 0 {
			continue
		}
		if !first {
			body = append(body, '\n')
		}
		first = false
		body = appendTime2(body, c.Start)
		body = append(body, ',')
		body = appendTime2(body, c.End)
		body = append(body, '\n')
		body = append(body, strings.Join(lines, "[br]")...)
		body = append(body, '\n')
	}
	return body
}

var lineBreaks = strings.NewReplacer("\r", " ", "\n", " ") //nolint:gochecknoglobals // a fixed replacer

// validKey reports whether the reader reads the tag key back as the same key: 1 or more characters without ], no
// white space at the ends, and no lower-case ASCII letters.
func validKey(key string) bool {
	return key != "" && !strings.ContainsAny(key, "]\r\n") && key == strings.Trim(key, markupcore.PHPTrimSet) && key == asciiUpper(key)
}

// readsAsHeader reports whether the reader takes line as a header line of SubViewer 2. It does not when the line is a
// style line or looks like a timing line.
func readsAsHeader(line string) bool {
	line = strings.Trim(line, markupcore.PHPTrimSet)
	return !isStyleLine(line) && !looksLikeTimingLine(line)
}

func appendTime1(body []byte, d time.Duration) []byte {
	n := timecode.Units(d, time.Second)
	return fmt.Appendf(body, "[%02d:%02d:%02d]", n/3600, n/60%60, n%60)
}

func appendTime2(body []byte, d time.Duration) []byte {
	n := timecode.Units(d, 10*time.Millisecond)
	return fmt.Appendf(body, "%02d:%02d:%02d.%02d", n/360000, n/6000%60, n/100%60, n%100)
}
