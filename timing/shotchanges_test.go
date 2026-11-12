package timing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/timing"
)

func TestShotChangeReaders(t *testing.T) {
	t.Parallel()
	ms := func(n ...int) []time.Duration {
		out := make([]time.Duration, 0, len(n))
		for _, v := range n {
			out = append(out, time.Duration(v)*time.Millisecond)
		}
		return out
	}
	const bad = "The shot change time %q is not valid. Use seconds or hh:mm:ss.mmm."
	tests := []struct {
		name     string
		read     func([]byte) ([]time.Duration, error)
		input    string
		want     []time.Duration
		wantLine int
		wantText string
	}{
		{"log sorted and unique", timing.ShotChangesFromFFmpegLog, "pts_time:8.5 pts_time:3 pts_time:8.5", ms(3000, 8500), 0, ""},
		{"log without pts_time", timing.ShotChangesFromFFmpegLog, "frame=1 pts:3", nil, 0, ""},
		{"log spaces, rounding and word boundary", timing.ShotChangesFromFFmpegLog, "x pts_time: 1.2345 npts_time:2\nspts_time:3", ms(1235), 0, ""},
		{"log at the limit", timing.ShotChangesFromFFmpegLog, "a\npts_time:1\npts_time:360000000", nil, 3, "360000000"},
		{"log rounds up to the limit", timing.ShotChangesFromFFmpegLog, "pts_time:359999999.9996", nil, 1, "359999999.9996"},
		{"text mixed", timing.ShotChangesFromText, "\xef\xbb\xbf01:00:00.040\r\n\r\n  2  \r\n00:00:01\r\n", ms(1000, 2000, 3600040), 0, ""},
		{"text lone CR and NUL", timing.ShotChangesFromText, "1.5\r\x002.25\x00\r0:00:01.0005", ms(1001, 1500, 2250), 0, ""},
		{"text rounds half a millisecond up", timing.ShotChangesFromText, "0.0205", ms(21), 0, ""},
		{"text bad line", timing.ShotChangesFromText, "1.5\n\n00:01:2x\n", nil, 3, "00:01:2x"},
		{"text bad shapes", timing.ShotChangesFromText, "1.\n", nil, 1, "1."},
		{"text minutes over 59", timing.ShotChangesFromText, "00:60:00\n", nil, 1, "00:60:00"},
		{"text 100000 hours", timing.ShotChangesFromText, "99999:59:59.999\n100000:00:00", nil, 2, "100000:00:00"},
		{"text many hour digits", timing.ShotChangesFromText, "00000000000000000001:00:00\n99999999999999999999:00:00", nil, 2, "99999999999999999999:00:00"},
		{"text seconds at the limit", timing.ShotChangesFromText, "360000000", nil, 1, "360000000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.read([]byte(tt.input))

			if tt.wantLine == 0 {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
				return
			}
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			require.ErrorIs(t, err, model.ErrParse)
			assert.Empty(t, perr.Format)
			assert.Equal(t, tt.wantLine, perr.Line)
			assert.Equal(t, sprintf(bad, tt.wantText), perr.Msg)
		})
	}
}

func TestShotChangeFiles(t *testing.T) {
	t.Parallel()
	want := []time.Duration{12500 * time.Millisecond, 62500 * time.Millisecond, 70 * time.Second, 80 * time.Second, 91250 * time.Millisecond}

	fromLog, err := timing.ShotChangesFromFFmpegLog(testfiles.Read(t, "shot-changes/own_ffmpeg_showinfo.log"))
	require.NoError(t, err)
	fromText, err := timing.ShotChangesFromText(testfiles.Read(t, "shot-changes/own_scenes.txt"))
	require.NoError(t, err)

	assert.Equal(t, want, fromLog)
	assert.Equal(t, want, fromText)
}
