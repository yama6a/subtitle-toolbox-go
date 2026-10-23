package csv

import (
	"errors"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// millisecondsMedian is the smallest median cue duration, read as seconds, of a table of whole numbers that the
// reader reads as milliseconds. openai-whisper and whisper.cpp write such tables.
const millisecondsMedian = 100

// synonyms holds the header names that stand for a role when the header lacks start or text, compared without
// case, spaces, underscores and hyphens. An earlier synonym wins.
var synonyms = map[string][]string{ //nolint:gochecknoglobals // a fixed table
	"start":    {"begin", "in", "starttime", "starttc", "timecode", "tcin"},
	"end":      {"out", "stop", "endtime", "endtc", "tcout"},
	"duration": {"length"},
	"speaker":  {"name", "character"},
	"text":     {"subtitle", "caption", "dialogue", "transcript"},
}

// roleIndex is a role and its column index.
type roleIndex struct {
	role  string
	index int
}

// layout holds the resolved roles of a table, in role order.
type layout []roleIndex

func (l layout) index(r string) (int, bool) {
	for _, ri := range l {
		if ri.role == r {
			return ri.index, true
		}
	}
	return 0, false
}

func (l layout) has(i int) bool {
	return slices.ContainsFunc(l, func(ri roleIndex) bool { return ri.index == i })
}

// cell returns the trimmed cell of role r, or "" when the table or the row has no such cell.
func (l layout) cell(cells []string, r string) string {
	if i, ok := l.index(r); ok && i < len(cells) {
		return trim(cells[i])
	}
	return ""
}

type reader struct {
	st        *parsing.State
	columns   Columns
	delimiter string
	fps       float64
}

// Read reads a CSV or TSV table from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.CSV, err)
	}
	st := parsing.NewState(model.CSV, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	text := string(content)
	delimiter := opts.Delimiter
	if delimiter == DelimiterDefault {
		delimiter = detectDelimiter(text)
	}
	rd := &reader{st: st, columns: opts.Columns, delimiter: string(delimiter), fps: opts.FrameRate}
	s, err := rd.read(text)
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func (rd *reader) read(content string) (*model.Subtitle, error) {
	recs, err := rd.records(content)
	if err != nil {
		return nil, err
	}
	if !rd.columns.NoHeader && rd.st.Lenient() {
		recs = rd.skipRowsBeforeHeader(recs)
	}
	var header *record
	if !rd.columns.NoHeader && len(recs) > 0 {
		header, recs = &recs[0], recs[1:]
	}
	roles, err := rd.resolveRoles(header)
	if err != nil {
		return nil, err
	}
	rd.warnSynonyms(header, roles)
	cues, timeFormat, err := rd.cues(recs, roles, header)
	if err != nil {
		return nil, err
	}
	s := &model.Subtitle{}
	s.AddCues(cues...)
	_ = s.SetFormatData(formatDataKey, rd.fileData(header, recs, roles, timeFormat)) // the data has the shape of the schema
	return s, nil
}

// cues reads each data row as a cue and returns the time format of the first cue. Lenient mode skips a row that
// fails.
func (rd *reader) cues(recs []record, roles layout, header *record) ([]model.Cue, TimeFormat, error) {
	millis := rd.millisecondTimes(recs, roles)
	var cues []model.Cue
	var open []int
	timeFormat := TimeDot
	for i, rec := range recs {
		c, hasEnd, err := rd.cue(rec, i, roles, header, millis)
		if err != nil {
			var perr *model.ParseError
			if !errors.As(err, &perr) {
				return nil, "", err
			}
			if err := rd.st.Fail(perr, rec.line, i, []string{rd.join(rec.cells)}); err != nil {
				return nil, "", err
			}
			continue
		}
		if len(cues) == 0 {
			timeFormat = timeFormatOf(roles.cell(rec.cells, "start"), millis)
		}
		if !hasEnd {
			open = append(open, len(cues))
		}
		cues = append(cues, c)
	}
	rd.closeOpenEnds(cues, open)
	return cues, timeFormat, nil
}

// records splits the content, warns for each unclosed quote, drops the blank rows and checks the row width.
func (rd *reader) records(content string) ([]record, error) {
	recs, unclosed, err := records(content, rd.delimiter[0], rd.st.Lenient())
	if err != nil {
		return nil, err
	}
	for _, u := range unclosed {
		rd.st.Warn("A quoted CSV cell has no closing quote. The cell ends at the end of the line.", u.line, model.NoBlock,
			[]string{u.text}, model.Repaired)
	}
	recs = slices.DeleteFunc(recs, func(r record) bool {
		return !slices.ContainsFunc(r.cells, func(c string) bool { return trim(c) != "" })
	})
	for _, r := range recs {
		if len(r.cells) > maxColumns {
			return nil, &model.ParseError{
				Format: model.CSV, Line: r.line,
				Msg: "The table has " + strconv.Itoa(len(r.cells)) + " columns. The limit is " + strconv.Itoa(maxColumns) + ".",
			}
		}
	}
	return recs, nil
}

// skipRowsBeforeHeader drops the rows before the first row that resolves the roles, with one warning. Without
// such a row, it keeps all rows, so resolveRoles names the missing column.
func (rd *reader) skipRowsBeforeHeader(recs []record) []record {
	for i := range recs {
		if _, err := rd.resolveRoles(&recs[i]); err != nil {
			continue
		}
		if i > 0 {
			rows := "1 row"
			if i > 1 {
				rows = strconv.Itoa(i) + " rows"
			}
			lines := make([]string, i)
			for j, r := range recs[:i] {
				lines[j] = rd.join(r.cells)
			}
			rd.st.Warn("The table has "+rows+" before the header row.", recs[0].line, model.NoBlock, lines, model.Skipped)
		}
		return recs[i:]
	}
	return recs
}

// resolveRoles returns the column index of each role that the table has. A missing start or text column, or a
// mapped column that the table lacks, fails.
func (rd *reader) resolveRoles(header *record) (layout, error) {
	names, line, width := headerNames(header)
	useSynonyms := rd.columns.isZero() && (!slices.Contains(names, "start") || !slices.Contains(names, "text"))
	var out layout
	for _, r := range roles {
		c := rd.columns.column(r.name)
		index := columnIndex(c, r.name, names, useSynonyms)
		switch {
		case index >= 0 && index < width:
			out = append(out, roleIndex{r.name, index})
		case needed(c, r.name):
			return nil, &model.ParseError{
				Format: model.CSV, Line: line, Msg: `The table has no column "` + columnName(c, r.name) + `" for ` + r.name + ".",
			}
		}
	}
	return out, nil
}

// headerNames returns the header names in lower case after a trim, the line of the header and the width that a
// column index must be below. Without a header, the width is the column limit and the line is 1.
func headerNames(header *record) (names []string, line, width int) {
	if header == nil {
		return nil, 1, maxColumns
	}
	names = make([]string, len(header.cells))
	for i, h := range header.cells {
		names[i] = markupcore.ASCIILower(trim(h))
	}
	return names, header.line, len(header.cells)
}

// columnIndex returns the index of column c of role r in the lower-case header names, or -1. A role without a
// header of its name takes a synonym when useSynonyms is set.
func columnIndex(c Column, r string, names []string, useSynonyms bool) int {
	switch {
	case c.isIndex:
		return c.index
	case c.isName:
		return slices.Index(names, markupcore.ASCIILower(trim(c.name)))
	}
	if i := slices.Index(names, r); i >= 0 || !useSynonyms {
		return i
	}
	return synonymIndex(r, names)
}

// needed reports whether a missing column fails the read: a mapped column, or the start or the text.
func needed(c Column, r string) bool {
	return !c.IsZero() || r == "start" || r == "text"
}

// columnName returns the name of column c of role r in a message.
func columnName(c Column, r string) string {
	switch {
	case c.isName:
		return c.name
	case c.isIndex:
		return strconv.Itoa(c.index)
	default:
		return r
	}
}

// synonymIndex returns the index of the first header name that is a synonym of role r, or -1.
func synonymIndex(r string, lowerNames []string) int {
	compact := make([]string, len(lowerNames))
	for i, n := range lowerNames {
		b := make([]byte, 0, len(n))
		for j := range len(n) {
			if !strings.ContainsRune(" \t\n\v\f\r_-", rune(n[j])) {
				b = append(b, n[j])
			}
		}
		compact[i] = string(b)
	}
	for _, syn := range synonyms[r] {
		if i := slices.Index(compact, syn); i >= 0 {
			return i
		}
	}
	return -1
}

// warnSynonyms warns, also in strict mode, about the roles whose header name is not the role name, when the
// roles come from the header without a column layout.
func (rd *reader) warnSynonyms(header *record, roles layout) {
	if header == nil || !rd.columns.isZero() {
		return
	}
	var found []string
	for _, ri := range roles {
		if name := trim(header.cells[ri.index]); markupcore.ASCIILower(name) != ri.role {
			found = append(found, `"`+name+`" as `+ri.role)
		}
	}
	if len(found) == 0 {
		return
	}
	msg := "column " + found[0]
	if n := len(found); n > 1 {
		msg = "columns " + strings.Join(found[:n-1], ", ") + " and " + found[n-1]
	}
	rd.st.Warn("The parser reads the "+msg+".", header.line, model.NoBlock, []string{rd.join(header.cells)}, model.Repaired)
}

// millisecondTimes reports whether every start, end and duration cell is digits only and the median cue duration,
// read as seconds, is millisecondsMedian or more. Then it warns once, also in strict mode.
func (rd *reader) millisecondTimes(recs []record, roles layout) bool {
	var durations []int64
	for _, rec := range recs {
		start, end, duration := roles.cell(rec.cells, "start"), roles.cell(rec.cells, "end"), roles.cell(rec.cells, "duration")
		for _, t := range []string{start, end, duration} {
			if t != "" && !isDigits(t) {
				return false
			}
		}
		switch {
		case start != "" && end != "":
			durations = append(durations, phpInt(end)-phpInt(start))
		case duration != "":
			durations = append(durations, phpInt(duration))
		}
	}
	if len(durations) == 0 {
		return false
	}
	median, text := medianOf(durations)
	if median < millisecondsMedian {
		return false
	}
	rd.st.Warn("The times are whole numbers, and the median cue lasts "+text+" seconds. The parser reads the times as milliseconds.",
		recs[0].line, model.NoBlock, []string{rd.join(recs[0].cells)}, model.Repaired)
	return true
}

// phpInt reads digits as PHP (int) does: beyond the int64 range it gives the int64 maximum.
func phpInt(digits string) int64 {
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 1<<63 - 1
	}
	return n
}

