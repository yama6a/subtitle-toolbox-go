package sbv

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Write writes s as SBV. It skips a cue without text, because an empty line would end the SBV cue.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var body []byte
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
		body = appendTime(body, c.Start)
		body = append(body, ',')
		body = appendTime(body, c.End)
		body = append(body, '\n')
		body = append(body, strings.Join(lines, "\n")...)
		body = append(body, '\n')
	}
	return writing.Write(w, model.SBV, body, common, false)
}

// appendTime appends d as h:mm:ss.mmm, with more hour digits from 10 hours. A negative time writes as 0.
func appendTime(dst []byte, d time.Duration) []byte {
	ms := max(d.Round(time.Millisecond).Milliseconds(), 0)
	dst = strconv.AppendInt(dst, ms/int64(time.Hour/time.Millisecond), 10)
	dst = append(dst, ':')
	dst = appendPadded(dst, ms/int64(time.Minute/time.Millisecond)%60, 2)
	dst = append(dst, ':')
	dst = appendPadded(dst, ms/int64(time.Second/time.Millisecond)%60, 2)
	dst = append(dst, '.')
	return appendPadded(dst, ms%1000, 3)
}

func appendPadded(dst []byte, n int64, width int) []byte {
	s := strconv.FormatInt(n, 10)
	for range width - len(s) {
		dst = append(dst, '0')
	}
	return append(dst, s...)
}
