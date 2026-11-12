package mcc

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea708"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

var (
	versionRegexp  = regexp.MustCompile(`^` + versionPrefix + `(\d+\.\d+)$`)
	fieldRegexp    = regexp.MustCompile(`^([A-Za-z][A-Za-z ]*)=(.*)$`)
	timeCodeRegexp = regexp.MustCompile(`(?i)^(24|25|30|50|60)(DF)?$`)
)

const timeCodeRate = "Time Code Rate"

// Read reads MCC from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.MCC, err)
	}
	st := parsing.NewState(model.MCC, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	lines := trimmedLines(string(content))
	version, err := readVersion(lines)
	if err != nil {
		return nil, nil, err
	}
	header, packets, rateCode, err := readLines(st, lines)
	if err != nil {
		return nil, nil, err
	}
	row, dropFrame, err := frameRate(st, header, rateCode)
	if err != nil {
		return nil, nil, err
	}
	header.Delete(timeCodeRate)
	if err := s.SetFormatData(formatDataKey, fileData(version, row, dropFrame, header)); err != nil {
		return nil, nil, fmt.Errorf("%s file data: %w", model.MCC, err)
	}

	timeline, err := frames(st, lines, packets, row, dropFrame)
	if err != nil {
		return nil, nil, err
	}
	cues := caption.CaptionDataCues(timeline, opts.cea708Service(),
		caption.CEA608Options{Channel: max(opts.Channel, 1), RollUp: opts.RollUp}, st.LastCueDuration())
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

func trimmedLines(content string) []string {
	lines := make([]string, 0, strings.Count(content, "\n")+1)
	for _, l := range parsing.Lines(content) {
		lines = append(lines, strings.Trim(l, markupcore.PHPTrimSet))
	}
	return lines
}

// cea708Service returns the service to read, or 0 for none.
func (o ReadOptions) cea708Service() int {
	if o.CEA608Only {
		return 0
	}
	return max(o.Service, 1)
}

func fileData(version string, row rate, dropFrame bool, header *model.Data) *model.Data {
	return model.NewData(
		model.Field{Key: "version", Value: version},
		model.Field{Key: "frameRate", Value: row.name},
		model.Field{Key: "dropFrame", Value: dropFrame},
		model.Field{Key: "header", Value: header},
	)
}

// readLines returns the header fields, the packets that hold triplets and the rate code of the first packet that
// holds a CDP, or -1.
func readLines(st *parsing.State, lines []string) (*model.Data, []rawPacket, int, error) {
	header := &model.Data{}
	packets := make([]rawPacket, 0, len(lines)/2)
	rateCode := -1
	for i, line := range lines {
		if isIgnored(line) || readField(header, line) {
			continue
		}
		n := i + 1
		code, tc, triplets, msg := readLine(line)
		if msg != "" {
			if err := st.Fail(&model.ParseError{Line: n, Msg: msg}, n, model.NoBlock, []string{line}); err != nil {
				return nil, nil, 0, err
			}
			continue
		}
		if code >= 0 && rateCode < 0 {
			rateCode = code
		}
		if len(triplets) > 0 {
			packets = append(packets, rawPacket{int32(tc.Hours), int32(n), uint8(tc.Minutes), uint8(tc.Seconds), uint8(tc.Frames), triplets})
		}
	}
	return header, packets, rateCode, nil
}

// isIgnored reports whether the line is blank, a comment or a version line.
func isIgnored(line string) bool {
	return line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, versionPrefix)
}

// readField stores the line as a header field when it is one. Data lines start with a digit, so only a letter can start a field.
func readField(header *model.Data, line string) bool {
	if c := line[0]; c < 'A' || c > 'z' {
		return false
	}
	m := fieldRegexp.FindStringSubmatch(line)
	if m == nil {
		return false
	}
	header.Set(strings.Trim(m[1], markupcore.PHPTrimSet), strings.Trim(m[2], markupcore.PHPTrimSet))
	return true
}

