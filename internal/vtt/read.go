// Package vtt holds the WebVTT block reader and writer that format/webvtt and stream share.
package vtt

import (
	"errors"
	"fmt"
	"iter"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	arrow       = "-->"
	phpTrimSet  = " \t\n\r\x00\v"
	utf8BOM     = "\ufeff"
	signature   = "WEBVTT"
	noSignature = "The file does not start with WEBVTT."
	// timestampPattern is a WebVTT timestamp: hours of any length or none, then exactly mm:ss.mmm.
	timestampPattern = `(?:\d+:)?[0-5]\d:[0-5]\d\.\d{3}`
)

//nolint:gochecknoglobals // lookup tables of the setting names and the entities that WebVTT decodes
var (
	cueSettings      = []string{"vertical", "line", "position", "size", "align", "region"}
	regionSettings   = []string{"id", "width", "lines", "regionanchor", "viewportanchor", "scroll"}
	entityCharacters = strings.NewReplacer("&nbsp;", "\u00a0", "&lrm;", "\u200e", "&rlm;", "\u200f")
)

var (
	strictTimeRegexp     = regexp.MustCompile(`^(?:(\d+):)?([0-5]\d):([0-5]\d)\.(\d{3})$`)
	endTimeRegexp        = regexp.MustCompile(`^(` + timestampPattern + `)(.*)$`)
	repairableTimeRegexp = regexp.MustCompile(`^` + timestampPattern + `[ \t]*` + arrow + `[ \t]*` + timestampPattern)
	ampersandRegexp      = regexp.MustCompile(`&(?:[a-zA-Z][a-zA-Z0-9]*;|#[0-9]+;|#[xX][0-9a-fA-F]+;)?`)
	shortTimestampRegexp = regexp.MustCompile(`<([0-5]\d:[0-5]\d\.\d{3})>`)
	keywordRegexp        = regexp.MustCompile(`(?i)^(?:NOTE|STYLE|REGION)`)
	spacesRegexp         = regexp.MustCompile(`[ \t]+`)
)

type line struct {
	n    int // 0-based
	text string
}

// Reader reads the blocks of a WebVTT file one at a time.
type Reader struct {
	st           *parsing.State
	file         *model.Data
	keepComments bool
	comments     []parsing.ParsedComment
	cues         int
}

type looseTime struct {
	text string
	d    time.Duration
}

// NewReader returns a reader that records its warnings and reads its mode from st.
// With keepComments, Comments returns the NOTE blocks. Otherwise the reader skips them.
func NewReader(st *parsing.State, keepComments bool) *Reader {
	return &Reader{st: st, file: model.NewData(), keepComments: keepComments}
}

// File returns the WebVTT file data: header, headerLines, regions and styles.
// It is complete once the reader has parsed the first cue block.
func (r *Reader) File() *model.Data {
	return r.file
}

// Comments returns the NOTE blocks that the reader has parsed, if it keeps them.
func (r *Reader) Comments() []parsing.ParsedComment {
	return r.comments
}

// Blocks yields the blocks of lines, the header block first. The keys of lines are 0-based line numbers.
// A file that does not start with the signature gives an error in strict mode. Lenient mode repairs it.
// An error ends the sequence.
func (r *Reader) Blocks(lines iter.Seq2[int, string]) iter.Seq2[blocks.Block, error] {
	return func(yield func(blocks.Block, error) bool) {
		var err error
		for b := range r.numberedBlocks(r.signedLines(trimLines(lines), &err)) {
			if !yield(b, nil) {
				return
			}
		}
		if err != nil {
			yield(blocks.Block{}, err)
		}
	}
}

