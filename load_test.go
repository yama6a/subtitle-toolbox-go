package stbox_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

const unknownFormatMessage = "stbox: unsupported format: no format detected: Format detection found no subtitle format."

func writeTemp(t *testing.T, name string, content []byte) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, content, 0o600))
	return p
}

// Ports testLoadWithoutFormatReadsTheFormatOfTheContentOrTheExtension of tests/LoadSaveTest.php and
// testDetectFormatAppliesTheIttAndExtensionRulesOfLoadWithoutFormat of tests/FormatDetectorTest.php of
// subtitle-toolbox-php.
func TestLoadWithoutFormat(t *testing.T) {
	t.Parallel()

	srt := testfiles.Read(t, "srt/real/own_styled.srt")
	vtt := testfiles.Read(t, "vtt/real/w3c_identifiers.vtt")
	itt := testfiles.Read(t, "itt/real/fcp_23976_styles.itt")
	tests := []struct {
		name    string
		file    string
		content []byte
		want    stbox.Format
		wantErr error
	}{
		{name: "SubRip by content", file: "trip.srt", content: srt, want: stbox.SubRip},
		{name: "WebVTT by content", file: "shop.vtt", content: vtt, want: stbox.WebVTT},
		{name: "content wins over the extension", file: "shop.srt", content: vtt, want: stbox.WebVTT},
		{name: "SubRip in a .json file", file: "trip.json", content: srt, want: stbox.SubRip},
		{name: "library JSON in a .txt file", file: "cues.txt", content: []byte(`{"version": 1, "cues": []}`), want: stbox.JSON},
		{name: "SubRip in a file without extension", file: "trip", content: srt, want: stbox.SubRip},
		{name: "extension fallback", file: "film.srt", content: []byte("plain words"), want: stbox.SubRip, wantErr: stbox.ErrParse},
		{name: "extension fallback in upper case", file: "film.VTT", content: []byte("plain words"), want: stbox.WebVTT, wantErr: stbox.ErrParse},
		{name: "no fallback for .json", file: "film.json", content: []byte("plain words"), wantErr: stbox.ErrUnknownFormat},
		{name: "JSON of no format in a .json file", file: "speech.json", content: []byte(`{"segments": []}`), wantErr: stbox.ErrUnknownFormat},
		{name: "no fallback for .txt", file: "film.txt", content: []byte("plain words"), wantErr: stbox.ErrUnknownFormat},
		{name: "no fallback for .ffmeta", file: "film.ffmeta", content: []byte("plain words"), wantErr: stbox.ErrUnknownFormat},
		{name: "extension fallback for ASS", file: "film.ass", content: []byte("plain words"), want: stbox.ASS, wantErr: stbox.ErrParse},
		{name: "extension fallback for MicroDVD", file: "film.sub", content: []byte("plain words"), want: stbox.MicroDVD, wantErr: stbox.ErrParse},
		{name: "extension fallback for SAMI", file: "film.smi", content: []byte("plain words"), want: stbox.SAMI, wantErr: stbox.ErrParse},
		{name: "extension fallback for CSV", file: "film.csv", content: []byte("plain words"), want: stbox.CSV, wantErr: stbox.ErrParse},
		{name: "extension fallback for TSV", file: "film.tsv", content: []byte("plain words"), want: stbox.TSV, wantErr: stbox.ErrParse},
		{name: "CSV by extension", file: "film.csv", content: []byte("start,end,text\n1,2,Hello\n"), want: stbox.CSV},
		{name: "TTML content in a .itt file is iTT", file: "film.itt", content: itt, want: stbox.ITT},
		{name: "TTML content in a .ttml file", file: "film.ttml", content: itt, want: stbox.TTML},
		{name: "blank .itt file", file: "film.itt", content: []byte("\n"), want: stbox.ITT},
		{name: "unknown extension", file: "film.xyz", content: []byte("plain words"), wantErr: stbox.ErrUnknownFormat},
		{name: "no extension", file: "film", content: []byte("plain words"), wantErr: stbox.ErrUnknownFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(writeTemp(t, tt.file, tt.content), stbox.ReadOptions{})
			switch {
			case tt.wantErr == nil:
				require.NoError(t, err)
				assert.Equal(t, tt.want, info.Format)
				explicit, _, err := parseBytes(tt.content, stbox.ReadOptions{Format: tt.want})
				require.NoError(t, err)
				assert.Equal(t, explicit, s)
			case errors.Is(tt.wantErr, stbox.ErrParse):
				var perr *stbox.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.want, perr.Format)
			default:
				require.ErrorIs(t, err, tt.wantErr)
				require.EqualError(t, err, unknownFormatMessage)
				assert.Nil(t, s)
				assert.Zero(t, info)
			}
		})
	}
}

