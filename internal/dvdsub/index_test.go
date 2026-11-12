package dvdsub_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/dvdsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	first   = "# VobSub index file, v7 (do not modify this line!)\n"
	palette = "palette: 000000, 100000, 200000, 300000, 400000, 500000, 600000, 700000, 800000, 900000, a00000, b00000, c00000, d00000, e00000, f00000\n"
)

func TestParseIndexErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		idx  string
		want string
	}{
		{"no first line", "size: 720x576\n" + palette, `The .idx content does not start with the "VobSub index file" line.`},
		{"empty content", "", `The .idx content does not start with the "VobSub index file" line.`},
		{"no size line", first + palette, "The .idx content has no size line."},
		{"size without x", first + "size: 720\n" + palette, `The .idx size line "size: 720" is not valid.`},
		{"size of 0", first + "size: 0x576\n" + palette, `The .idx size line "size: 0x576" is not valid.`},
		{"no palette and no custom colors", first + "size: 720x576\n", "The .idx content has no palette line."},
		{"custom colors that are off", first + "size: 720x576\ncustom colors: OFF, tridx: 0000, colors: 000000, 000000, 000000, 000000\n", "The .idx content has no palette line."},
		{"palette of 15 colors", first + "size: 720x576\npalette: 000000, 100000, 200000, 300000, 400000, 500000, 600000, 700000, 800000, 900000, a00000, b00000, c00000, d00000, e00000\n", `The .idx line "palette: 000000, 100000, 200000, 300000, 400000, 500000, 600000, 700000, 800000, 900000, a00000, b00000, c00000, d00000, e00000" needs 16 colors as hex RGB.`},
		{"color of 7 digits", first + "size: 720x576\ncustom colors: ON, tridx: 1000, colors: 0000000, 1, 2, 3\n", `The .idx line "custom colors: ON, tridx: 1000, colors: 0000000, 1, 2, 3" needs 4 colors as hex RGB.`},
		{"custom colors with a bad tridx", first + "size: 720x576\ncustom colors: ON, tridx: 100, colors: 0, 1, 2, 3\n", `The .idx custom colors line "custom colors: ON, tridx: 100, colors: 0, 1, 2, 3" is not valid.`},
		{"index above 31", first + "size: 720x576\n" + palette + "id: en, index: 32\n", `The .idx id line "id: en, index: 32" is not valid.`},
		{"id without index", first + "size: 720x576\n" + palette + "id: en\n", `The .idx id line "id: en" is not valid.`},
		{"timestamp before any id", first + "size: 720x576\n" + palette + "timestamp: 00:00:01:000, filepos: 0\n", `The .idx timestamp line "timestamp: 00:00:01:000, filepos: 0" comes before any id line.`},
		{"timestamp without filepos", first + "size: 720x576\n" + palette + "id: en, index: 0\ntimestamp: 00:00:01:000\n", `The .idx timestamp line "timestamp: 00:00:01:000" is not valid.`},
		{"time with 1 digit of minutes and 4 of milliseconds", first + "size: 720x576\n" + palette + "id: en, index: 0\ntimestamp: 00:00:01:0000, filepos: 0\n", `The .idx time "timestamp: 00:00:01:0000, filepos: 0" is not valid.`},
		{"time of 100000 hours", first + "size: 720x576\n" + palette + "id: en, index: 0\ntimestamp: 100000:00:00:000, filepos: 0\n", `The time "timestamp: 100000:00:00:000, filepos: 0" is not below 100000 hours.`},
		{"time that only the minutes push over", first + "size: 720x576\n" + palette + "id: en, index: 0\ntimestamp: 99999:99:99:999, filepos: 0\n", `The time "timestamp: 99999:99:99:999, filepos: 0" is not below 100000 hours.`},
		{"sum with the delay over the limit", first + "size: 720x576\n" + palette + "id: en, index: 0\ndelay: 00:00:10:500\ntimestamp: 99999:59:55:000, filepos: 0\n", `The time "timestamp: 99999:59:55:000, filepos: 0 with delay 10.5s" is not below 100000 hours.`},
		{"sum with the delay below the negative limit", first + "size: 720x576\n" + palette + "id: en, index: 0\ndelay: -99999:00:00:000\ntimestamp: -99999:00:00:000, filepos: 0\n", `The time "timestamp: -99999:00:00:000, filepos: 0 with delay -359996400s" is not below 100000 hours.`},
		{"bad delay", first + "size: 720x576\n" + palette + "id: en, index: 0\ndelay: soon\n", `The .idx time "delay: soon" is not valid.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := dvdsub.ParseIndex([]byte(tt.idx))
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, model.ParseError{Format: model.VobSub, Msg: tt.want}, *perr)
		})
	}
}

func TestParseIndex(t *testing.T) {
	t.Parallel()

	idx := "\ufeff# VobSub index file, v7\r\n\r\n# a comment\r\nSIZE : 720 x 576\r\norg: 0, 0\r\n" + palette +
		"Custom Colors: 1, tridx: 1010, colors: ff0000, 00ff00, 0000ff, ffffff\r\n" +
		"id: en , index: 1\r\n" +
		"delay: 00:00:00:500\r\ndelay: -00:00:00.100\r\n" +
		"timestamp: 00:00:05:5, filepos: 00000001F\r\n" +
		"timestamp: 00:00:01:000, filepos: 000000000\r\n" +
		"timestamp: 00:00:01,000, filepos: 000000ABC\r\n" +
		"id: , index: 0\r\n" +
		"timestamp: +01:02:03.004, filepos: 0\r\n"
	got, err := dvdsub.ParseIndex([]byte(idx))
	require.NoError(t, err)

	assert.Equal(t, [2]int{720, 576}, [2]int{got.Width, got.Height})
	assert.Equal(t, uint32(0x300000), got.Palette[3])
	assert.Equal(t, &[4]uint32{0xFF000000, 0x00FF00FF, 0x0000FF00, 0xFFFFFFFF}, got.Custom)
	assert.Equal(t, []dvdsub.Track{
		{ID: "en", Index: 1, Entries: []dvdsub.Entry{
			{Time: 1400 * time.Millisecond, FilePos: 0, Line: 1},
			{Time: 1400 * time.Millisecond, FilePos: 0xABC, Line: 2},
			{Time: 5405 * time.Millisecond, FilePos: 0x1F, Line: 0},
		}},
		{ID: "", Index: 0, Entries: []dvdsub.Entry{{Time: time.Hour + 2*time.Minute + 3*time.Second + 4*time.Millisecond, FilePos: 0, Line: 0}}},
	}, got.Tracks)
}

func TestParseIndexKeepsASumOfDelaysThatPassesTheLimit(t *testing.T) {
	t.Parallel()

	idx := first + "size: 720x576\n" + palette + "id: en, index: 0\ndelay: -99999:00:00:000\ndelay: -99999:00:00:000\n" +
		"delay: 99999:00:00:000\ntimestamp: 00:00:01:000, filepos: 0\n"
	got, err := dvdsub.ParseIndex([]byte(idx))
	require.NoError(t, err)
	assert.Equal(t, -99999*time.Hour+time.Second, got.Tracks[0].Entries[0].Time)
}

func FuzzParseIndex(f *testing.F) {
	for _, name := range []string{"two-tracks-pal", "custom-colors-ntsc", "text-pal", "one-packet-pal"} {
		f.Add(testfiles.Read(f, "vobsub/"+name+".idx"))
	}
	f.Add([]byte(first + "size: 1x1\n" + palette + "id: a, index: 0\ndelay: 99999:00:00.000\ndelay: -99999:00:00.000\ntimestamp: 0:0:0.0, filepos: ffffffffffffffffff\n"))

	f.Fuzz(func(t *testing.T, idx []byte) {
		got, err := dvdsub.ParseIndex(idx)
		if err != nil {
			var perr *model.ParseError
			if !errors.As(err, &perr) || perr.Format != model.VobSub {
				t.Fatalf("ParseIndex returned %v, which is no *ParseError of vobsub", err)
			}
			return
		}
		if got.Width < 1 || got.Height < 1 {
			t.Fatalf("the screen size is %dx%d", got.Width, got.Height)
		}
		for _, track := range got.Tracks {
			if track.Index < 0 || track.Index > dvdsub.MaxTrackIndex {
				t.Fatalf("the track index is %d", track.Index)
			}
			var previous time.Duration
			for _, e := range track.Entries {
				if e.Time < previous || e.Time%time.Millisecond != 0 || e.FilePos < 0 {
					t.Fatalf("the entry %+v follows %v", e, previous)
				}
				previous = e.Time
			}
		}
	})
}