// trimLines drops the white space around the file and joins runs of empty lines into one. A line of white
// space inside the file stays, because the block rules decide whether it ends a block.
func trimLines(lines iter.Seq2[int, string]) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		var held line
		started := false
		var pending []line
		for n, text := range lines {
			if phpTrim(text) == "" {
				if started && (text != "" || len(pending) == 0 || pending[len(pending)-1].text != "") {
					pending = append(pending, line{n: n, text: text})
				}
				continue
			}
			if !started {
				text = strings.TrimLeft(text, phpTrimSet)
			} else {
				if !yield(held.n, held.text) {
					return
				}
				for _, p := range pending {
					if !yield(p.n, p.text) {
						return
					}
				}
			}
			held, started = line{n: n, text: text}, true
			pending = pending[:0]
		}
		if started {
			yield(held.n, strings.TrimRight(held.text, phpTrimSet))
		}
	}
}

// signedLines yields the lines from the signature, each without its leading BOMs, so joined files work.
// Lenient mode repairs a missing signature. A failure goes to err, and the sequence ends.
func (r *Reader) signedLines(lines iter.Seq2[int, string], err *error) iter.Seq[line] {
	return func(yield func(line) bool) {
		var skipped []line
		firstLine := 0
		first, signed := true, false
		for n, text := range lines {
			l := line{n: n, text: trimBOMs(text)}
			if signed {
				if !yield(l) {
					return
				}
				continue
			}
			if first {
				first = false
				if HasSignature(text) {
					signed = true
					if !yield(l) {
						return
					}
					continue
				}
				if !r.st.Lenient() {
					*err = &model.ParseError{Format: model.WebVTT, Line: n + 1, Msg: noSignature}
					return
				}
			}
			if firstLine == 0 && phpTrim(l.text) != "" {
				firstLine = l.n + 1
			}
			if HasSignature(l.text) {
				r.warnMissingSignature(skipped, firstLine, l.n+1)
				signed = true
				if !yield(l) {
					return
				}
				continue
			}
			if !hasArrow(l.text) {
				skipped = append(skipped, l)
				continue
			}
			if !isRepairableTimingLine(phpTrim(l.text)) {
				break
			}
			cueStart := l.n
			var identifier []line
			if last := len(skipped) - 1; last >= 0 && phpTrim(skipped[last].text) != "" {
				identifier = []line{skipped[last]}
				skipped = skipped[:last]
				cueStart = identifier[0].n
			}
			r.warnMissingSignature(skipped, firstLine, cueStart+1)
			signed = true
			for _, repaired := range slices.Concat([]line{{n: cueStart - 2, text: signature}, {n: cueStart - 1}}, identifier, []line{l}) {
				if !yield(repaired) {
					return
				}
			}
		}
		if !signed && !first {
			*err = &model.ParseError{Format: model.WebVTT, Line: max(firstLine, 1), Msg: noSignature}
		}
	}
}

func isRepairableTimingLine(text string) bool {
	loc := repairableTimeRegexp.FindStringIndex(text)
	return loc != nil && (loc[1] == len(text) || !isDigit(text[loc[1]]))
}

func (r *Reader) warnMissingSignature(skipped []line, firstLine, readFrom int) {
	var lines []string
	for _, l := range skipped {
		if t := phpTrim(l.text); t != "" {
			lines = append(lines, t)
		}
	}
	msg := noSignature + " The parser read the cues without it."
	if len(lines) > 0 {
		msg = noSignature + " The parser skipped the lines before line " + strconv.Itoa(readFrom) + "."
	}
	r.st.Warn(msg, firstLine, 0, lines, model.Repaired)
}

// numberedBlocks returns the blocks of lines. Lenient mode joins cue text after an empty line to its cue.
func (r *Reader) numberedBlocks(lines iter.Seq[line]) iter.Seq[blocks.Block] {
	split := r.splitHeader(lines)
	if !r.st.Lenient() {
		return split
	}
	onJoin := func(block, line int, lines []string) {
		r.st.Warn(blocks.JoinWarning(block, line), line, block, lines, model.Repaired)
	}
	return blocks.JoinCueTextBlocks(split, blocks.Rules{IsTimingLine: hasArrow}, isOtherBlock, onJoin)
}

func isOtherBlock(lines []string) bool {
	first := phpTrim(lines[0])
	return keywordRegexp.MatchString(first) || HasSignature(first)
}

