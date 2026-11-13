package youtube

import (
	"fmt"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// caption is one caption of the source: a json3 event, a srv3 paragraph or a srv1 or srv2 text.
type caption struct {
	timing
	isAppend  bool
	alignment model.Alignment
	segments  []segment
	// data is the cue format data. source is what an append caption adds to the format data of the next cue.
	data, source *model.Data
}

// timing is what the end rule needs of a caption. A window is "" for none.
type timing struct {
	start, end time.Duration
	hasEnd     bool
	window     string
	hasWindow  bool
}

// endsOf returns the end of each caption in one backward pass. A caption without an end ends at the first later
// start that is greater than its own, else lasts last. A caption in a window ends at most at the first later start
// of that window that is greater than its own. Each stack holds the starts after the caption, from the first
// later start that is greater at the top.
func endsOf(ts []timing, last time.Duration) []time.Duration {
	ends := make([]time.Duration, len(ts))
	var all []time.Duration
	byWindow := map[string][]time.Duration{}
	for i := len(ts) - 1; i >= 0; i-- {
		t := ts[i]
		all = popNotAfter(all, t.start)
		end := t.end
		if !t.hasEnd {
			end = parsing.EndAfter(t.start, last)
			if len(all) > 0 {
				end = all[len(all)-1]
			}
		}
		if t.hasWindow {
			stack := popNotAfter(byWindow[t.window], t.start)
			if len(stack) > 0 {
				end = min(end, stack[len(stack)-1])
			}
			byWindow[t.window] = append(stack, t.start)
		}
		all = append(all, t.start)
		ends[i] = end
	}
	return ends
}

func popNotAfter(stack []time.Duration, start time.Duration) []time.Duration {
	for len(stack) > 0 && stack[len(stack)-1] <= start {
		stack = stack[:len(stack)-1]
	}
	return stack
}

const asciiSpace = " \t\n\v\f\r"

// captionLines returns the lines of the cue markup of the segments of a caption. With wordTimes and at least one
// segment with an offset, each segment with text gets a word timestamp tag.
func captionLines(segs []segment, start time.Duration, wordTimes bool) []string {
	timed := false
	if wordTimes {
		for _, s := range segs {
			if s.hasOffset {
				timed = true
				break
			}
		}
	}
	var b strings.Builder
	for _, s := range segs {
		if !timed {
			b.WriteString(styled(s.text, s.style))
			continue
		}
		rest := strings.TrimLeft(s.text, asciiSpace)
		b.WriteString(s.text[:len(s.text)-len(rest)])
		if rest != "" {
			b.WriteString("<" + markupcore.CoreTimestamp(start+s.offset) + ">" + styled(rest, s.style))
		}
	}
	return strings.Split(b.String(), "\n")
}

func styled(text string, st style) string {
	lines := strings.Split(markupcore.NormalizeLineEnds(text), "\n")
	for i, line := range lines {
		lines[i] = markupcore.StyleRuns([]markupcore.Run{{
			Text: line, Color: st.color, Bold: st.bold, Italic: st.italic, Underline: st.underline,
		}}, true)
	}
	return strings.Join(lines, "\n")
}

// moveAppends removes the append captions from caps. Each goes to the format data of the next caption that is
// not an append, and the ones after the last caption go to file.
func moveAppends(caps []caption, file *model.Data) []caption {
	var kept []caption
	var appends []any
	for _, c := range caps {
		if c.isAppend {
			appends = append(appends, c.source)
			continue
		}
		if len(appends) > 0 {
			if c.data == nil {
				c.data = model.NewData()
			}
			c.data.Set("appends", appends)
			appends = nil
		}
		kept = append(kept, c)
	}
	if len(appends) > 0 {
		file.Set("appends", appends)
	}
	return kept
}

// assemble sets the ends and builds the subtitle.
func assemble(st *parsing.State, caps []caption, file *model.Data, wordTimes bool) (*model.Subtitle, error) {
	kept := moveAppends(caps, file)
	ts := make([]timing, len(kept))
	for i, c := range kept {
		ts[i] = c.timing
	}
	ends := endsOf(ts, st.LastCueDuration())

	s := &model.Subtitle{}
	if err := s.SetFormatData(key, file); err != nil {
		return nil, fmt.Errorf("%s file data: %w", key, err)
	}
	cues := make([]model.Cue, 0, len(kept))
	for i, c := range kept {
		cue, ok, err := c.cue(ends[i], wordTimes)
		if err != nil {
			return nil, err
		}
		if ok {
			cues = append(cues, cue)
		}
	}
	s.AddCues(cues...)
	return s, nil
}

// cue returns the cue of a caption. A caption without lines gives none.
func (c caption) cue(end time.Duration, wordTimes bool) (model.Cue, bool, error) {
	lines := markupcore.CleanLines(captionLines(c.segments, c.start, wordTimes))
	if len(lines) == 0 {
		return model.Cue{}, false, nil
	}
	cue := model.Cue{Start: c.start, End: end, Lines: lines}
	cue.Position.Alignment = c.alignment
	if wordTimes {
		cue.ExtractWordTags()
	}
	if err := cue.SetFormatData(key, c.data); err != nil {
		return model.Cue{}, false, fmt.Errorf("%s cue data: %w", key, err)
	}
	return cue, true, nil
}
