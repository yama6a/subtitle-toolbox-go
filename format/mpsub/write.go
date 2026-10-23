package mpsub

import (
	"io"
	"math"
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
// A cue without visible text is not written. The FORMAT line is TIME, or the frame rate of opts.
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
			value, _ := data.String(key)
			body = appendKey(body, key, value)
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

func appendKey(body []byte, key, value string) []byte {
	body = append(body, key...)
	body = append(body, '=')
	body = append(body, value...)
	return append(body, '\n')
}

// appendNumber appends the time from "from" to "to" as seconds, or as frames at fps frames per second. The frames
// round from the absolute times, so the rounding errors of the cues do not add up.
func appendNumber(body []byte, to, from time.Duration, fps float64) []byte {
	if fps == 0 {
		return append(body, timecode.Seconds(to-from)...)
	}
	return strconv.AppendFloat(body, frames(to, fps)-frames(from, fps), 'f', 0, 64)
}

// frames returns the frame of t, rounded half away from zero. The seconds come from whole milliseconds, as PHP
// holds them. The result stops at the last frame that the reader reads as a time below 100000 hours, because the
// frame of 99999:59:59.999 can round up to 100000 hours.
func frames(t time.Duration, fps float64) float64 {
	return min(math.Round(float64(t.Milliseconds())/1000*fps), lastFrame(fps))
}

// lastFrame returns the largest frame whose time the reader reads as below 100000 hours. The first guess can be
// one frame too high through float rounding.
func lastFrame(fps float64) float64 {
	last := math.Ceil((timecode.MaxTime.Seconds()-0.0005)*fps) - 1
	for range 2 {
		if _, ok := timeOf(last / fps); !ok {
			last--
		}
	}
	return last
}