// splitHeader returns the blocks of lines. Lenient mode splits the cues off a header block without an empty line after it.
func (r *Reader) splitHeader(lines iter.Seq[line]) iter.Seq[blocks.Block] {
	return func(yield func(blocks.Block) bool) {
		isHeader := true
		for b := range collectBlocks(lines, false) {
			offset := -1
			if isHeader && r.st.Lenient() {
				offset = firstTimingLine(b.Lines)
			}
			isHeader = false
			if offset < 0 {
				if !yield(b) {
					return
				}
				continue
			}

			at := b.Line + offset
			r.st.Warn("The WEBVTT header has no empty line before the first cue. The parser split the header block at line "+
				strconv.Itoa(at)+".", at, 0, b.Lines, model.Repaired)
			if !yield(blocks.Block{Line: b.Line, Lines: b.Lines[:offset]}) {
				return
			}
			rest := make([]line, len(b.Lines)-offset)
			for i, text := range b.Lines[offset:] {
				rest[i] = line{n: at - 1 + i, text: text}
			}
			for rb := range collectBlocks(slices.Values(rest), true) {
				if !yield(rb) {
					return
				}
			}
		}
	}
}

// endsBlock reports whether text ends the block of current. The spec ends a cue only at an empty line.
// YouTube auto captions put a line with one space into each cue.
func endsBlock(text string, current []string) bool {
	return text == "" || phpTrim(text) == "" && !slices.ContainsFunc(current, hasArrow)
}

func firstTimingLine(lines []string) int {
	for i, l := range lines[1:] {
		if hasArrow(l) {
			return i + 1
		}
	}
	return -1
}

// collectBlocks splits lines at empty lines, and before a timing line that cannot belong to the current block.
// While hasBlocks is false, the first block does not split before a timing line, so a missing empty line after
// the header still fails in strict mode. See https://www.w3.org/TR/webvtt1/#collect-a-webvtt-block.
func collectBlocks(lines iter.Seq[line], hasBlocks bool) iter.Seq[blocks.Block] {
	return func(yield func(blocks.Block) bool) {
		c := collector{hasBlocks: hasBlocks}
		for l := range lines {
			if b, ok := c.add(l); ok && !yield(b) {
				return
			}
		}
		if len(c.current) > 0 {
			yield(c.block())
		}
	}
}

type collector struct {
	current   []string
	start     int
	hasBlocks bool
}

func (c *collector) add(l line) (blocks.Block, bool) {
	if endsBlock(l.text, c.current) {
		if len(c.current) == 0 {
			return blocks.Block{}, false
		}
		b := c.block()
		c.current = nil
		c.hasBlocks = true
		return b, true
	}
	var ended blocks.Block
	ok := false
	startsCue := len(c.current) == 0 || len(c.current) == 1 && !hasArrow(c.current[0])
	if hasArrow(l.text) && !startsCue && c.hasBlocks {
		ended, ok = c.block(), true
		c.current = nil
	}
	if len(c.current) == 0 {
		c.start = l.n + 1
	}
	c.current = append(c.current, l.text)
	return ended, ok
}

func (c *collector) block() blocks.Block {
	return blocks.Block{Line: c.start, Lines: c.current}
}

func (r *Reader) parseHeader(b blocks.Block) error {
	text, _ := strings.CutPrefix(b.Lines[0], signature)
	if text = phpTrim(text); text != "" {
		r.file.Set("header", text)
	}
	if len(b.Lines) == 1 {
		return nil
	}
	headerLines := make([]any, len(b.Lines)-1)
	for i, l := range b.Lines[1:] {
		if hasArrow(l) {
			return &model.ParseError{Format: model.WebVTT, Line: b.Line + 1 + i, Msg: "The WEBVTT header has no empty line before the first cue."}
		}
		headerLines[i] = l
	}
	r.file.Set("headerLines", headerLines)
	return nil
}

