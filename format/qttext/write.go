package qttext

import (
	"fmt"
	"io"
	"math/bits"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// defaultHeader is the header that Subtitle Edit writes. It reads only 2 fraction digits.
//
//nolint:gochecknoglobals // fixed header lines
var defaultHeader = []string{
	"{QTtext} {font:Tahoma}",
	"{plain} {size:20}",
	"{timeScale:30}",
	"{width:160} {height:32}",
	"{timestamps:absolute} {language:0}",
}

//nolint:gochecknoglobals // fixed pattern and replacer
var (
	styleTagRegexp = regexp.MustCompile(`^<(/?)([biuBIU])>$`)
	lineBreaks     = strings.NewReplacer("\r", " ", "\n", " ")
)

type writer struct {
	body      []byte
	perSecond uint64
	digits    int
	relative  bool
	style     style
	// position is the time that a reader has after the relative timestamps written so far.
	position time.Duration
}

// count is a time in whole seconds and units of the time scale.
type count struct {
	seconds, units uint64
}

// Write writes s as QuickTime text. The header is the stored header, or the header of Subtitle Edit with the paired
// layout for a new file. A CR or LF in a stored header line becomes a space, a stored header line that reads as a
// timestamp is not written, and a header that does not start with {QTtext} gets that line first, so the output
// always reads back. The style descriptors of a cue without stored source text come from the markup. Such a text
// line that reads as a timestamp line starts with U+2060 WORD JOINER.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	file, _ := s.FormatData(formatDataKey)
	stored, hasHeader := file.Strings("header")
	paired, _ := file.Bool("pairedTimestamps")
	paired = paired || !hasHeader
	header := defaultHeader
	if hasHeader {
		header = writableHeader(stored)
	}

	wr := &writer{perSecond: uint64(timeScale(header)), relative: isRelative(header)}
	if wr.perSecond == 0 {
		wr.perSecond = 1000
	}
	wr.digits = len(strconv.FormatUint(wr.perSecond-1, 10))
	readLines(header, &wr.style)
	for _, line := range header {
		wr.body = append(wr.body, line...)
		wr.body = append(wr.body, '\n')
	}
	wr.cues(s, paired)
	return writing.Write(w, model.QuickTimeText, wr.body, common, false)
}

func (wr *writer) cues(s *model.Subtitle, paired bool) {
	// QuickTime writes samples from the start of the track, so a gap before the first cue is an empty sample.
	var previous time.Duration
	started := false
	if first, ok := s.Cue(0); ok && !paired && first.Start > 0 {
		wr.timestamp(0)
		started = true
	}
	for _, c := range s.All() {
		start := max(0, c.Start)
		end := max(start, c.End)
		if paired || !started || wr.count(start) != wr.count(previous) {
			wr.timestamp(start)
		}
		started = true
		for _, line := range wr.textLines(c) {
			wr.body = append(wr.body, line...)
			wr.body = append(wr.body, '\n')
		}
		wr.timestamp(end)
		if paired {
			wr.body = append(wr.body, '\n')
		}
		previous = end
	}
}

func writableHeader(stored []string) []string {
	var header []string
	for _, line := range stored {
		if line = lineBreaks.Replace(line); !isTimestampLine(line) {
			header = append(header, line)
		}
	}
	if len(header) == 0 || !headerRegexp.MatchString(header[0]) {
		header = slices.Insert(header, 0, "{QTtext}")
	}
	return header
}

// count returns d in units of the time scale. Up to 1000 units per second it rounds the float seconds times the scale,
// as PHP 8.5 and the MicroDVD and MPSub writers do, so a time on an exact half unit can round down. Above that, floats
// lose milliseconds, so it rounds the exact milliseconds half away from zero. A time at the limit stops at the largest
// count whose time a reader reads below the limit.
func (wr *writer) count(d time.Duration) count {
	var c count
	if wr.perSecond <= 1000 {
		n := uint64(timecode.Frames(d, float64(wr.perSecond)))
		c = count{n / wr.perSecond, n % wr.perSecond}
	} else {
		c = wr.exactCount(d)
	}
	if wr.read(c) >= timecode.MaxTime {
		c = wr.previous(c)
	}
	return c
}

