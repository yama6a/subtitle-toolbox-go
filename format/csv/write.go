package csv

import (
	"io"
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

const defaultSecondTextHeader = "text2"

var leadingVoiceRegexp = regexp.MustCompile(`^` + markupcore.VoiceTagPattern)

// column is a written column: a role, or the stored cells of the other column with the key.
type column struct {
	role string // a role name, "second" for the SecondText column, or "" for another column
	key  string
}

// settings holds the resolved options of one write.
type settings struct {
	delimiter  string
	timeFormat TimeFormat
	fps        float64
	escape     bool
}

// Write writes s as a CSV or TSV table. A subtitle that Read returned keeps its columns, header names, delimiter,
// time format and quoting. A line break inside a cell stays LF, as Excel writes it. The rows end with the line
// ending of common.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	file, hasFile := s.FormatData(formatDataKey)
	set, err := resolve(file, opts)
	if err != nil {
		return err
	}
	cues := s.Cues()
	speakers := make([]string, len(cues))
	texts := make([]string, len(cues))
	for i, c := range cues {
		speakers[i], texts[i] = splitSpeaker(c.Lines)
	}
	columns, header, quoted := layoutOf(file, hasFile, cues, speakers)
	var seconds []string
	if opts.SecondText != nil {
		name := opts.SecondTextHeader
		if name == "" {
			name = defaultSecondTextHeader
		}
		at := slices.Index(columns, column{role: "text"}) + 1
		if at == 0 {
			at = 1
		}
		at = min(at, len(columns))
		columns = slices.Insert(columns, at, column{role: "second"})
		header = slices.Insert(header, at, name)
		seconds = secondTexts(cues, opts.SecondText.Cues())
	}
	var quotedAt []bool
	for _, c := range columns {
		quotedAt = append(quotedAt, slices.Contains(quoted, c))
	}

	eol := "\n"
	if common.LineEnding == model.CRLF {
		eol = "\r\n"
	}
	var b strings.Builder
	if !hasFile || !isNull(file, "header") {
		set.writeRow(&b, header, nil)
		b.WriteString(eol)
	}
	for i, c := range cues {
		row := make([]string, len(columns))
		for j, col := range columns {
			row[j] = set.cell(c, col, speakers[i], texts[i], seconds, i)
		}
		set.writeRow(&b, row, quotedAt)
		b.WriteString(eol)
	}
	common.LineEnding = model.LF
	return writing.Write(w, model.CSV, []byte(b.String()), common, true)
}

func isNull(d *model.Data, key string) bool {
	v, _ := d.Get(key)
	return v == nil
}

// resolve returns the settings of the options, else of the stored file data, else of the defaults.
func resolve(file *model.Data, opts WriteOptions) (settings, error) {
	set := settings{delimiter: string(opts.Delimiter), timeFormat: opts.TimeFormat, fps: opts.FrameRate, escape: opts.EscapeFormulas}
	if set.delimiter == "" {
		set.delimiter, _ = file.String("delimiter")
	}
	if set.delimiter == "" {
		set.delimiter = string(Comma)
	}
	if set.timeFormat == TimeFormatDefault {
		stored, _ := file.String("timeFormat")
		set.timeFormat = TimeFormat(stored)
	}
	if set.timeFormat == TimeFormatDefault {
		set.timeFormat = TimeDot
	}
	if set.fps == 0 {
		if fps, ok := file.Number("frameRate"); ok {
			if err := checkFrameRate(fps, "WriteOptions.CSV.FrameRate", "The frame rate"); err != nil {
				return settings{}, err
			}
			set.fps = fps
		}
	}
	if set.timeFormat == TimeFrames && set.fps == 0 {
		return settings{}, invalid("WriteOptions.CSV.FrameRate", "The time format hh:mm:ss:ff needs a frame rate.")
	}
	return set, nil
}

// layoutOf returns the columns, the header cells and the columns that every row of the read table quoted. A
// subtitle with csv file data keeps its columns. Other input gets identifier when a cue has an ID, then start,
// end, text and the keys of the stored cue cells. A speaker column goes before text when a cue has a speaker.
func layoutOf(file *model.Data, hasFile bool, cues []model.Cue, speakers []string) (columns []column, header []string, quoted []column) {
	if hasFile {
		columns, header, quoted = storedLayout(file)
	} else {
		columns, header = defaultLayout(cues)
	}
	if slices.ContainsFunc(speakers, func(s string) bool { return s != "" }) && !slices.Contains(columns, column{role: "speaker"}) {
		at := max(slices.Index(columns, column{role: "text"}), 0)
		columns = slices.Insert(columns, at, column{role: "speaker"})
		header = slices.Insert(header, at, "speaker")
	}
	return columns, header, quoted
}

func storedLayout(file *model.Data) (columns []column, header []string, quoted []column) {
	width, _ := file.Int("width")
	storedRoles, _ := file.Get("roles")
	storedHeader, _ := file.Get("header")
	for i := range int(width) {
		key := strconv.Itoa(i)
		name, hasName := item(storedHeader, key)
		if r, ok := roleAt(storedRoles, int64(i)); ok {
			columns = append(columns, column{role: r})
		} else if hasName {
			columns = append(columns, column{key: phpString(name)})
		} else {
			columns = append(columns, column{key: key})
		}
		header = append(header, phpString(name))
	}
	stored, _ := file.Get("quotedColumns")
	for _, v := range values(stored) {
		if i, ok := v.(int64); ok && i < int64(len(columns)) {
			quoted = append(quoted, columns[i])
		}
	}
	return columns, header, quoted
}

