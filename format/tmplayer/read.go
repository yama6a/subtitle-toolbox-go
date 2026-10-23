package tmplayer

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var lineRegexp = regexp.MustCompile(`^(\d+):([0-5]\d):([0-5]\d)(?:,(\d+))?[:=](.*)$`)

// entry is one line of the file. An entry without lines ends the cue before it.
type entry struct {
	time  time.Duration
	lines []string
}

// Read reads TMPlayer and TMPlayer+ text. It expects UTF-8, as stbox.Parse passes it.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.TMPlayer, err)
	}
	st := parsing.NewState(model.TMPlayer, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	entries, err := readEntries(st, string(content))
	if err != nil {
		return nil, nil, err
	}
	s.AddCues(entriesToCues(st, entries)...)
	return s, st.Warnings(), nil
}

func readEntries(st *parsing.State, content string) ([]entry, error) {
	var entries []entry
	block := -1
	for n, raw := range parsing.Lines(content) {
		raw = strings.Trim(raw, markupcore.PHPTrimSet)
		if raw == "" {
			continue
		}
		block++
		e, number, err := parseLine(raw, n+1)
		if err != nil {
			if err := failLine(st, err, n+1, block, raw); err != nil {
				return nil, err
			}
			continue
		}
		// TMPlayer+ writes each line of a cue as its own entry, with the line number after a comma.
		if last := len(entries) - 1; number > 1 && last >= 0 && entries[last].time == e.time {
			entries[last].lines = append(entries[last].lines, e.lines...)
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func failLine(st *parsing.State, err error, line, block int, raw string) error {
	var perr *model.ParseError
	if !errors.As(err, &perr) {
		return err
	}
	return st.Fail(perr, line, block, []string{raw})
}

// entriesToCues sorts the entries by time. An entry with text becomes a cue that ends at the next entry.
func entriesToCues(st *parsing.State, entries []entry) []model.Cue {
	slices.SortStableFunc(entries, func(a, b entry) int { return cmp.Compare(a.time, b.time) })
	var cues []model.Cue
	for i, e := range entries {
		if len(e.lines) == 0 {
			continue
		}
		end := st.LastCueEnd(e.time)
		if i+1 < len(entries) {
			end = entries[i+1].time
		}
		cues = append(cues, model.Cue{Start: e.time, End: end, Lines: e.lines})
	}
	return cues
}

// parseLine returns the entry of raw and its line number, 0 when the line has none.
func parseLine(raw string, line int) (entry, int64, error) {
	m := lineRegexp.FindStringSubmatch(raw)
	if m == nil {
		return entry{}, 0, &model.ParseError{Line: line, Msg: `The line "` + raw + `" is not a TMPlayer line.`}
	}
	// A digit run too long for int64 counts as the limit. The check comes before the multiplication, so no time wraps.
	hours, _ := strconv.ParseInt(m[1], 10, 64)
	d := timecode.MaxTime
	if hours < timecode.MaxHours {
		minutes, _ := strconv.Atoi(m[2])
		seconds, _ := strconv.Atoi(m[3])
		d = time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	}
	d, err := parsing.BoundedTime(d, m[1]+":"+m[2]+":"+m[3], line)
	if err != nil {
		return entry{}, 0, err
	}
	number, _ := strconv.ParseInt(m[4], 10, 64)
	var lines []string
	for _, part := range parsing.PipeLines(m[5]) {
		lines = append(lines, markupcore.EscapeText(part))
	}
	return entry{time: d, lines: lines}, number, nil
}