func TestParseReadsITTContentAsTTML(t *testing.T) {
	t.Parallel()

	_, info, err := parseBytes(testfiles.Read(t, "itt/real/fcp_23976_styles.itt"), stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.TTML, info.Format)
}

// Ports testLoadWithoutFormatReadsAnEmptyFileInTheFormatOfItsExtension of tests/LoadSaveTest.php and the empty file
// tests of tests/SubtitleReaderTest.php of subtitle-toolbox-php.
func TestLoadEmptyFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		file         string
		content      string
		want         stbox.Format
		wantEncoding string
	}{
		{"0 bytes, .vtt", "empty.vtt", "", stbox.WebVTT, "UTF-8"},
		{"BOM and whitespace, .srt", "blank.srt", "\xEF\xBB\xBF \r\n\t\n", stbox.SubRip, "UTF-8"},
		{"space and LF, .srt", "empty.srt", " \n", stbox.SubRip, "UTF-8"},
		{"2 BOMs, NUL and VT, .vtt", "blank.vtt", "\xEF\xBB\xBF\xEF\xBB\xBF\x00\v\n", stbox.WebVTT, "UTF-8"},
		{"UTF-16 BOM and spaces, .srt", "blank.srt", "\xFF\xFE \x00 \x00", stbox.SubRip, "UTF-16LE"},
		{"upper-case extension", "EMPTY.SRT", "", stbox.SubRip, "UTF-8"},
		{"UTF-16 BOM and spaces, .ass", "blank.ass", "\xFF\xFE \x00 \x00", stbox.ASS, "UTF-16LE"},
		{"UTF-16 BOM and spaces, .smi", "blank.smi", "\xFF\xFE \x00 \x00", stbox.SAMI, "UTF-16LE"},
		{"UTF-16 BOM and spaces, .txt without a reader", "blank.txt", "\xFF\xFE \x00 \x00", "", ""},
		{"0 bytes, unknown extension", "empty.xyz", "", "", ""},
		{"0 bytes, format without a reader", "empty.txt", "", "", ""},
		{"0 bytes, shared extension", "empty.json", "", "", ""},
		{"0 bytes, no extension", "empty", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(writeTemp(t, tt.file, []byte(tt.content)), stbox.ReadOptions{})
			if tt.want == "" {
				require.ErrorIs(t, err, stbox.ErrUnknownFormat)
				require.EqualError(t, err, unknownFormatMessage)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, stbox.ReadInfo{Format: tt.want, SourceEncoding: tt.wantEncoding}, info)
			assert.Zero(t, s.Len())
		})
	}
}

// Ports testAnEmptyFileWithAFormatGivesNoCuesAndEmptyContentWithoutAPathThrows of tests/LoadSaveTest.php of
// subtitle-toolbox-php.
func TestReadEmptyContent(t *testing.T) {
	t.Parallel()

	s, info, err := stbox.Load(writeTemp(t, "empty.srt", nil), stbox.ReadOptions{Format: stbox.SubRip})
	require.NoError(t, err)
	assert.Equal(t, stbox.SubRip, info.Format)
	assert.Zero(t, s.Len())

	for _, content := range []string{"", " \n", "\xEF\xBB\xBF"} {
		s, info, err := parseBytes([]byte(content), stbox.ReadOptions{})
		require.ErrorIs(t, err, stbox.ErrUnknownFormat, "%q", content)
		require.EqualError(t, err, unknownFormatMessage)
		assert.Nil(t, s)
		assert.Zero(t, info)
	}
}

func TestReadNoContainer(t *testing.T) {
	t.Parallel()

	ts := "G\n1\n00:00:01,000 --> 00:00:02,000\nHello\n"
	ts += strings.Repeat("x", 188-len(ts)) + "G" + strings.Repeat("x", 187)
	m2ts := "abcdG\n1\n00:00:01,000 --> 00:00:02,000\nHello\n"
	m2ts += strings.Repeat("x", 196-len(m2ts)) + "G" + strings.Repeat("x", 191)
	tests := []struct {
		name    string
		content string
	}{
		{"SubRip in a .mkv file", "1\n00:00:01,000 --> 00:00:02,000\nHello\n"},
		{"ftyp at byte 0", "ftyp\n1\n00:00:01,000 --> 00:00:02,000\nHello\n"},
		{"G at bytes 0 and 188 of 376", ts},
		{"G at bytes 4 and 196 of 388", m2ts},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, info, err := stbox.Load(writeTemp(t, "movie.mkv", []byte(tt.content)), stbox.ReadOptions{Lenient: true})
			require.NoError(t, err)
			assert.Equal(t, stbox.SubRip, info.Format)
		})
	}
}

