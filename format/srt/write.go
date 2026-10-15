package srt

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Write writes s as SubRip.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var body []byte
	for i, c := range s.All() {
		if i > 0 {
			body = append(body, '\n')
		}
		body = appendCue(body, i+1, c, common.StripTags)
	}
	return writing.Write(w, model.SubRip, body, common, true)
}

// appendCue appends the block of cue c with cue number n, which ends with LF.
func appendCue(dst []byte, n int, c model.Cue, stripTags bool) []byte {
	dst = strconv.AppendInt(dst, int64(n), 10)
	dst = append(dst, '\n')
	dst = appendTime(dst, c.Start)
	dst = append(dst, " --> "...)
	dst = appendTime(dst, c.End)
	dst = appendCoordinates(dst, c)
	if text := cueLines(c, stripTags); text != "" {
		dst = append(dst, '\n')
		dst = append(dst, text...)
	}
	return append(dst, '\n')
}

// appendTime appends d as hh:mm:ss,mmm, with more hour digits from 100 hours. A negative time writes as 0.
func appendTime(dst []byte, d time.Duration) []byte {
	ms := max(d.Round(time.Millisecond).Milliseconds(), 0)
	dst = appendPadded(dst, ms/int64(time.Hour/time.Millisecond), 2)
	dst = append(dst, ':')
	dst = appendPadded(dst, ms/int64(time.Minute/time.Millisecond)%60, 2)
	dst = append(dst, ':')
	dst = appendPadded(dst, ms/int64(time.Second/time.Millisecond)%60, 2)
	dst = append(dst, ',')
	return appendPadded(dst, ms%1000, 3)
}

func appendPadded(dst []byte, n int64, width int) []byte {
	s := strconv.FormatInt(n, 10)
	for range width - len(s) {
		dst = append(dst, '0')
	}
	return append(dst, s...)
}

// appendCoordinates appends the coordinates of the srt format data, such as " X1:10 X2:20 Y1:30 Y2:40".
func appendCoordinates(dst []byte, c model.Cue) []byte {
	data, _ := c.FormatData(formatDataKey)
	coordinates, ok := data.Object("coordinates")
	if !ok {
		return dst
	}
	keys := []string{"x1", "x2", "y1", "y2"}
	values := make([]int64, len(keys))
	for i, k := range keys {
		if values[i], ok = coordinates.Int(k); !ok {
			return dst
		}
	}
	for i, k := range keys {
		dst = append(dst, ' ')
		dst = append(dst, strings.ToUpper(k)...)
		dst = append(dst, ':')
		dst = strconv.AppendInt(dst, values[i], 10)
	}
	return dst
}

// cueLines returns the text of c with the tags that SubRip players show, the entities decoded, without blank
// lines, and with an {\anN} tag first for an alignment other than bottom center.
func cueLines(c model.Cue, stripTags bool) string {
	text := markupcore.RubyAsText(strings.Join(c.Lines, "\n"))
	if stripTags {
		text = markupcore.StripAllTags(text)
	} else {
		text = markupcore.KeepTags(markupcore.WebVTTColorsToFont(text), "b", "u", "i", "s", "font")
	}
	text = markupcore.DecodeEntities(text)

	// A line that holds only a tag becomes empty, and an empty line ends a SubRip cue.
	var kept []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.Trim(line, phpTrimSet) != "" {
			kept = append(kept, line)
		}
	}
	text = strings.Join(kept, "\n")
	if n := c.Alignment.Numpad(); n != model.BottomCenter.Numpad() {
		text = `{\an` + strconv.Itoa(n) + "}" + text
	}
	return text
}