// rawPacket is the triplets of one data line, with its time code packed to keep a large file small.
type rawPacket struct {
	hours    int32
	line     int32
	minutes  uint8
	seconds  uint8
	frames   uint8
	triplets []cea708.Triplet
}

func (p rawPacket) timeCode() model.Timecode {
	return model.Timecode{Hours: int(p.hours), Minutes: int(p.minutes), Seconds: int(p.seconds), Frames: int(p.frames)}
}

func readVersion(lines []string) (string, error) {
	for i, line := range lines {
		if line == "" {
			continue
		}
		m := versionRegexp.FindStringSubmatch(line)
		if m == nil {
			return "", &model.ParseError{
				Format: model.MCC, Line: i + 1,
				Msg: `An MCC file must start with the line "` + versionPrefix + `2.0" or "` + versionPrefix + `1.0".`,
			}
		}
		if m[1] != "1.0" && m[1] != "2.0" {
			return "", &model.ParseError{Format: model.MCC, Line: i + 1, Msg: "The MCC version " + m[1] + " is unknown. The parser reads 1.0 and 2.0."}
		}
		return m[1], nil
	}
	return "", nil
}

// readLine returns the CDP rate code, the time code and the triplets of a data line. The code is -1 for an ANC
// packet of other data. A non-empty message says why the line is bad.
func readLine(line string) (code int, tc model.Timecode, triplets []cea708.Triplet, msg string) {
	tc, text, ok := parseDataLine(line)
	if !ok {
		return 0, tc, nil, `The MCC line "` + line + `" does not start with a time code.`
	}
	data, msg := decodeHex(text)
	if msg != "" {
		return 0, tc, nil, msg
	}
	if len(data) < 3 || data[0] != 0x61 || data[1] != 0x01 {
		return -1, tc, nil, ""
	}
	code, triplets, err := cea708.ReadCDP(data[3:min(3+int(data[2]), len(data))])
	if err != nil {
		return 0, tc, nil, err.Error()
	}
	return code, tc, triplets, ""
}

// parseDataLine splits a data line into its time code and its hex text. The time code has 2 to 6 hour digits, then
// MM, SS and FF with ":" or ";" before the frames. Version 2.0 may add a field suffix ".0" or ".1" and a line number
// ",N", which the reader ignores. White space separates the time code from the hex text, which has none.
func parseDataLine(line string) (tc model.Timecode, text string, ok bool) {
	tc, i, ok := timeCodeAt(line)
	if !ok {
		return tc, "", false
	}
	if i, ok = skipSuffix(line, i); !ok {
		return tc, "", false
	}
	start := i
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	if i == start || i == len(line) || strings.ContainsAny(line[i:], "\t\n\f\r ") {
		return tc, "", false
	}
	return tc, line[i:], true
}

// timeCodeAt reads HH:MM:SS:FF at the start of line and returns the index after it.
func timeCodeAt(line string) (tc model.Timecode, next int, ok bool) {
	var n [4]int
	for k, maxDigits := range [4]int{6, 2, 2, 2} {
		if k > 0 {
			if next >= len(line) || line[next] != ':' && (k < 3 || line[next] != ';') {
				return tc, 0, false
			}
			next++
		}
		if n[k], next, ok = digitsAt(line, next, 2, maxDigits); !ok {
			return tc, 0, false
		}
	}
	return model.Timecode{Hours: n[0], Minutes: n[1], Seconds: n[2], Frames: n[3]}, next, true
}

// skipSuffix skips the optional field suffix and line number after the frames of a version 2.0 line.
func skipSuffix(line string, i int) (int, bool) {
	if i+1 >= len(line) || line[i] != '.' || line[i+1] != '0' && line[i+1] != '1' {
		return i, true
	}
	i += 2
	if i < len(line) && line[i] == ',' {
		_, next, ok := digitsAt(line, i+1, 1, len(line))
		return next, ok
	}
	return i, true
}