// Ports testMicroDvdWithTheFrameRateOfTheOptions and testSaveMicroDvdTakesTheFrameRateFromTheOptionsThenFromTheFormatData
// of tests/LoadSaveTest.php of subtitle-toolbox-php.
func TestLoadAndWriteMicroDVD(t *testing.T) {
	t.Parallel()

	s, info, err := stbox.Load(testfiles.Path(t, "cli/frames.sub"), stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}})
	require.NoError(t, err)
	assert.Equal(t, stbox.MicroDVD, info.Format)
	first, _ := s.Cue(0)
	assert.Equal(t, []time.Duration{time.Second, 3 * time.Second}, []time.Duration{first.Start, first.End})

	var stored, explicit, other bytes.Buffer
	_, err = stbox.Write(&stored, s, stbox.MicroDVD, stbox.WriteOptions{})
	require.NoError(t, err)
	_, err = stbox.Write(&explicit, s, stbox.MicroDVD, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}})
	require.NoError(t, err)
	assert.Equal(t, explicit.String(), stored.String())
	_, err = stbox.Write(&other, s, stbox.MicroDVD, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 50, Den: 1}}})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(other.String(), "{50}{150}"), other.String())

	trip, _, err := stbox.Load(testfiles.Path(t, "cli/trip.srt"), stbox.ReadOptions{})
	require.NoError(t, err)
	_, err = stbox.Write(&bytes.Buffer{}, trip, stbox.MicroDVD, stbox.WriteOptions{})
	var argErr *stbox.InvalidArgumentError
	require.ErrorAs(t, err, &argErr)
	assert.Equal(t, "WriteOptions.MicroDVD.FrameRate", argErr.Option)
}

// Ports the MicroDVD case of testAutoDetectionGivesTheSameCuesAsTheDetectedFormat of tests/FormatDetectorTest.php of
// subtitle-toolbox-php: a file without a {1}{1}<fps> line needs the frame rate with and without a format.
func TestMicroDVDWithoutFrameRateLine(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"cli/frames.sub", "microdvd/real/sub_microdvd.sub", "microdvd/real/subsrt_sample.sub"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			for _, f := range []stbox.Format{"", stbox.MicroDVD} {
				_, _, err := stbox.Load(testfiles.Path(t, file), stbox.ReadOptions{Format: f})
				var perr *stbox.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, stbox.MicroDVD, perr.Format)
				assert.Equal(t, "ReadOptions.MicroDVD.FrameRate", perr.Option)
				assert.True(t, strings.HasPrefix(perr.Msg, "The frame rate is unknown."), perr.Msg)
			}
		})
	}
}

// TestBinaryFormatRead covers the read path of a binary format: the raw bytes reach the reader, Encoding is
// ignored and SourceEncoding stays empty.
func TestBinaryFormatRead(t *testing.T) {
	t.Parallel()

	stl := testfiles.Read(t, "stl/real/bakery_teletext_25fps.stl")
	spaced := append([]byte("   "), stl[3:]...)
	want, _, err := parseBytes(stl, stbox.ReadOptions{Format: stbox.EBUSTL})
	require.NoError(t, err)
	require.Positive(t, want.Len())

	tests := []struct {
		name    string
		file    string // empty reads with Parse
		content []byte
		opts    stbox.ReadOptions
		cues    int // -1 reads SubRip
		wantErr error
	}{
		{name: "Parse with a format and an encoding", content: stl, opts: stbox.ReadOptions{Format: stbox.EBUSTL, Encoding: "ISO-8859-7"}, cues: want.Len()},
		{name: "Parse by content", content: stl, cues: want.Len()},
		{name: "Load by content", file: "show.bin", content: stl, cues: want.Len()},
		{name: "Load by content with an encoding", file: "show.stl", content: stl, opts: stbox.ReadOptions{Encoding: "ISO-8859-7"}, cues: want.Len()},
		{name: "Load of an empty .stl file", file: "show.stl", content: nil},
		{name: "Load by extension when the code page field is spaces", file: "show.stl", content: spaced, cues: want.Len()},
		{name: "Load of SubRip content named .stl", file: "show.stl", content: testfiles.Read(t, "srt/real/own_styled.srt"), cues: -1},
		{name: "Parse when the code page field is spaces", content: spaced, wantErr: stbox.ErrUnknownFormat},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var s *stbox.Subtitle
			var info stbox.ReadInfo
			var err error
			if tt.file == "" {
				s, info, err = parseBytes(tt.content, tt.opts)
			} else {
				s, info, err = stbox.Load(writeTemp(t, tt.file, tt.content), tt.opts)
			}
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if tt.cues < 0 {
				assert.Equal(t, stbox.SubRip, info.Format)
				assert.Positive(t, s.Len())
				return
			}
			assert.Equal(t, stbox.EBUSTL, info.Format)
			assert.Empty(t, info.SourceEncoding)
			assert.Empty(t, info.Warnings)
			assert.Equal(t, tt.cues, s.Len())
		})
	}
}