// ParseBlock reads the block number index, which comes from Blocks. The header block is number 0.
// It returns a cue and true for a cue block. Lenient mode skips a broken block with a warning.
func (r *Reader) ParseBlock(b blocks.Block, index int) (model.Cue, bool, error) {
	if index == 0 {
		return model.Cue{}, false, r.parseHeader(b)
	}
	var perr *model.ParseError
	if hasArrow(b.Lines[0]) || len(b.Lines) > 1 && hasArrow(b.Lines[1]) {
		c, err := r.parseCue(b, index)
		if err == nil {
			r.cues++
			return c, true, nil
		}
		if !errors.As(err, &perr) {
			return model.Cue{}, false, err
		}
	} else {
		perr = r.parseOtherBlock(b, index)
	}
	if perr == nil {
		return model.Cue{}, false, nil
	}
	return model.Cue{}, false, r.st.Fail(perr, perr.Line, index, b.Lines)
}

func (r *Reader) parseOtherBlock(b blocks.Block, index int) *model.ParseError {
	first := phpTrim(b.Lines[0])
	switch {
	case isNote(first):
		if r.keepComments {
			r.comments = append(r.comments, parsing.ParsedComment{Text: commentText(b.Lines), CuesBefore: r.cues})
		}
	case r.cues == 0 && first == "STYLE":
		r.appendFileData("styles", strings.Join(b.Lines[1:], "\n"))
	case r.cues == 0 && first == "REGION":
		r.appendFileData("regions", parseSettings(strings.Join(b.Lines[1:], " "), regionSettings))
	case keywordRegexp.MatchString(first):
		r.warnLateBlock(b, index, first)
	default:
		return &model.ParseError{
			Format: model.WebVTT,
			Line:   b.Line,
			Msg:    "Block #" + strconv.Itoa(index) + " is not a WebVTT cue, comment, style or region. The line is " + parsing.Quote(first) + ".",
		}
	}
	return nil
}

// isNote reports whether the first line, trimmed, starts a comment block. The keyword is case-sensitive.
// See https://www.w3.org/TR/webvtt1/#webvtt-comment-block.
func isNote(first string) bool {
	return first == "NOTE" || strings.HasPrefix(first, "NOTE ") || strings.HasPrefix(first, "NOTE\t")
}

// warnLateBlock warns in lenient mode about a STYLE or REGION block after a cue. The spec parser ignores every block
// that is not a cue, so such blocks do not fail.
func (r *Reader) warnLateBlock(b blocks.Block, index int, first string) {
	if r.st.Lenient() && (first == "STYLE" || first == "REGION") {
		r.st.Warn("Block #"+strconv.Itoa(index)+" is a "+first+" block after a cue. The spec allows it only before the first cue, so the parser skipped it.",
			b.Line, index, b.Lines, model.Skipped)
	}
}

func (r *Reader) appendFileData(key string, value any) {
	list, _ := r.file.List(key)
	r.file.Set(key, append(list, value))
}

// parseCue reads a cue block: an optional identifier, the timing line with the settings, and the text lines.
func (r *Reader) parseCue(b blocks.Block, index int) (model.Cue, error) {
	lines := make([]string, len(b.Lines))
	for i, l := range b.Lines {
		lines[i] = phpTrim(spacesRegexp.ReplaceAllString(l, " "))
	}
	timingLine := b.Line
	var id string
	if len(lines) > 1 && hasArrow(lines[1]) {
		id = lines[0]
		lines = lines[1:]
		timingLine++
	}

	startText, endText, settingsText, err := r.splitTimingLine(lines[0], timingLine)
	if err != nil {
		return model.Cue{}, err
	}
	text := textLines(lines[1:])
	for i, l := range text {
		if err := parsing.CheckWordTimestamps([]string{l}, timingLine+1+i); err != nil {
			return model.Cue{}, err
		}
	}
	start, end, err := r.cueTimes(startText, endText, timingLine, index, lines)
	if err != nil {
		return model.Cue{}, err
	}

	c := model.Cue{Start: start, End: end, ID: id}
	c.SetText(strings.Join(text, "\n"))
	settings := parseSettings(settingsText, cueSettings)
	if a, ok := settingsAlignment(settings); ok {
		c.Alignment = a
		settings.Set("alignment", int64(a.Numpad()))
	}
	if err := c.SetFormatData(string(model.WebVTT), settings); err != nil {
		return model.Cue{}, fmt.Errorf("%s cue data: %w", model.WebVTT, err)
	}
	return c, nil
}

