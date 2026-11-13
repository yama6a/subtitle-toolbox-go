package refsync_test

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/refsync"
)

const logLine = "[silencedetect @ 0x55f1d2a8b340] "

func sec(f float64) time.Duration { return time.Duration(math.Round(f*1000)) * time.Millisecond }

func intervalsOf(s *model.Subtitle) []refsync.Interval {
	var out []refsync.Interval
	for _, c := range s.All() {
		out = append(out, refsync.Interval{Start: c.Start, End: c.End})
		if len(c.Lines) != 0 {
			panic("a speech cue has lines")
		}
	}
	return out
}

func spans(pairs ...float64) []refsync.Interval {
	var out []refsync.Interval
	for pair := range slices.Chunk(pairs, 2) {
		out = append(out, refsync.Interval{Start: sec(pair[0]), End: sec(pair[1])})
	}
	return out
}

// Adapted from SpeechReferenceTest.
func TestSpeechFromSilenceLog(t *testing.T) {
	t.Parallel()

	base := logLine + "silence_start: -0.0213\n" +
		logLine + "silence_end: 2.5 | silence_duration: 2.5213\r\n" +
		logLine + "silence_start: 4.25\r" +
		logLine + "silence_end: 6 | silence_duration: 1.75\n"
	tests := []struct {
		name     string
		log      string
		duration time.Duration
		want     []refsync.Interval
		line     int
		invalid  bool
	}{
		{name: "CR LF and CR end lines", log: base, duration: 10 * time.Second, want: spans(2.5, 4.25, 6, 10)},
		{name: "an open silence runs to the end", log: base + logLine + "silence_start: 6.5\n", duration: 10 * time.Second, want: spans(2.5, 4.25, 6, 6.5)},
		{name: "no silence", log: "ffmpeg version 6.1.1\n", duration: 12500 * time.Millisecond, want: spans(0, 12.5)},
		{name: "a time past the end", log: logLine + "silence_start: 99999999999999999999999\n", duration: 10 * time.Second, want: spans(0, 10)},
		{name: "a time that goes back", log: logLine + "silence_start: 5\n" + logLine + "silence_end: 3\n" + logLine + "silence_start: 4\n", duration: 10 * time.Second, want: spans(0, 5)},
		{name: "a line per channel", log: "Press [q] to stop\n" + logLine + "channel: 0 | silence_start: 1.5\n", duration: 10 * time.Second, line: 2},
		{name: "an end first", log: logLine + "silence_end: 1.5 | silence_duration: 1.5\n", duration: 10 * time.Second, line: 1},
		{name: "2 starts", log: logLine + "silence_start: 1\n" + logLine + "silence_start: 2\n", duration: 10 * time.Second, line: 2},
		{name: "no duration", log: "", invalid: true},
		{name: "a duration of 100000 hours", log: "", duration: 100000 * time.Hour, invalid: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := refsync.SpeechFromSilenceLog([]byte(tt.log), tt.duration)
			switch {
			case tt.invalid:
				require.ErrorIs(t, err, model.ErrInvalidArgument)
			case tt.line > 0:
				pe, ok := errors.AsType[*model.ParseError](err)
				require.True(t, ok, "%v", err)
				assert.Equal(t, model.Format(""), pe.Format)
				assert.Equal(t, tt.line, pe.Line)
				require.ErrorIs(t, err, model.ErrParse)
			default:
				require.NoError(t, err)
				assert.Equal(t, tt.want, intervalsOf(s))
			}
		})
	}
}

func TestSpeechFromSilenceLogMessages(t *testing.T) {
	t.Parallel()

	_, err := refsync.SpeechFromSilenceLog(nil, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "The media duration must be greater than 0")

	_, err = refsync.SpeechFromSilenceLog([]byte(logLine+"silence_end: 1\n"), time.Second)
	assert.Contains(t, err.Error(), "The line silence_end is out of order. The silence_start and silence_end lines must alternate.")

	_, err = refsync.SpeechFromSilenceLog([]byte(logLine+"channel: 0 | silence_start: 1\n"), time.Second)
	assert.Contains(t, err.Error(), "Cannot read a silencedetect log with one line per channel. Run silencedetect without mono.")
}

