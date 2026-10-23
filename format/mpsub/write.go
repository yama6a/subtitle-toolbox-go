package mpsub

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	defaultType = "VIDEO"
	defaultNote = "Created with subtitle-toolbox-go (https://github.com/yama6a/subtitle-toolbox-go)"
)

// Write writes s as MPSub. The header holds the title and the author, then the stored keys, then FORMAT and NOTE.
// A stored key that is not ASCII capitals is not written, and a CR or LF in a header value becomes a space, so the
// output always reads back. A cue without visible text is not written. The FORMAT line is TIME, or the frame rate of opts.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	body := appendHeader(nil, s, opts.FrameRate)
	var previousEnd time.Duration
	for _, c := range s.All() {
		lines := markupcore.PlainLines(c.Lines)
		if len(lines) == 0 {
			continue
		}
		start, end := max(c.Start, 0), max(c.End, 0)
		body = append(body, '\n')
		body = appendNumber(body, start, previousEnd, opts.FrameRate)
		body = append(body, ' ')
		body = appendNumber(body, end, start, opts.FrameRate)
		body = append(body, '\n')
		body = append(body, strings.Join(lines, "\n")...)
		body = append(body, '\n')
		previousEnd = end
	}
	return writing.Write(w, model.MPSub, body, common, true)
}

func appendHeader(body []byte, s *model.Subtitle, fps float64) []byte {
	data, _ := s.FormatData(formatDataKey)
	title, _ := s.Metadata(model.MetadataTitle)
	author, _ := s.Metadata(model.MetadataAuthor)
	fileType, ok := data.String("TYPE")
	if !ok {
		fileType = defaultType
	}
	body = appendKey(body, "TITLE", title)
	body = appendKey(body, "AUTHOR", author)
	body = appendKey(body, "TYPE", fileType)
	for _, key := range data.Keys() {
		switch key {
		case "TITLE", "AUTHOR", "TYPE", "FORMAT", "NOTE":
		default:
			if isHeaderKey(key) {
				value, _ := data.String(key)
				body = appendKey(body, key, value)
			}
		}
	}
	format := "TIME"
	if fps != 0 {
		format = strconv.FormatFloat(fps, 'f', 0, 64)
	}
	body = appendKey(body, "FORMAT", format)
	note, ok := data.String("NOTE")
	if !ok {
		note = defaultNote
	}
	return appendKey(body, "NOTE", note)
}

// appendKey writes a header line. A CR or LF in the value becomes a space, so that the line stays one line.
func appendKey(body []byte, key, value string) []byte {
	body = append(body, key...)
	body = append(body, '=')
	body = append(body, lineBreaks.Replace(value)...)
	return append(body, '\n')
}

var lineBreaks = strings.NewReplacer("\r", " ", "\n", " ") //nolint:gochecknoglobals // a fixed replacer

// isHeaderKey reports whether key is 1 or more ASCII capitals, the only keys that the reader reads as headers.
func isHeaderKey(key string) bool {
	return key != "" && strings.Trim(key, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") == ""
}

// appendNumber appends the time from "from" to "to" as seconds, or as frames at fps frames per second. The frames
// round from the absolute times, so the rounding errors of the cues do not add up.
func appendNumber(body []byte, to, from time.Duration, fps float64) []byte {
	if fps == 0 {
		return append(body, timecode.Seconds(to-from)...)
	}
	return strconv.AppendFloat(body, timecode.Frames(to, fps)-timecode.Frames(from, fps), 'f', 0, 64)
}
