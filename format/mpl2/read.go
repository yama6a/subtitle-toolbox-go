package mpl2

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const tenth = 100 * time.Millisecond

var cueRegexp = regexp.MustCompile(`^\[(\d+)\]\[(\d+)\](.*)$`)

// Read reads MPL2 text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.MPL2, err)
	}
	st := parsing.NewState(model.MPL2, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	var cues []model.Cue
	block := 0
	for n, raw := range parsing.Lines(string(content)) {
		raw = strings.Trim(raw, markupcore.PHPTrimSet)
		if raw == "" {
			continue
		}
		cue, err := parseLine(st, raw, n+1, block)
		if err != nil {
			var perr *model.ParseError
			if !errors.As(err, &perr) {
				return nil, nil, err
			}
			if err := st.Fail(perr, n+1, block, []string{raw}); err != nil {
				return nil, nil, err
			}
		} else {
			cues = append(cues, cue)
		}
		block++
	}
	s.AddCues(cues...)
	return s, st.Warnings(), nil
}

func parseLine(st *parsing.State, raw string, line, block int) (model.Cue, error) {
	m := cueRegexp.FindStringSubmatch(raw)
	if m == nil {
		return model.Cue{}, &model.ParseError{Line: line, Msg: `The line "` + raw + `" is not an MPL2 cue.`}
	}
	start, err := tenths(m[1], line)
	if err != nil {
		return model.Cue{}, err
	}
	end, err := tenths(m[2], line)
	if err != nil {
		return model.Cue{}, err
	}
	start, end, err = st.OrderedTimes(start, end, line, block, []string{raw}, true)
	if err != nil {
		return model.Cue{}, err
	}
	return model.Cue{Start: start, End: end, Lines: parseText(m[3])}, nil
}

// tenths reads a run of digits as tenths of a second. A run too long for int64 counts as the limit.
func tenths(digits string, line int) (time.Duration, error) {
	n, _ := strconv.ParseInt(digits, 10, 64)
	d := timecode.MaxTime
	if n < int64(timecode.MaxTime/tenth) {
		d = time.Duration(n) * tenth
	}
	return parsing.BoundedTime(d, "["+digits+"]", line)
}

func parseText(text string) []string {
	var lines []string
	for _, part := range parsing.PipeLines(text) {
		if rest, italic := strings.CutPrefix(part, "/"); italic {
			if rest = strings.Trim(rest, markupcore.PHPTrimSet); rest != "" {
				lines = append(lines, "<i>"+markupcore.EscapeText(rest)+"</i>")
			}
		} else {
			lines = append(lines, markupcore.EscapeText(part))
		}
	}
	return lines
}
