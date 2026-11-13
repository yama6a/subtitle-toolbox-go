package transcript

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/paragraphs"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// WriteHTML writes s as an HTML transcript. It groups the Podcasting 2.0 segments of s, one per cue and speaker,
// into paragraphs. A new speaker or a gap of opts.HTML.ParagraphGap after the latest end so far starts a paragraph.
// Each paragraph has a cite when its first segment has a speaker, a time and a p.
func WriteHTML(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.HTMLTranscript, common)
	if err != nil {
		return err
	}
	if err := opts.HTML.Validate(); err != nil {
		return err
	}
	segments := podcastSegments(s)
	items := make([]paragraphs.Item[podcastSegment], len(segments))
	for i, seg := range segments {
		items[i] = paragraphs.Item[podcastSegment]{Start: seg.start, End: seg.end, Value: seg}
	}
	newSpeaker := func(previous, seg podcastSegment) bool { return previous.speaker != seg.speaker }

	var b strings.Builder
	for _, p := range paragraphs.ByGap(items, opts.HTML.gap(), newSpeaker) {
		if name := p.Values[0].speaker; name != "" {
			b.WriteString("<cite>" + markupcore.EscapeText(name) + ":</cite>\n")
		}
		bodies := make([]string, len(p.Values))
		for i, seg := range p.Values {
			bodies[i] = seg.body
		}
		b.WriteString("<time>" + shortClock(p.Start) + "</time>\n")
		b.WriteString("<p>" + markupcore.EscapeText(strings.Join(bodies, " ")) + "</p>\n")
	}
	return writing.Write(w, model.HTMLTranscript, []byte(b.String()), common, false)
}

// shortClock writes d floored to whole seconds, a negative d as 0: m:ss below 1 hour, else h:mm:ss.
func shortClock(d time.Duration) string {
	n := int64(max(d, 0) / time.Second)
	h, m, sec := n/3600, n/60%60, n%60
	pad := func(v int64) string { return strconv.FormatInt(100+v, 10)[1:] }
	if h == 0 {
		return strconv.FormatInt(m, 10) + ":" + pad(sec)
	}
	return strconv.FormatInt(h, 10) + ":" + pad(m) + ":" + pad(sec)
}