func (r *Reader) splitTimingLine(timing string, timingLine int) (start, end, settings string, err error) {
	start, endPart, _ := strings.Cut(timing, arrow)
	endPart = phpTrim(endPart)
	end, settings, ok := splitEnd(endPart)
	if ok {
		return start, end, settings, nil
	}
	if !r.st.Lenient() {
		return "", "", "", invalidTime(endPart, timingLine)
	}
	if i := strings.IndexAny(endPart, " \t"); i >= 0 {
		return start, endPart[:i], strings.TrimLeft(endPart[i:], " \t"), nil
	}
	return start, endPart, "", nil
}

func (r *Reader) cueTimes(startText, endText string, timingLine, index int, lines []string) (time.Duration, time.Duration, error) {
	var loose []looseTime
	start, err := r.time(startText, timingLine, &loose)
	if err != nil {
		return 0, 0, err
	}
	end, err := r.time(endText, timingLine, &loose)
	if err != nil {
		return 0, 0, err
	}
	start, end, err = r.st.OrderedTimes(start, end, timingLine, index, lines, true)
	if err != nil {
		return 0, 0, err
	}
	for _, lt := range loose {
		r.st.Warn("Block #"+strconv.Itoa(index)+" has the time \""+lt.text+"\", which is not in the form hh:mm:ss.mmm. The parser read it as "+
			timecode.Seconds(lt.d)+" s.", timingLine, index, lines, model.Repaired)
	}
	return start, end, nil
}

// splitEnd splits the text after the arrow into the end time and the settings. Settings may follow the end time
// without white space, as in "00:01.000line:40%". A 4th fraction digit is no setting.
func splitEnd(endPart string) (end, settings string, ok bool) {
	m := endTimeRegexp.FindStringSubmatch(endPart)
	if m == nil || m[2] != "" && isDigit(m[2][0]) {
		return "", "", false
	}
	return m[1], strings.TrimLeft(m[2], " \t"), true
}

// textLines turns the cue text lines into core markup. WebVTT text keeps a raw ">" and a "<" before white space.
func textLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = escapeBareAmpersands(entityCharacters.Replace(l))
	}
	out = markupcore.UnknownTagsAsText(out)
	for i, l := range out {
		if strings.Contains(l, "<") {
			out[i] = shortTimestampRegexp.ReplaceAllString(l, "<00:$1>")
		}
	}
	return out
}

// escapeBareAmpersands escapes an & that starts no character reference, as in "Rye & spelt" or "&foo;",
// because the WebVTT spec reads it as text.
func escapeBareAmpersands(l string) string {
	if !strings.Contains(l, "&") {
		return l
	}
	return ampersandRegexp.ReplaceAllStringFunc(l, func(m string) string {
		if m == "&" || markupcore.DecodeEntities(m) == m {
			return "&amp;" + m[1:]
		}
		return m
	})
}

// time reads a cue time. Lenient mode also reads the timecode.Loose forms with "." or "," and adds them to loose.
func (r *Reader) time(text string, timingLine int, loose *[]looseTime) (time.Duration, error) {
	text = phpTrim(text)
	if d, ok := strictTime(text); ok {
		return parsing.BoundedTime(d, text, timingLine)
	}
	// A last field of 1 digit without a fraction is a time that the end of the file cut off.
	cutOff := len(text) >= 2 && text[len(text)-2] == ':' && isDigit(text[len(text)-1])
	if r.st.Lenient() && !cutOff {
		if d, ok := timecode.Loose(text, ".,", true); ok {
			if !slices.ContainsFunc(*loose, func(lt looseTime) bool { return lt.text == text }) {
				*loose = append(*loose, looseTime{text: text, d: d})
			}
			return d, nil
		}
	}
	return 0, invalidTime(text, timingLine)
}

