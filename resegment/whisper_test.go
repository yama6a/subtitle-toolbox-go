package resegment_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/resegment"
)

func readWhisper(tb testing.TB) *model.Subtitle {
	tb.Helper()

	s, _, err := transcript.ReadWhisper(bytes.NewReader(testfiles.Read(tb, "resegmenting/own_whisper_long_segments.json")),
		model.CommonReadOptions{}, transcript.ReadOptions{WordTimestamps: true})
	require.NoError(tb, err)
	return s
}

func readFile[T any](tb testing.TB, name string, read func(*strings.Reader) (T, error)) T {
	tb.Helper()

	got, err := read(strings.NewReader(string(testfiles.Read(tb, "resegmenting/"+name))))
	require.NoError(tb, err)
	return got
}

func readVTTFile(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	return readFile(tb, name, func(r *strings.Reader) (*model.Subtitle, error) {
		s, _, err := webvtt.Read(r, model.CommonReadOptions{}, webvtt.ReadOptions{})
		return s, err
	})
}

func readSRTFile(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	return readFile(tb, name, func(r *strings.Reader) (*model.Subtitle, error) {
		s, _, err := srt.Read(r, model.CommonReadOptions{}, srt.ReadOptions{})
		return s, err
	})
}

type wordKey struct {
	Text  string
	Start time.Duration
}

type cueKey struct {
	Start, End time.Duration
	Lines      []string
	Words      []wordKey
}

func keys(s *model.Subtitle, withWords bool) []cueKey {
	var out []cueKey
	for _, c := range s.All() {
		k := cueKey{Start: c.Start, End: c.End, Lines: c.Lines}
		if withWords {
			for _, w := range c.Words {
				k.Words = append(k.Words, wordKey{w.Text, w.Start})
			}
		}
		out = append(out, k)
	}
	return out
}

func TestWhisperFixtureReadsWithWords(t *testing.T) {
	t.Parallel()

	s := readWhisper(t)
	assert.Equal(t, keys(readVTTFile(t, "own_whisper_long_segments_words.vtt"), true)[0].Words, keys(s, true)[0].Words)
	assert.Equal(t, 4, s.Len())
}

func TestWhisperFixtureSplitLong(t *testing.T) {
	t.Parallel()

	s := readWhisper(t)
	report, err := resegment.SplitLong(s, resegment.Options{})
	require.NoError(t, err)

	assert.Equal(t, resegment.Report{CuesBefore: 4, CuesAfter: 6}, report)
	got := keys(s, true)
	assert.Equal(t, got, keys(readVTTFile(t, "own_whisper_long_segments_split.vtt"), true))
	assert.Len(t, got[0].Words, 6)
	assert.Equal(t, wordKey{"Welcome", 0}, got[0].Words[0])
	assert.Equal(t, wordKey{"tour.", 2100 * time.Millisecond}, got[0].Words[5])
	assert.Equal(t, [2]time.Duration{2550 * time.Millisecond, 8550 * time.Millisecond}, [2]time.Duration{got[1].Start, got[1].End})
	assert.Equal(t, [2]time.Duration{20200 * time.Millisecond, 24 * time.Second}, [2]time.Duration{got[5].Start, got[5].End})
	assert.Len(t, got[5].Words, 9)
}

func TestWhisperFixtureByWords(t *testing.T) {
	t.Parallel()

	gap := 300 * time.Millisecond
	tests := []struct {
		name     string
		opts     resegment.Options
		expected string
		cues     int
	}{
		{"defaults", resegment.Options{}, "own_whisper_long_segments_resegmented.srt", 6},
		{
			"30 x 1 and a gap of 0.3 s",
			resegment.Options{Limits: model.CueLimits{MaxCharactersPerLine: 30, MaxLinesPerCue: 1}, MaxWordGap: &gap},
			"own_whisper_long_segments_resegmented_30x1.srt", 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readWhisper(t)
			report, err := resegment.ByWords(s, tt.opts)
			require.NoError(t, err)

			assert.Equal(t, keys(readSRTFile(t, tt.expected), false), keys(s, false))
			if tt.cues > 0 {
				assert.Equal(t, resegment.Report{CuesBefore: 4, CuesAfter: tt.cues}, report)
			}
		})
	}

	t.Run("2 source cues join", func(t *testing.T) {
		t.Parallel()

		s := readWhisper(t)
		_, err := resegment.ByWords(s, resegment.Options{})
		require.NoError(t, err)
		third, _ := s.Cue(3)
		last, _ := s.Cue(5)
		assert.Equal(t, [2]time.Duration{12400 * time.Millisecond, 18200 * time.Millisecond}, [2]time.Duration{third.Start, third.End})
		assert.Equal(t, []string{"The cafe on the ground floor opens at", "nine and closes at six every weekday."}, third.Lines)
		assert.Equal(t, [2]time.Duration{22520 * time.Millisecond, 24 * time.Second}, [2]time.Duration{last.Start, last.End})
		assert.Equal(t, "Then let us begin.", last.Text())
		assert.Len(t, last.Words, 4)
	})
}

func TestWhisperFixtureWordsStayWithTheirText(t *testing.T) {
	t.Parallel()

	runs := map[string]func(*model.Subtitle, resegment.Options) (resegment.Report, error){"SplitLong": resegment.SplitLong, "ByWords": resegment.ByWords}
	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			s := readWhisper(t)
			_, err := run(s, resegment.Options{Limits: model.CueLimits{MaxCharactersPerLine: 20, MaxLinesPerCue: 1}})
			require.NoError(t, err)
			for i, c := range s.All() {
				require.NoError(t, c.ValidateWords(), "cue %d", i)
				require.NotEmpty(t, c.Words, "cue %d", i)
				assert.Equal(t, c.Start, c.Words[0].Start, "cue %d", i)
				for _, w := range c.Words {
					assert.GreaterOrEqual(t, w.Start, c.Start, "cue %d", i)
					assert.Less(t, w.Start, c.End, "cue %d", i)
				}
			}
		})
	}
}