// medianOf returns the median of values and its text as PHP prints the number. The median of an even count is
// the mean of the 2 middle values.
func medianOf(values []int64) (float64, string) {
	slices.Sort(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return float64(values[mid]), strconv.FormatInt(values[mid], 10)
	}
	a, b := values[mid-1], values[mid]
	if b > 0 && a > math.MaxInt64-b || b < 0 && a < math.MinInt64-b {
		m := (float64(a) + float64(b)) / 2
		return m, timecode.Number(m)
	}
	if sum := a + b; sum%2 == 0 {
		return float64(sum / 2), strconv.FormatInt(sum/2, 10)
	}
	m := float64(a+b) / 2
	return m, timecode.Number(m)
}

// cue reads one data row. hasEnd is false for a row without an end or a duration.
func (rd *reader) cue(rec record, index int, roles layout, header *record, millis bool) (model.Cue, bool, error) {
	start, err := parseTime(roles.cell(rec.cells, "start"), rd.fps, millis, rec.line)
	if err != nil {
		return model.Cue{}, false, err
	}
	end, hasEnd, err := rd.end(rec, roles, start, millis)
	if err != nil {
		return model.Cue{}, false, err
	}
	if hasEnd {
		if _, _, err := rd.st.OrderedTimes(start, end, rec.line, index, []string{rd.join(rec.cells)}, false); err != nil {
			return model.Cue{}, false, err
		}
	}
	var text string
	if i, ok := roles.index("text"); ok && i < len(rec.cells) {
		text = rec.cells[i]
	}
	c := model.Cue{
		Start: start, End: end, ID: roles.cell(rec.cells, "identifier"),
		Lines: markupcore.AddSpeaker(strings.Split(markupcore.EscapeText(text), "\n"), roles.cell(rec.cells, "speaker")),
	}
	if other := otherCells(rec.cells, roles, header); other != nil {
		_ = c.SetFormatData(formatDataKey, model.NewData(model.Field{Key: "columns", Value: other})) // the data has the shape of the schema
	}
	return c, hasEnd, nil
}