func invalidTime(text string, line int) *model.ParseError {
	return &model.ParseError{Format: model.WebVTT, Line: line, Msg: `The time "` + text + `" is not valid.`}
}

// strictTime reads a WebVTT timestamp. Hours of 100000 or more give timecode.MaxTime, also when they overflow.
func strictTime(text string) (time.Duration, bool) {
	m := strictTimeRegexp.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	var hours int
	if m[1] != "" {
		h, err := strconv.Atoi(m[1])
		if err != nil || h >= timecode.MaxHours {
			return timecode.MaxTime, true
		}
		hours = h
	}
	minutes, _ := strconv.Atoi(m[2])
	seconds, _ := strconv.Atoi(m[3])
	millis, _ := strconv.Atoi(m[4])
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second + time.Duration(millis)*time.Millisecond, true
}

// parseSettings keeps the exact value of each "name:value" token whose name is in names. The last token of a name
// wins and keeps the place of the first. See https://www.w3.org/TR/webvtt1/#parse-the-webvtt-cue-settings.
func parseSettings(input string, names []string) *model.Data {
	d := model.NewData()
	for _, token := range strings.FieldsFunc(phpTrim(input), func(r rune) bool { return r == ' ' || r == '\t' }) {
		colon := strings.IndexByte(token, ':')
		if colon <= 0 || colon == len(token)-1 {
			continue
		}
		if name := token[:colon]; slices.Contains(names, name) {
			d.Set(name, token[colon+1:])
		}
	}
	return d
}

// settingsAlignment maps only the settings with a clear place on the screen. Start and end depend on the text direction.
func settingsAlignment(d *model.Data) (model.Alignment, bool) {
	if _, vertical := d.Get("vertical"); d.Len() == 0 || vertical {
		return model.BottomCenter, false
	}
	lineValue, hasLine := d.String("line")
	alignValue, hasAlign := d.String("align")
	row, rowOK := settingsRow(lineValue, hasLine)
	column, columnOK := settingsColumn(alignValue, hasAlign)
	if !rowOK || !columnOK || !hasLine && !hasAlign {
		return model.BottomCenter, false
	}
	return model.AlignmentAt(row, column)
}

func settingsRow(line string, ok bool) (int, bool) {
	switch {
	case !ok || line == "-1" || line == "100%,end":
		return 0, true
	case line == "50%,center":
		return 1, true
	case line == "0" || line == "0%" || line == "0,start":
		return 2, true
	default:
		return 0, false
	}
}

func settingsColumn(align string, ok bool) (int, bool) {
	switch {
	case align == "left":
		return 1, true
	case !ok || align == "center":
		return 2, true
	case align == "right":
		return 3, true
	default:
		return 0, false
	}
}

// commentText returns the text of a NOTE block: the text after NOTE and the other lines, trimmed, without empty lines.
func commentText(lines []string) string {
	first, _ := strings.CutPrefix(phpTrim(lines[0]), "NOTE")
	out := make([]string, 0, len(lines))
	for _, l := range slices.Concat([]string{first}, lines[1:]) {
		if l = phpTrim(l); l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// HasSignature reports whether text starts with WEBVTT, then a space, a tab, LF or the end.
func HasSignature[T string | []byte](text T) bool {
	if len(text) < len(signature) || string(text[:len(signature)]) != signature {
		return false
	}
	if len(text) == len(signature) {
		return true
	}
	next := text[len(signature)]
	return next == ' ' || next == '\t' || next == '\n'
}

func hasArrow(l string) bool {
	return strings.Contains(l, arrow)
}

func phpTrim(s string) string {
	return strings.Trim(s, phpTrimSet)
}

func trimBOMs(s string) string {
	for strings.HasPrefix(s, utf8BOM) {
		s = s[len(utf8BOM):]
	}
	return s
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
