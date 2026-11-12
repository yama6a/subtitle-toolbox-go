package hls_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/hls"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

func TestParseTimestampMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		want    hls.TimestampMap
		wantErr string
	}{
		{"the RFC order", "X-TIMESTAMP-MAP=LOCAL:01:00:00.000,MPEGTS:324000000", hls.TimestampMap{MPEGTS: 324000000, Local: time.Hour}, ""},
		{"MPEGTS first, a short LOCAL time, spaces", " X-TIMESTAMP-MAP=MPEGTS:181083, LOCAL:01:02.345 ", hls.TimestampMap{MPEGTS: 181083, Local: ms(62345)}, ""},
		{"lower case names", "X-TIMESTAMP-MAP=mpegts:5,local:00:01.000", hls.TimestampMap{MPEGTS: 5, Local: ms(1000)}, ""},
		{"3 hour digits", "X-TIMESTAMP-MAP=LOCAL:100:00:00.500,MPEGTS:0", hls.TimestampMap{Local: 100*time.Hour + ms(500)}, ""},
		{"2^33 parses and fails Validate", "X-TIMESTAMP-MAP=LOCAL:00:00.000,MPEGTS:8589934592", hls.TimestampMap{MPEGTS: hls.MPEGTSWrap}, ""},
		{"another header", "WEBVTT", hls.TimestampMap{}, `The line "WEBVTT" does not start with X-TIMESTAMP-MAP=.`},
		{"no MPEGTS", "X-TIMESTAMP-MAP=LOCAL:00:00:00.000", hls.TimestampMap{}, `The X-TIMESTAMP-MAP header "X-TIMESTAMP-MAP=LOCAL:00:00:00.000" needs a LOCAL cue time and an integer MPEGTS value.`},
		{"a LOCAL time with 1 minute digit", "X-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:0:00.000", hls.TimestampMap{}, `The X-TIMESTAMP-MAP header "X-TIMESTAMP-MAP=MPEGTS:900000,LOCAL:0:00.000" needs a LOCAL cue time and an integer MPEGTS value.`},
		{"a negative MPEGTS", "X-TIMESTAMP-MAP=MPEGTS:-1,LOCAL:00:00.000", hls.TimestampMap{}, `The X-TIMESTAMP-MAP header "X-TIMESTAMP-MAP=MPEGTS:-1,LOCAL:00:00.000" needs a LOCAL cue time and an integer MPEGTS value.`},
		{"a LOCAL time at 100000 hours", "X-TIMESTAMP-MAP=MPEGTS:0,LOCAL:100000:00:00.000", hls.TimestampMap{}, `The X-TIMESTAMP-MAP header "X-TIMESTAMP-MAP=MPEGTS:0,LOCAL:100000:00:00.000" needs a LOCAL cue time and an integer MPEGTS value.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := hls.ParseTimestampMap(tt.line)
			if tt.wantErr != "" {
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, model.WebVTT, perr.Format)
				assert.Equal(t, tt.wantErr, perr.Msg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTimestampMapValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		m       hls.TimestampMap
		option  string
		wantMsg string
	}{
		{"largest MPEGTS", hls.TimestampMap{MPEGTS: hls.MPEGTSWrap - 1}, "", ""},
		{"MPEGTS 2^33", hls.TimestampMap{MPEGTS: hls.MPEGTSWrap}, "hls.TimestampMap.MPEGTS", "The MPEGTS value must be a 33-bit timestamp from 0 to 8589934591, got 8589934592."},
		{"negative MPEGTS", hls.TimestampMap{MPEGTS: -1}, "hls.TimestampMap.MPEGTS", "The MPEGTS value must be a 33-bit timestamp from 0 to 8589934591, got -1."},
		{"negative LOCAL", hls.TimestampMap{Local: -1500 * time.Millisecond}, "hls.TimestampMap.Local", "The LOCAL time must not be negative, got -1.5."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.m.Validate()
			if tt.wantMsg == "" {
				require.NoError(t, err)
				return
			}
			var ierr *model.InvalidArgumentError
			require.ErrorAs(t, err, &ierr)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Equal(t, tt.option, ierr.Option)
			assert.Equal(t, tt.wantMsg, ierr.Msg)
		})
	}
}

func TestTimestampMapHeader(t *testing.T) {
	t.Parallel()

	assert.Equal(t, defaultMap, hls.TimestampMap{MPEGTS: hls.DefaultMPEGTS}.Header())
	assert.Equal(t, "X-TIMESTAMP-MAP=LOCAL:01:01:01.001,MPEGTS:0", hls.TimestampMap{Local: time.Hour + time.Minute + time.Second + ms(1)}.Header())
	assert.Equal(t, "X-TIMESTAMP-MAP=LOCAL:100:00:00.000,MPEGTS:1", hls.TimestampMap{MPEGTS: 1, Local: 100 * time.Hour}.Header())
}

func TestTimestampMapOffset(t *testing.T) {
	t.Parallel()

	const second = 90000
	tests := []struct {
		name        string
		m           hls.TimestampMap
		streamStart int64
		want        time.Duration
	}{
		{"same start", hls.TimestampMap{MPEGTS: 900000}, 900000, 0},
		{"stream starts earlier", hls.TimestampMap{MPEGTS: 900000}, 0, 10 * time.Second},
		{"LOCAL at the stream start", hls.TimestampMap{MPEGTS: 324000000, Local: time.Hour}, 324000000, -time.Hour},
		{"LOCAL and MPEGTS agree", hls.TimestampMap{MPEGTS: 324000000, Local: time.Hour}, 0, 0},
		{"a half millisecond tick count converts exactly", hls.TimestampMap{MPEGTS: 45 * 9}, 0, 4500 * time.Microsecond},
		{"start before the wrap", hls.TimestampMap{MPEGTS: second}, hls.MPEGTSWrap - second, 2 * time.Second},
		{"MPEGTS before the wrap", hls.TimestampMap{MPEGTS: hls.MPEGTSWrap - second}, second, -2 * time.Second},
		{"2^32 ticks count as a wrap", hls.TimestampMap{MPEGTS: hls.MPEGTSWrap / 2}, 0, -hls.MPEGTSWrap / 2 * 100000 / 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, tt.m.Offset(tt.streamStart))
		})
	}
}

func TestTimestampMapOf(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    hls.TimestampMap
		found   bool
		wantErr bool
	}{
		{"shaka segment", string(readFixtureText(t, "shaka-vtt-071.vtt")), hls.TimestampMap{MPEGTS: 324000000, Local: time.Hour}, true, false},
		{"no map", "WEBVTT\n", hls.TimestampMap{}, false, false},
		{"the first of 2 maps", "WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00.000,MPEGTS:1\nX-TIMESTAMP-MAP=LOCAL:00:00.000,MPEGTS:2\n", hls.TimestampMap{MPEGTS: 1}, true, false},
		{"a broken map", "WEBVTT\nX-TIMESTAMP-MAP=LOCAL:00:00.000\n", hls.TimestampMap{}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, found, err := hls.TimestampMapOf(readVTT(t, tt.content))
			if tt.wantErr {
				require.ErrorIs(t, err, model.ErrParse)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.found, found)
			assert.Equal(t, tt.want, got)
		})
	}
}