func TestSpeechFromSilenceLogRealLog(t *testing.T) {
	t.Parallel()

	s, err := refsync.SpeechFromSilenceLog(testfiles.Read(t, "sync/own_ffmpeg_silencedetect.log"), 840*time.Second)
	require.NoError(t, err)
	got := intervalsOf(s)
	require.Len(t, got, 60)
	assert.Equal(t, spans(77.804, 79.067)[0], got[0])
	assert.Equal(t, spans(800.876, 803.915)[0], got[59])
}

func TestSpeechFromIntervals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []refsync.Interval
		want []refsync.Interval
		msg  string
	}{
		{name: "sorted", in: spans(5.5, 7, 1, 2.25, 3, 4), want: spans(1, 2.25, 3, 4, 5.5, 7)},
		{name: "none", in: nil, want: nil},
		{name: "a zero length interval stays", in: spans(2, 2), want: spans(2, 2)},
		{name: "the end is before the start", in: spans(1, 2, 2, 1), msg: "Interval 1 must have a start of 0 or more and an end at or after its start, got 2 and 1."},
		{name: "a negative start", in: spans(-1, 2), msg: "Interval 0 must have a start of 0 or more and an end at or after its start, got -1 and 2."},
		{name: "an end of 100000 hours", in: []refsync.Interval{{End: 100000 * time.Hour}}, msg: "Interval 0 must have"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, err := refsync.SpeechFromIntervals(tt.in)
			if tt.msg != "" {
				require.ErrorIs(t, err, model.ErrInvalidArgument)
				assert.Contains(t, err.Error(), tt.msg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, intervalsOf(s))
		})
	}
}

// Adapted from SpeechReferenceTest::testSyncToSpeechFindsOffsetAndScale.
func TestApplyToTheSpeech(t *testing.T) {
	t.Parallel()

	target := load(t, "own_target_de_25fps.srt")
	speech, err := refsync.SpeechFromSilenceLog(testfiles.Read(t, "sync/own_ffmpeg_silencedetect.log"), 840*time.Second)
	require.NoError(t, err)

	r, err := refsync.Apply(target, refsync.Options{Reference: speech})
	require.NoError(t, err)
	assert.InDelta(t, frameRateScale, r.Scale, 0.00001)
	assert.InDelta(t, -2.3, r.Offset.Seconds(), 0.15)
	assert.Greater(t, r.Score, 0.7)
	want := load(t, "own_target_de_synced.srt")
	assert.Equal(t, want.Cues(), target.Cues())
}

// logOf1MB returns a log of 1 MB with a silence of 1 s every 2 s.
func logOf1MB() ([]byte, time.Duration) {
	var b strings.Builder
	var n int
	for ; b.Len() < 1<<20; n++ {
		at := strconv.Itoa(2 * n)
		b.WriteString(logLine + "silence_start: " + at + ".5\n" + logLine + "silence_end: " + strconv.Itoa(2*n+1) + " | silence_duration: 0.5\n")
	}
	return []byte(b.String()), time.Duration(2*n+2) * time.Second
}

func BenchmarkSpeechFromSilenceLog(b *testing.B) {
	log, duration := logOf1MB()
	b.SetBytes(int64(len(log)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := refsync.SpeechFromSilenceLog(log, duration); err != nil {
			b.Fatal(err)
		}
	}
}

func FuzzSpeechFromSilenceLog(f *testing.F) {
	f.Add([]byte(logLine+"silence_start: -0.0213\n"+logLine+"silence_end: 2.5 | silence_duration: 2.5213\r\n"+logLine+"silence_start: 4.25\r"), int64(10_000))
	f.Add([]byte("Press [q] to stop\n"+logLine+"channel: 0 | silence_start: 1.5\n"), int64(10_000))
	f.Add([]byte(logLine+"silence_end: 1.5\nsilence_start: 99999999999999999999\n"), int64(1))
	f.Add([]byte("silence_start: 5\nsilence_end: 3\nsilence_start: 4\n"), int64(36_000_000))
	f.Fuzz(func(t *testing.T, log []byte, ms int64) {
		duration := time.Duration(1+uint64(ms)%36_000_000) * time.Millisecond
		s, err := refsync.SpeechFromSilenceLog(log, duration)
		if err != nil {
			assert.True(t, errors.Is(err, model.ErrParse) || errors.Is(err, model.ErrInvalidArgument), "%v", err)
			return
		}
		var end time.Duration
		for _, c := range s.All() {
			assert.GreaterOrEqual(t, c.Start, end)
			assert.Greater(t, c.End, c.Start)
			end = c.End
		}
		assert.LessOrEqual(t, end, duration+time.Millisecond)
	})
}