// digitsAt reads between minDigits and maxDigits decimal digits of s at i. It returns the number and the next index.
func digitsAt(s string, i, minDigits, maxDigits int) (n, next int, ok bool) {
	next = i
	for next < len(s) && s[next] >= '0' && s[next] <= '9' && next-i < maxDigits {
		n = n*10 + int(s[next]-'0')
		next++
	}
	return n, next, next-i >= minDigits
}

// timeCode returns the labels per second and the drop-frame flag of the Time Code Rate field. In lenient mode, a
// missing or unknown field warns and reads 30DF.
func timeCode(st *parsing.State, header *model.Data) (labels int, dropFrame bool, err error) {
	field, found := header.String(timeCodeRate)
	m := timeCodeRegexp.FindStringSubmatch(field)
	if found && m != nil && (m[2] == "" || m[1] == "30" || m[1] == "60") {
		labels, _ = strconv.Atoi(m[1])
		return labels, m[2] != "", nil
	}
	msg := "The MCC file has no Time Code Rate line."
	if found {
		msg = `The MCC Time Code Rate "` + field + `" is unknown.`
	}
	if !st.Lenient() {
		return 0, false, &model.ParseError{Format: model.MCC, Msg: msg}
	}
	st.Warn(msg+" The parser read 30DF.", model.NoLine, model.NoBlock, nil, model.Repaired)
	return 30, true, nil
}

// frameRate returns the table row and the drop-frame flag of the Time Code Rate field. The CDP rate code of the
// first packet tells 23.976 from 24, 29.97 from 30 and 59.94 from 60.
func frameRate(st *parsing.State, header *model.Data, code int) (rate, bool, error) {
	labels, dropFrame, err := timeCode(st, header)
	if err != nil {
		return rate{}, false, err
	}
	for _, row := range rates {
		if row.labels == labels && row.code == code && (row.fps.Den != 1 || !dropFrame) {
			return row, dropFrame, nil
		}
	}
	name := strconv.Itoa(labels)
	if dropFrame {
		name = "29.97"
		if labels == 60 {
			name = "59.94"
		}
	}
	row, _ := rateNamed(name)
	return row, dropFrame, nil
}

// frames joins the packets of one time code in file order and returns them in time order. A packet at 100000
// hours or later fails its line.
func frames(st *parsing.State, lines []string, packets []rawPacket, row rate, dropFrame bool) ([]cea708.Frame, error) {
	fps := row.fps
	fps.DropFrame = dropFrame
	limit := fps.Frames(timecode.MaxTime)
	for fps.Duration(limit) >= timecode.MaxTime {
		limit--
	}
	for fps.Duration(limit+1) < timecode.MaxTime {
		limit++
	}
	limit++

	type placed struct {
		frame  int64
		packet int
	}
	order := make([]placed, 0, len(packets))
	for i, p := range packets {
		n := fps.FrameOf(p.timeCode())
		if n >= limit {
			_, err := parsing.BoundedTime(timecode.MaxTime, p.timeCode().String(), int(p.line))
			var perr *model.ParseError
			if errors.As(err, &perr) {
				if err := st.Fail(perr, int(p.line), model.NoBlock, []string{lines[p.line-1]}); err != nil {
					return nil, err
				}
			}
			continue
		}
		order = append(order, placed{n, i})
	}
	slices.SortStableFunc(order, func(a, b placed) int { return cmp.Compare(a.frame, b.frame) })

	out := make([]cea708.Frame, 0, len(order))
	for i := 0; i < len(order); {
		triplets := packets[order[i].packet].triplets
		j := i + 1
		for ; j < len(order) && order[j].frame == order[i].frame; j++ {
			triplets = append(triplets[:len(triplets):len(triplets)], packets[order[j].packet].triplets...)
		}
		out = append(out, cea708.Frame{Time: fps.Duration(order[i].frame), Triplets: triplets})
		i = j
	}
	return out, nil
}