// roleAt returns the first stored role whose column index is i.
func roleAt(storedRoles any, i int64) (string, bool) {
	d, ok := storedRoles.(*model.Data)
	if !ok {
		return "", false
	}
	for _, k := range d.Keys() {
		if n, _ := d.Int(k); n == i {
			return k, true
		}
	}
	return "", false
}

func defaultLayout(cues []model.Cue) (columns []column, header []string) {
	if slices.ContainsFunc(cues, func(c model.Cue) bool { return c.ID != "" }) {
		columns, header = append(columns, column{role: "identifier"}), append(header, "identifier")
	}
	for _, r := range []string{"start", "end", "text"} {
		columns, header = append(columns, column{role: r}), append(header, r)
	}
	var seen []string
	for _, c := range cues {
		for _, k := range keys(storedCells(c)) {
			if !slices.Contains(seen, k) {
				seen = append(seen, k)
				columns, header = append(columns, column{key: k}), append(header, k)
			}
		}
	}
	return columns, header
}

// storedCells returns the stored csv.columns of c: a list or an object.
func storedCells(c model.Cue) any {
	d, _ := c.FormatData(formatDataKey)
	v, _ := d.Get("columns")
	return v
}

// item returns the value of a list or an object by the PHP array key, where the key "1" is the list index 1.
func item(v any, key string) (any, bool) {
	switch v := v.(type) {
	case []any:
		i, err := strconv.Atoi(key)
		if err != nil || i < 0 || i >= len(v) || strconv.Itoa(i) != key {
			return nil, false
		}
		return v[i], v[i] != nil
	case *model.Data:
		x, ok := v.Get(key)
		return x, ok && x != nil
	default:
		return nil, false
	}
}

// keys returns the PHP array keys of a list or an object.
func keys(v any) []string {
	switch v := v.(type) {
	case []any:
		out := make([]string, len(v))
		for i := range v {
			out[i] = strconv.Itoa(i)
		}
		return out
	case *model.Data:
		return v.Keys()
	default:
		return nil
	}
}

func values(v any) []any {
	switch v := v.(type) {
	case []any:
		return v
	case *model.Data:
		out := make([]any, 0, v.Len())
		for _, k := range v.Keys() {
			x, _ := v.Get(k)
			out = append(out, x)
		}
		return out
	default:
		return nil
	}
}

// phpString returns a stored scalar as PHP (string) writes it. Nil and false give "", true gives "1".
func phpString(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return timecode.Number(v)
	case bool:
		if v {
			return "1"
		}
		return ""
	default:
		return ""
	}
}

// splitSpeaker returns the name of the leading <v> tag of the first line and the plain text of the lines.
func splitSpeaker(lines []string) (speaker, text string) {
	lines = slices.Clone(lines)
	for i, l := range lines {
		lines[i] = markupcore.RubyAsText(l)
	}
	if len(lines) > 0 {
		if m := leadingVoiceRegexp.FindString(lines[0]); m != "" {
			speaker, _ = markupcore.Speaker(m)
			lines[0] = lines[0][len(m):]
		}
	}
	return speaker, strings.Join(markupcore.PlainLines(lines), "\n")
}

// secondTexts gives each cue the plain text of the second cue with the largest positive overlap. A second cue
// fills only the first cue that picks it.
func secondTexts(cues, second []model.Cue) []string {
	texts := make([]string, len(cues))
	used := make([]bool, len(second))
	for i, c := range cues {
		best := -1
		var bestOverlap time.Duration
		for j, sc := range second {
			if overlap := min(c.End, sc.End) - max(c.Start, sc.Start); overlap > bestOverlap {
				best, bestOverlap = j, overlap
			}
		}
		if best < 0 || used[best] {
			continue
		}
		used[best] = true
		texts[i] = strings.Join(markupcore.PlainLines(second[best].Lines), "\n")
	}
	return texts
}

// cell returns the cell of column col for cue c, the i-th row.
func (set settings) cell(c model.Cue, col column, speaker, text string, seconds []string, i int) string {
	switch col.role {
	case "identifier":
		return c.ID
	case "start":
		return timeCell(c.Start, set.timeFormat, set.fps)
	case "end":
		return timeCell(c.End, set.timeFormat, set.fps)
	case "duration":
		return timeCell(c.End-c.Start, set.timeFormat, set.fps)
	case "speaker":
		return speaker
	case "text":
		return text
	case "second":
		return seconds[i]
	default:
		v, _ := item(storedCells(c), col.key)
		return phpString(v)
	}
}

// writeRow writes the cells of one row without its line ending. quotedAt marks the cells that get quotes even
// when they need none.
func (set settings) writeRow(b *strings.Builder, cells []string, quotedAt []bool) {
	for i, cell := range cells {
		if i > 0 {
			b.WriteString(set.delimiter)
		}
		if set.escape && cell != "" && strings.IndexByte("=+-@", cell[0]) >= 0 {
			cell = "'" + cell
		}
		if (i < len(quotedAt) && quotedAt[i]) || strings.ContainsAny(cell, "\"\r\n"+set.delimiter) {
			cell = `"` + strings.ReplaceAll(cell, `"`, `""`) + `"`
		}
		b.WriteString(cell)
	}
}
