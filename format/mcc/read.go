package mcc

import (
	"cmp"
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
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

var (
	versionRegexp  = regexp.MustCompile(`^` + versionPrefix + `(\d+\.\d+)$`)
	fieldRegexp    = regexp.MustCompile(`^([A-Za-z][A-Za-z ]*)=(.*)$`)
	dataRegexp     = regexp.MustCompile(`^(\d{2,6}):(\d{2}):(\d{2})[:;](\d{2})(?:\.[01](?:,\d+)?)?[ \t]+(\S+)$`)
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

	cues := caption.CaptionDataCues(frames(packets, row, dropFrame), opts.cea708Service(),
		caption.CEA608Options{Channel: max(opts.Channel, 1), RollUp: opts.RollUp}, st.LastCueDuration())
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

func trimmedLines(content string) []string {
	var lines []string
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

// readLines returns the header fields, the packets that hold a CDP and the rate code of the first of them, or -1.
func readLines(st *parsing.State, lines []string) (*model.Data, []rawPacket, int, error) {
	header := &model.Data{}
	var packets []rawPacket
	rateCode := -1
	for i, line := range lines {
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, versionPrefix) {
			continue
		}
		if m := fieldRegexp.FindStringSubmatch(line); m != nil {
			header.Set(strings.Trim(m[1], markupcore.PHPTrimSet), strings.Trim(m[2], markupcore.PHPTrimSet))
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
		if code >= 0 {
			if rateCode < 0 {
				rateCode = code
			}
			packets = append(packets, rawPacket{tc, triplets})
		}
	}
	return header, packets, rateCode, nil
}

type rawPacket struct {
	timeCode model.Timecode
	triplets []cea708.Triplet
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
	m := dataRegexp.FindStringSubmatch(line)
	if m == nil {
		return 0, tc, nil, `The MCC line "` + line + `" does not start with a time code.`
	}
	data, msg := decodeHex(m[5])
	if msg != "" {
		return 0, tc, nil, msg
	}
	var f [4]int
	for i, v := range []string{m[1], m[2], m[3], m[4]} {
		f[i], _ = strconv.Atoi(v) // two digits
	}
	tc = model.Timecode{Hours: f[0], Minutes: f[1], Seconds: f[2], Frames: f[3]}
	if len(data) < 3 || data[0] != 0x61 || data[1] != 0x01 {
		return -1, tc, nil, ""
	}
	code, triplets, err := cea708.ReadCDP(data[3:min(3+int(data[2]), len(data))])
	if err != nil {
		return 0, tc, nil, err.Error()
	}
	return code, tc, triplets, ""
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

// frames joins the packets of one time code in file order and returns them in time order.
func frames(packets []rawPacket, row rate, dropFrame bool) []cea708.Frame {
	fps := row.fps
	fps.DropFrame = dropFrame
	byFrame := map[int64][]cea708.Triplet{}
	for _, p := range packets {
		if len(p.triplets) > 0 {
			n := fps.FrameOf(p.timeCode)
			byFrame[n] = append(byFrame[n], p.triplets...)
		}
	}
	out := make([]cea708.Frame, 0, len(byFrame))
	for n, triplets := range byFrame {
		out = append(out, cea708.Frame{Time: fps.Duration(n), Triplets: triplets})
	}
	slices.SortFunc(out, func(a, b cea708.Frame) int { return cmp.Compare(a.Time, b.Time) })
	return out
}
