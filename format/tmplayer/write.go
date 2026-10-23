package tmplayer

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
		start := timecode.Units(c.Start, time.Second)
		body = appendTime(body, start)
		body = append(body, strings.Join(texts[i], "|")...)
		body = append(body, '\n')
		if i+1 == len(cues) {
			continue
		}
		if end := max(timecode.Units(c.End, time.Second), start+1); end < timecode.Units(cues[i+1].Start, time.Second) {
			body = appendTime(body, end)
			body = append(body, '\n')
		}
	}
	return writing.Write(w, model.TMPlayer, body, common, false)
}

func appendTime(body []byte, seconds int64) []byte {
	return fmt.Appendf(body, "%02d:%02d:%02d:", seconds/3600, seconds%3600/60, seconds%60)
}
