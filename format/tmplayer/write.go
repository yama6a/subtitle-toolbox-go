package tmplayer

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Write writes s as TMPlayer. A cue without visible text is not written. After a cue that ends before the next
// one starts, it writes an entry without text at the end of the cue, except after the last cue.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	var cues []model.Cue
	var texts [][]string
	for _, c := range s.All() {
		if lines := markupcore.PlainLines(c.Lines); len(lines) > 0 {
			cues = append(cues, c)
			texts = append(texts, lines)
		}
	}

	var body []byte
	for i, c := range cues {
		start := writtenSeconds(c.Start)
		body = appendTime(body, start)
		body = append(body, strings.Join(texts[i], "|")...)
		body = append(body, '\n')
		if i+1 == len(cues) {
			continue
		}
		if end := max(writtenSeconds(c.End), start+1); end < writtenSeconds(cues[i+1].Start) {
			body = appendTime(body, end)
			body = append(body, '\n')
		}
	}
	return writing.Write(w, model.TMPlayer, body, common, false)
}

// writtenSeconds rounds d to whole seconds, half away from zero on whole milliseconds. A time below 0 is 0.
func writtenSeconds(d time.Duration) int64 {
	return (max(d, 0).Round(time.Millisecond).Milliseconds() + 500) / 1000
}

func appendTime(body []byte, seconds int64) []byte {
	return fmt.Appendf(body, "%02d:%02d:%02d:", seconds/3600, seconds%3600/60, seconds%60)
}