// end returns the end cell of a row, else its start plus the duration cell. hasEnd is false without both.
func (rd *reader) end(rec record, roles layout, start time.Duration, millis bool) (end time.Duration, hasEnd bool, err error) {
	if cell := roles.cell(rec.cells, "end"); cell != "" {
		end, err = parseTime(cell, rd.fps, millis, rec.line)
		return end, true, err
	}
	cell := roles.cell(rec.cells, "duration")
	if cell == "" {
		return start, false, nil
	}
	d, err := parseTime(cell, rd.fps, millis, rec.line)
	// The sum stays a valid time, as a computed last cue end does.
	return min(start+d, timecode.MaxValidTime), true, err
}

// otherCells returns the cells of the columns without a role, keyed by header name, else by column index. A short
// row is padded with "" up to the header width. The result is a list when its keys are 0 to n-1 in order, as PHP
// encodes such an array, else an object. It is nil when no cell is left.
func otherCells(cells []string, roles layout, header *record) any {
	width := len(cells)
	if header != nil {
		width = max(width, len(header.cells))
	}
	named := model.NewData()
	for i := range width {
		if roles.has(i) {
			continue
		}
		key := strconv.Itoa(i)
		if header != nil && i < len(header.cells) {
			key = header.cells[i]
		}
		value := ""
		if i < len(cells) {
			value = cells[i]
		}
		named.Set(key, value)
	}
	if named.Len() == 0 {
		return nil
	}
	keys := named.Keys()
	list := make([]any, len(keys))
	for i, k := range keys {
		if k != strconv.Itoa(i) {
			return named
		}
		list[i], _ = named.Get(k)
	}
	return list
}

