package mkv

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

var (
	blankLines   = regexp.MustCompile(`\n(?:[ \t]*\n)+`)
	vttTimestamp = regexp.MustCompile(`<(?:(\d+):)?(\d{2}):(\d{2})\.(\d{3})>`)
)

const (
	assBlockFields = 9 // ReadOrder, Layer, Style, Name, MarginL, MarginR, MarginV, Effect and Text
	trimSet        = " \t\n\r\x00\x0B"
)

// clock writes a time as hh:mm:ss with the separator sep before the milliseconds. The hours have at least hourDigits digits.
func clock(ms int64, sep string, hourDigits int) string {
	ms = max(ms, 0)
	return fmt.Sprintf("%0*d:%02d:%02d%s%03d", hourDigits, ms/3_600_000, ms/60_000%60, ms/1000%60, sep, ms%1000)
}

// cueText removes the empty lines that would end the cue in a text file.
func cueText(data string) string {
	return blankLines.ReplaceAllString(strings.Trim(markupcore.NormalizeLineEnds(data), "\n"), "\n")
}

// subRipFile rebuilds a SubRip file from the blocks.
func subRipFile(blocks []timedBlock) string {
	var b strings.Builder
	for i, blk := range blocks {
		fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, clock(blk.startMs, ",", 2), clock(blk.endMs, ",", 2), cueText(blk.data))
	}
	return b.String()
}

type assEvent struct {
	order int
	line  string
}

// assFile rebuilds an ASS or SSA file as mkvextract writes it: CodecPrivate, then one Dialogue line per block in
// ReadOrder. Each line takes its fields in the order of the Format line of the header.
func assFile(ssa bool, codecPrivate string, blocks []timedBlock) string {
	header, format := assHeader(ssa, codecPrivate)
	events := make([]assEvent, len(blocks))
	for i, blk := range blocks {
		events[i] = newAssEvent(format, blk)
	}
	slices.SortStableFunc(events, func(a, b assEvent) int { return cmp.Compare(a.order, b.order) })
	var sb strings.Builder
	sb.WriteString(header)
	for _, e := range events {
		sb.WriteString(e.line + "\n")
	}
	return sb.String()
}

// assHeader returns the header of the file and the names of the Format line of its events.
func assHeader(ssa bool, codecPrivate string) (string, []string) {
	header := strings.TrimRight(markupcore.NormalizeLineEnds(strings.TrimPrefix(codecPrivate, "\xEF\xBB\xBF")), trimSet) + "\n"
	format := strings.Split("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", ", ")
	if ssa {
		format[0] = "Marked"
	}
	lines := strings.Split(header, "\n")
	if !slices.ContainsFunc(lines, isEventsLine) {
		return header + "\n[Events]\nFormat: " + strings.Join(format, ", ") + "\n", format
	}
	if names, ok := eventsFormat(lines); ok {
		format = names
	}
	return header, format
}

func newAssEvent(format []string, blk timedBlock) assEvent {
	fields := strings.SplitN(blk.data, ",", assBlockFields)
	for len(fields) < assBlockFields {
		fields = append(fields, "")
	}
	values := make([]string, len(format))
	for i, name := range format {
		values[i] = assValue(strings.ToLower(name), fields, blk)
	}
	return assEvent{order: leadingInt(fields[0]), line: "Dialogue: " + strings.Join(values, ",")}
}

// assValue returns the value of the Format column key for a block with the given fields.
func assValue(key string, fields []string, blk timedBlock) string {
	switch key {
	case "start":
		return clock(blk.startMs, ".", 1)
	case "end":
		return clock(blk.endMs, ".", 1)
	case "marked":
		if strings.HasPrefix(fields[1], "Marked=") {
			return fields[1]
		}
		return "Marked=" + cmp.Or(fields[1], "0")
	case "text":
		return strings.ReplaceAll(markupcore.NormalizeLineEnds(fields[8]), "\n", `\N`)
	}
	if i, ok := assFieldIndex(key); ok {
		return fields[i]
	}
	return ""
}