func (wr *writer) exactCount(d time.Duration) count {
	ms := uint64(max(min(d, timecode.MaxValidTime), 0).Milliseconds())
	hi, lo := bits.Mul64(ms%1000, wr.perSecond)
	units, rem := bits.Div64(hi, lo, 1000)
	if rem >= 500 {
		units++
	}
	if units == wr.perSecond {
		return count{ms/1000 + 1, 0}
	}
	return count{ms / 1000, units}
}

// read returns the time that a reader reads for c.
func (wr *writer) read(c count) time.Duration {
	return time.Duration(c.seconds*1000+scaledMillis(c.units, wr.perSecond)) * time.Millisecond
}

// previous returns c less one unit.
func (wr *writer) previous(c count) count {
	if c.units > 0 {
		return count{c.seconds, c.units - 1}
	}
	return count{c.seconds - 1, wr.perSecond - 1}
}

// timestamp writes the time at. A relative timestamp counts from the time that the reader has after the timestamps
// before it, so rounding errors and overlapping cues do not move the cues after them.
func (wr *writer) timestamp(at time.Duration) {
	var c count
	if wr.relative {
		c = wr.relativeCount(at)
	} else {
		c = wr.count(at)
	}
	wr.body = fmt.Appendf(wr.body, "[%02d:%02d:%02d.%0*d]\n", c.seconds/3600, c.seconds/60%60, c.seconds%60, wr.digits, c.units)
}

// relativeCount returns the count from the position of the reader to at, and moves the position. The reader adds
// the timestamps, so near the time limit the count shrinks until the sum stays below the limit.
func (wr *writer) relativeCount(at time.Duration) count {
	c := wr.count(at - wr.position)
	if wr.position+wr.read(c) >= timecode.MaxTime {
		c = wr.count(timecode.MaxValidTime - wr.position)
		if wr.position+wr.read(c) >= timecode.MaxTime {
			c = wr.previous(c)
		}
	}
	wr.position += wr.read(c)
	return c
}

func (wr *writer) textLines(c model.Cue) []string {
	data, _ := c.FormatData(formatDataKey)
	if source, ok := data.Strings("source"); ok {
		running := wr.style
		if slices.Equal(readLines(source, &running), c.Lines) {
			wr.style = running
			return source
		}
	}

	lines := make([]string, len(c.Lines))
	for i, l := range c.Lines {
		lines[i] = markupcore.RubyAsText(l)
	}
	var out []string
	for _, line := range markupcore.CarryOpenTags(lines) {
		text := wr.markupLine(line)
		switch {
		case text == "":
			continue
		case isTimestampLine(text):
			text = writing.WordJoiner + text
		}
		out = append(out, text)
	}
	return out
}

func (wr *writer) markupLine(line string) string {
	var open style
	var text strings.Builder
	for i, token := range markupcore.SplitTags(line) {
		if i%2 == 1 {
			open = open.after(token)
			continue
		}
		token = markupcore.DecodeEntities(token)
		if strings.Trim(token, markupcore.PHPTrimSet) != "" {
			text.WriteString(descriptors(wr.style, open))
			wr.style = open
		}
		text.WriteString(token)
	}
	return strings.Trim(text.String(), markupcore.PHPTrimSet)
}

func (st style) after(tag string) style {
	m := styleTagRegexp.FindStringSubmatch(tag)
	if m == nil {
		return st
	}
	on := m[1] == ""
	switch markupcore.ASCIILower(m[2]) {
	case "b":
		st.bold = on
	case "i":
		st.italic = on
	case "u":
		st.underline = on
	}
	return st
}

// descriptors returns the descriptors that change from to to. Only {plain} turns a style off.
func descriptors(from, to style) string {
	if from.turnsOff(to) {
		return "{plain}" + to.turnsOn(style{})
	}
	return to.turnsOn(from)
}

func (st style) turnsOff(to style) bool {
	return (st.bold && !to.bold) || (st.italic && !to.italic) || (st.underline && !to.underline)
}

// turnsOn returns the descriptors of the styles that st has and base has not.
func (st style) turnsOn(base style) string {
	var b strings.Builder
	if st.bold && !base.bold {
		b.WriteString("{bold}")
	}
	if st.italic && !base.italic {
		b.WriteString("{italic}")
	}
	if st.underline && !base.underline {
		b.WriteString("{underline}")
	}
	return b.String()
}