// closeOpenEnds ends each open cue at the first later distinct start of all cues, else at its start plus the
// last cue duration.
func (rd *reader) closeOpenEnds(cues []model.Cue, open []int) {
	if len(open) == 0 {
		return
	}
	starts := make([]time.Duration, len(cues))
	for i, c := range cues {
		starts[i] = c.Start
	}
	slices.Sort(starts)
	for _, i := range open {
		j, _ := slices.BinarySearch(starts, cues[i].Start)
		cues[i].End = rd.st.EndAtNextStart(starts, j)
	}
}

// fileData returns the csv file data of the table.
func (rd *reader) fileData(header *record, recs []record, roles layout, timeFormat TimeFormat) *model.Data {
	var headerCells any
	width := 0
	if header != nil {
		headerCells = anyStrings(header.cells)
		width = len(header.cells)
	}
	for _, r := range recs {
		width = max(width, len(r.cells))
	}
	roleData := model.NewData()
	for _, ri := range roles {
		roleData.Set(ri.role, int64(ri.index))
	}
	var fps any
	if rd.fps != 0 {
		fps = rd.fps
	}
	d := model.NewData(
		model.Field{Key: "delimiter", Value: rd.delimiter},
		model.Field{Key: "header", Value: headerCells},
		model.Field{Key: "roles", Value: roleData},
		model.Field{Key: "width", Value: int64(width)},
		model.Field{Key: "timeFormat", Value: string(timeFormat)},
		model.Field{Key: "frameRate", Value: fps},
	)
	if quoted := quotedColumns(recs); len(quoted) > 0 {
		d.Set("quotedColumns", quoted)
	}
	return d
}

// quotedColumns returns the column indexes that every data row quotes, in the order of the first row.
func quotedColumns(recs []record) []any {
	if len(recs) == 0 {
		return nil
	}
	var out []any
	for _, i := range recs[0].quoted {
		if !slices.ContainsFunc(recs[1:], func(r record) bool { return !slices.Contains(r.quoted, i) }) {
			out = append(out, int64(i))
		}
	}
	return out
}

func (rd *reader) join(cells []string) string {
	return strings.Join(cells, rd.delimiter)
}

func anyStrings(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func trim(s string) string {
	return strings.Trim(s, markupcore.PHPTrimSet)
}
