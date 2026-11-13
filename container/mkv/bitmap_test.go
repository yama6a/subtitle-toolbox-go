package mkv_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/pgs"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const segmentEnd = 0x80

// supDisplaySet returns the segments of the first display set of a .sup file without their PG headers.
func supDisplaySet(tb testing.TB, name string) []byte {
	tb.Helper()
	sup := testfiles.Read(tb, "pgs/"+name)
	var set []byte
	for pos := 0; pos < len(sup); {
		segmentType := sup[pos+10]
		size := int(binary.BigEndian.Uint16(sup[pos+11:]))
		set = append(set, sup[pos+10:pos+13+size]...)
		pos += 13 + size
		if segmentType == segmentEnd {
			break
		}
	}
	return set
}

func TestExtractPGS(t *testing.T) {
	t.Parallel()

	sup, _, err := pgs.Read(bytes.NewReader(testfiles.Read(t, "pgs/shapes_1080p.sup")), model.CommonReadOptions{}, pgs.ReadOptions{})
	require.NoError(t, err)
	f := openTestdata(t, "pgs.mkv")
	require.Equal(t, []model.Track{
		{Container: model.Matroska, Number: 3, CodecID: "S_HDMV/PGS", Format: model.PGS, Language: "ger", Default: true},
		{Container: model.Matroska, Number: 4, CodecID: "S_HDMV/PGS", Format: model.PGS, Language: "eng", Default: true, Forced: true},
	}, f.Tracks())

	for _, tt := range []struct {
		name   string
		track  int
		lang   string
		forced bool
	}{
		{"one segment per block", 3, "ger", false},
		{"one display set per block with the forced flag", 4, "eng", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := f.Extract(tt.track, model.CommonReadOptions{})
			require.NoError(t, err)
			assert.Empty(t, warnings)
			lang, _ := s.Metadata(model.MetadataLanguage)
			assert.Equal(t, tt.lang, lang)
			want := sup.Cues()
			require.NotEmpty(t, want)
			got := s.Cues()
			require.Len(t, got, len(want))
			for i, c := range got {
				if tt.forced {
					assert.True(t, c.Forced, "forced of cue %d", i)
					c.Forced = want[i].Forced
				}
				assert.Equal(t, want[i], c, "cue %d", i)
			}
		})
	}
}

func TestExtractPGSBlocks(t *testing.T) {
	t.Parallel()

	set := supDisplaySet(t, "shapes_1080p.sup")
	pgsFile := func(clusterMs uint64, blocks ...[]byte) []byte {
		return segmentFile("matroska", info(1000000, "x"), tracks(subTrack(2, "S_HDMV/PGS")), cluster(clusterMs, blocks...))
	}

	t.Run("a block at 14 hours keeps its time", func(t *testing.T) {
		t.Parallel()

		s, _, err := openFile(t, pgsFile(14*3600*1000, simpleBlock(2, 0, string(set), 0))).Extract(2, model.CommonReadOptions{})
		require.NoError(t, err)
		require.Len(t, s.Cues(), 1)
		assert.Equal(t, 14*time.Hour, s.Cues()[0].Start)
	})

	cut := pgsFile(0, simpleBlock(2, 0, string(set), 0), simpleBlock(2, 1000, "\x14\x00\x10ab", 0))
	t.Run("a cut segment fails a strict read", func(t *testing.T) {
		t.Parallel()

		_, _, err := openFile(t, cut).Extract(2, model.CommonReadOptions{})
		var pe *model.ParseError
		require.ErrorAs(t, err, &pe)
		assert.Regexp(t, `^The block of track 2 at byte \d+ holds a PGS segment that is cut off\.$`, pe.Msg)
	})
	t.Run("a cut segment ends a lenient read with a warning", func(t *testing.T) {
		t.Parallel()

		s, warnings, err := openFile(t, cut).Extract(2, model.CommonReadOptions{Lenient: true})
		require.NoError(t, err)
		assert.Len(t, s.Cues(), 1)
		require.Len(t, warnings, 1)
		assert.Equal(t, model.Skipped, warnings[0].Action)
	})
}

func TestExtractVobSub(t *testing.T) {
	t.Parallel()

	idx := string(testfiles.Read(t, "vobsub/two-tracks-pal.idx"))
	ref, _, err := vobsub.Read(bytes.NewReader(testfiles.Read(t, "vobsub/two-tracks-pal.sub")), model.CommonReadOptions{},
		vobsub.ReadOptions{IDX: idx, Language: "en"})
	require.NoError(t, err)
	f := openTestdata(t, "vobsub.mkv")
	require.Len(t, ref.Cues(), 5)

	s3, _, err := f.Extract(3, model.CommonReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, ref.Cues(), s3.Cues())
	lang, _ := s3.Metadata(model.MetadataLanguage)
	assert.Equal(t, "eng", lang)

	// Track 4 has the units of track 3 in blocks of 1500 ms. A unit without a stop command ends with its block.
	s4, _, err := f.Extract(4, model.CommonReadOptions{})
	require.NoError(t, err)
	want := ref.Cues()
	want[2].End = 10500 * time.Millisecond
	want[4].End = 21500 * time.Millisecond
	assert.Equal(t, want, s4.Cues())
}

func TestExtractVobSubErrors(t *testing.T) {
	t.Parallel()

	const header = "size: 720x576\npalette: 000000, 100000, 200000, 300000, 400000, 500000, 600000, 700000, 800000, 900000, a00000, b00000, c00000, d00000, e00000, f00000\n"
	vobFile := func(private string, blocks ...[]byte) []byte {
		return segmentFile("matroska", info(1000000, "x"),
			tracks(trackEntry(trackFields{number: 2, kind: trackSub, codecID: "S_VOBSUB", codecPrivate: []byte(private)})), cluster(0, blocks...))
	}

	t.Run("a header without a palette", func(t *testing.T) {
		t.Parallel()

		_, _, err := openFile(t, vobFile("size: 720x576\n")).Extract(2, model.CommonReadOptions{})
		var pe *model.ParseError
		require.ErrorAs(t, err, &pe)
		assert.Equal(t, model.ParseError{Format: model.VobSub, Msg: "The .idx content has no palette line."}, *pe)
	})

	broken := vobFile(header, simpleBlock(2, 0, "\x00\x01", 0))
	t.Run("a broken unit fails a strict read", func(t *testing.T) {
		t.Parallel()

		_, _, err := openFile(t, broken).Extract(2, model.CommonReadOptions{})
		var pe *model.ParseError
		require.ErrorAs(t, err, &pe)
		assert.Equal(t, model.VobSub, pe.Format)
	})
	t.Run("a broken unit is skipped in a lenient read", func(t *testing.T) {
		t.Parallel()

		s, warnings, err := openFile(t, broken).Extract(2, model.CommonReadOptions{Lenient: true})
		require.NoError(t, err)
		assert.Empty(t, s.Cues())
		require.Len(t, warnings, 1)
		assert.Equal(t, model.Skipped, warnings[0].Action)
		assert.Equal(t, 0, warnings[0].Block)
	})
}
