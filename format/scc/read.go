package scc

import (
	"cmp"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// padding is the null byte pair with odd parity that fills frames without data.
const padding = 0x8080

var (
	codeLineRegexp = regexp.MustCompile(`^(\d{2,6}):(\d{2}):(\d{2})([:;])(\d{2})(?:[\t\n\v\f\r ]+(.*))?$`)
	bytePairRegexp = regexp.MustCompile(`^[0-9a-fA-F]{4}$`)
)

// Read reads SCC from UTF-8 text.
func Read(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.SCC, err)
	}
	st := parsing.NewState(model.SCC, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	codeLines, dropFrame, err := readCodeLines(st, string(content))
	if err != nil {
		return nil, nil, err
	}
	if dropFrame != nil {
		_ = s.SetFormatData(formatDataKey, model.NewData(model.Field{Key: "dropFrame", Value: *dropFrame})) // the data has the shape of the schema
	}
	s.AddCues(caption.CEA608Cues(codeLines, caption.CEA608Options{Channel: max(opts.Channel, 1), RollUp: opts.RollUp}, st.LastCueDuration())...)
	return s, st.Warnings(), nil
}

// readCodeLines returns the code lines in time order and the drop-frame flag of the first code line.
// The flag is nil when the file has no code line.
func readCodeLines(st *parsing.State, content string) ([]cea608.CodeLine, *bool, error) {
	var codeLines []cea608.CodeLine
	var dropFrame *bool
	block := -1
	for n, raw := range parsing.Lines(content) {
		raw = strings.Trim(raw, markupcore.PHPTrimSet)
		if raw == "" {
			continue
		}
		block++
		line := n + 1
		if block == 0 {
			if raw != header {
				return nil, nil, &model.ParseError{Format: model.SCC, Line: line, Msg: `An SCC file must start with the line "` + header + `".`}
			}
			continue
		}
		m, start, err := matchCodeLine(st, raw, line, block)
		if err != nil {
			return nil, nil, err
		}
		if m == nil {
			continue
		}
		drop := m[4] == ";"
		if dropFrame == nil {
			dropFrame = &drop
		}
		var words []uint16
		for _, pair := range strings.FieldsFunc(m[6], isSpace) {
			word, err := readBytePair(st, pair, line, block, raw)
			if err != nil {
				return nil, nil, err
			}
			words = append(words, word)
		}
		codeLines = append(codeLines, cea608.CodeLine{Start: start, Words: words})
	}
	slices.SortStableFunc(codeLines, func(a, b cea608.CodeLine) int { return cmp.Compare(a.Start, b.Start) })
	return codeLines, dropFrame, nil
}

// matchCodeLine returns the fields and the start frame of a code line. Each byte pair takes one frame, so the last pair must stay below 100000 hours too.
// The fields are nil when lenient mode skips the line.
func matchCodeLine(st *parsing.State, raw string, line, block int) (m []string, start int64, err error) {
	m = codeLineRegexp.FindStringSubmatch(raw)
	if m == nil {
		err = st.Fail(&model.ParseError{Line: line, Msg: `The SCC line "` + raw + `" does not start with a time code.`}, line, block, []string{raw})
		return nil, 0, err
	}
	dropFrame := m[4] == ";"
	start = startFrame(m, dropFrame)
	pairs := len(strings.FieldsFunc(m[6], isSpace))
	if start+int64(max(pairs-1, 0)) >= parsing.FrameLimit(frameRate(dropFrame)) {
		err = st.Fail(parsing.OverLimit(m[1]+":"+m[2]+":"+m[3]+m[4]+m[5], line), line, block, []string{raw})
		return nil, 0, err
	}
	return m, start, nil
}

// isSpace matches the white space of the PHP pattern \s.
func isSpace(r rune) bool {
	return strings.ContainsRune("\t\n\v\f\r ", r)
}

func startFrame(m []string, dropFrame bool) int64 {
	var field [4]int
	for i, v := range []string{m[1], m[2], m[3], m[5]} {
		field[i], _ = strconv.Atoi(v) // two digits
	}
	return frameRate(dropFrame).FrameOf(model.Timecode{Hours: field[0], Minutes: field[1], Seconds: field[2], Frames: field[3]})
}

// readBytePair returns the word of a pair. A bad pair fails in strict mode. Lenient mode warns and returns padding,
// so the later pairs of the line keep their frames.
func readBytePair(st *parsing.State, pair string, line, block int, raw string) (uint16, error) {
	if !bytePairRegexp.MatchString(pair) {
		err := st.Fail(&model.ParseError{Line: line, Msg: `The SCC line has the invalid byte pair "` + pair + `".`}, line, block, []string{raw})
		return padding, err
	}
	word, _ := strconv.ParseUint(pair, 16, 16) // four hex digits
	return uint16(word), nil
}

func frameRate(dropFrame bool) model.FrameRate {
	if dropFrame {
		return model.FrameRate2997DF
	}
	return model.FrameRate2997
}