func assFieldIndex(key string) (int, bool) {
	switch key {
	case "layer":
		return 1, true
	case "style":
		return 2, true
	case "name", "actor":
		return 3, true
	case "marginl":
		return 4, true
	case "marginr":
		return 5, true
	case "marginv":
		return 6, true
	case "effect":
		return 7, true
	}
	return 0, false
}

func isEventsLine(line string) bool {
	return strings.EqualFold(strings.TrimRight(line, " \t"), "[Events]")
}

// eventsFormat returns the names of the first Format line that follows an [Events] line and precedes the next
// line that starts with "[".
func eventsFormat(lines []string) ([]string, bool) {
	for i, line := range lines {
		if !isEventsLine(line) {
			continue
		}
		if names, ok := formatAfter(lines[i+1:]); ok {
			return names, true
		}
	}
	return nil, false
}

func formatAfter(lines []string) ([]string, bool) {
	for _, line := range lines {
		if strings.HasPrefix(line, "[") {
			return nil, false
		}
		if len(line) >= 7 && strings.EqualFold(line[:7], "Format:") {
			names := strings.Split(line[7:], ",")
			for i := range names {
				names[i] = strings.TrimSpace(names[i])
			}
			return names, true
		}
	}
	return nil, false
}

// leadingInt reads the integer at the start of s after white space, 0 when there is none, as PHP casts a string.
func leadingInt(s string) int {
	s = strings.TrimLeft(s, " \t\n\r\v\f")
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	n, _ := strconv.Atoi(s[:end])
	return n
}

// webVTTFile rebuilds a WebVTT file. A track of the codec S_TEXT/WEBVTT holds the settings, the identifier and the
// comments of a cue in the BlockAdditional. A WebM track holds the identifier, the settings and the text in the block.
func webVTTFile(webm bool, codecPrivate string, blocks []timedBlock) string {
	file := "WEBVTT"
	if codecPrivate != "" {
		file = strings.TrimRight(markupcore.NormalizeLineEnds(codecPrivate), trimSet)
	}
	var b strings.Builder
	b.WriteString(file)
	for _, blk := range blocks {
		b.WriteString(webVTTCue(webm, blk))
	}
	b.WriteString("\n")
	return b.String()
}

func webVTTCue(webm bool, blk timedBlock) string {
	settings, identifier, comments := split3(markupcore.NormalizeLineEnds(blk.additional))
	data := blk.data
	if webm {
		identifier, settings, data = split3(markupcore.NormalizeLineEnds(data))
	}
	var b strings.Builder
	b.WriteString("\n\n")
	if strings.TrimSpace(comments) != "" {
		b.WriteString(strings.TrimRight(comments, trimSet) + "\n\n")
	}
	if identifier != "" {
		b.WriteString(identifier + "\n")
	}
	b.WriteString(clock(blk.startMs, ".", 2) + " --> " + clock(blk.endMs, ".", 2))
	if settings != "" {
		b.WriteString(" " + settings)
	}
	b.WriteString("\n" + absoluteTimestamps(cueText(data), blk.startMs))
	return b.String()
}

// absoluteTimestamps turns the timestamps in the text, which count from the cue start, into absolute times.
func absoluteTimestamps(text string, startMs int64) string {
	return vttTimestamp.ReplaceAllStringFunc(text, func(m string) string {
		p := vttTimestamp.FindStringSubmatch(m)
		h, _ := strconv.ParseInt(p[1], 10, 64)
		mm, _ := strconv.ParseInt(p[2], 10, 64)
		ss, _ := strconv.ParseInt(p[3], 10, 64)
		ms, _ := strconv.ParseInt(p[4], 10, 64)
		return "<" + clock(startMs+(h*3600+mm*60+ss)*1000+ms, ".", 2) + ">"
	})
}

// split3 splits s at the first 2 line feeds into 3 parts. Missing parts are empty.
func split3(s string) (a, b, c string) {
	p := strings.SplitN(s, "\n", 3)
	for len(p) < 3 {
		p = append(p, "")
	}
	return p[0], p[1], p[2]
}
