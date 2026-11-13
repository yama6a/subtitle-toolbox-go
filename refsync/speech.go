package refsync

import (
	"bytes"
	"iter"
	"math"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

var silenceLine = regexp.MustCompile(`\bsilence_(start|end):\s*(-?\d+(?:\.\d+)?)`)

// Interval is a time range of speech.
type Interval struct {
	Start time.Duration
	End   time.Duration
}

// SpeechFromSilenceLog returns a reference with one cue without text for the speech between the silences of an
// FFmpeg silencedetect log. mediaDuration is the length of the media. A time in the log that is below the one
// before it counts as the one before it.
// It returns a *model.InvalidArgumentError for a mediaDuration of 0 or less or of 100000 hours or more,
// and a *model.ParseError with an empty Format for a log that has a line per channel or has silence_start and
// silence_end lines out of order.
func SpeechFromSilenceLog(log []byte, mediaDuration time.Duration) (*model.Subtitle, error) {
	if mediaDuration <= 0 || mediaDuration >= timecode.MaxTime {
		return nil, &model.InvalidArgumentError{Msg: "The media duration must be greater than 0 and less than 100000 hours, got " +
			timecode.Number(mediaDuration.Seconds()) + "."}
	}
	limit := mediaDuration.Seconds()
	var intervals []Interval
	var speechStart, last time.Duration
	silent := false
	for n, line := range logLines(log) {
		m := silenceLine.FindSubmatch(line)
		if m == nil {
			continue
		}
		if bytes.Contains(line, []byte("channel:")) {
			return nil, &model.ParseError{Line: n + 1, Msg: "Cannot read a silencedetect log with one line per channel. " +
				"Run silencedetect without mono."}
		}
		kind := string(m[1])
		if (kind == "start") == silent {
			return nil, &model.ParseError{Line: n + 1, Msg: "The line silence_" + kind +
				" is out of order. The silence_start and silence_end lines must alternate."}
		}
		seconds, _ := strconv.ParseFloat(string(m[2]), 64)
		at := max(last, time.Duration(math.Round(min(max(seconds, 0), limit)*1000))*time.Millisecond)
		last = at
		if silent = kind == "start"; silent {
			intervals = append(intervals, Interval{speechStart, at})
		} else {
			speechStart = at
		}
	}
	if !silent {
		intervals = append(intervals, Interval{speechStart, mediaDuration})
	}
	intervals = slices.DeleteFunc(intervals, func(iv Interval) bool { return iv.End <= iv.Start })
	return SpeechFromIntervals(intervals)
}

// SpeechFromIntervals returns a reference with one cue without text for each interval, in start order.
// It returns a *model.InvalidArgumentError for an interval with a start below 0, an end before its start,
// or an end of 100000 hours or more.
func SpeechFromIntervals(intervals []Interval) (*model.Subtitle, error) {
	cues := make([]model.Cue, len(intervals))
	for i, iv := range intervals {
		if iv.Start < 0 || iv.End < iv.Start || iv.End >= timecode.MaxTime {
			return nil, &model.InvalidArgumentError{Msg: "Interval " + strconv.Itoa(i) +
				" must have a start of 0 or more and an end at or after its start, got " + timecode.Number(iv.Start.Seconds()) +
				" and " + timecode.Number(iv.End.Seconds()) + "."}
		}
		cues[i] = model.Cue{Start: iv.Start, End: iv.End}
	}
	var s model.Subtitle
	s.AddCues(cues...)
	return &s, nil
}

// logLines yields the lines of log and their 0-based numbers. CR LF, CR and LF each end a line.
func logLines(log []byte) iter.Seq2[int, []byte] {
	return func(yield func(int, []byte) bool) {
		for n := 0; ; n++ {
			i := bytes.IndexAny(log, "\r\n")
			if i < 0 {
				yield(n, log)
				return
			}
			if !yield(n, log[:i]) {
				return
			}
			next := i + 1
			if log[i] == '\r' && next < len(log) && log[next] == '\n' {
				next++
			}
			log = log[next:]
		}
	}
}
