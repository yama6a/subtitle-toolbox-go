package subrip

import (
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// maxTime is 999:59:59.999. The SubRip reader takes at most 3 hour digits, so a later time would not read back.
const maxTime = 1000*time.Hour - time.Millisecond

// AppendCue appends the block of cue c with cue number n, which ends with LF.
func AppendCue(dst []byte, n int, c model.Cue, stripTags bool) []byte {
	dst = strconv.AppendInt(dst, int64(n), 10)
	dst = append(dst, '\n')
	dst = timecode.AppendClock(dst, min(c.Start, maxTime), 2, ',')
	dst = append(dst, " --> "...)
	dst = timecode.AppendClock(dst, min(c.End, maxTime), 2, ',')
	dst = appendCoordinates(dst, c)
	if text := cueLines(c, stripTags); text != "" {
		dst = append(dst, '\n')
		dst = append(dst, text...)
	}
	return append(dst, '\n')
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
		if strings.Trim(line, markupcore.PHPTrimSet) == "" {
			continue
		}
		if isLenientTimingLine(line) {
			line = writing.WordJoiner + line
		}
		kept = append(kept, line)
	}
	text = strings.Join(kept, "\n")
	if n := c.Alignment.Numpad(); n != model.BottomCenter.Numpad() {
		text = `{\an` + strconv.Itoa(n) + "}" + text
	}
	return text
}
