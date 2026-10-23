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

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
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
				explicit, _, err := stbox.Parse(bytes.NewReader(tt.content), stbox.ReadOptions{Format: tt.want})
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
		s, info, err := stbox.Parse(strings.NewReader(content), stbox.ReadOptions{})
		require.ErrorIs(t, err, stbox.ErrUnknownFormat, "%q", content)
		require.EqualError(t, err, unknownFormatMessage)
		assert.Nil(t, s)
		assert.Zero(t, info)
	}
}

func TestReadContainer(t *testing.T) {
	t.Parallel()

	ts := make([]byte, 377)
	ts[0], ts[188], ts[376] = 0x47, 0x47, 0x47
	m2ts := make([]byte, 389)
	m2ts[4], m2ts[196], m2ts[388] = 0x47, 0x47, 0x47
	tests := []struct {
		name   string
		file   string
		head   []byte
		format stbox.Format
		label  string
	}{
		{"Matroska", "movie.mkv", []byte("\x1A\x45\xDF\xA3\x01\x00\x00\x00"), "", "a Matroska"},
		{"WebM with a format", "movie.webm", []byte("\x1A\x45\xDF\xA3\x01\x00\x00\x00"), stbox.SubRip, "a Matroska"},
		{"MP4", "movie.mp4", []byte("\x00\x00\x00\x20ftypisom\x00\x00\x02\x00"), "", "an MP4"},
		{"MPEG-TS", "movie.ts", ts, "", "an MPEG-TS"},
		{"MPEG-TS with a .srt extension", "movie.srt", ts, "", "an MPEG-TS"},
		{"M2TS", "movie.m2ts", m2ts, "", "an MPEG-TS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := writeTemp(t, tt.file, tt.head)
			opts := stbox.ReadOptions{Format: tt.format}
			s, info, err := stbox.Load(p, opts)
			require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
			require.NotErrorIs(t, err, stbox.ErrUnknownFormat)
			require.EqualError(t, err, "stbox: unsupported format: "+p+" is "+tt.label+" file. This version reads no video files.")
			assert.Nil(t, s)
			assert.Zero(t, info)

			_, _, err = stbox.Parse(bytes.NewReader(tt.head), opts)
			require.EqualError(t, err, "stbox: unsupported format: The content is "+tt.label+" file. This version reads no video files.")
		})
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

	s, info, err := stbox.Load(testfiles.Path(t, "cli/frames.sub"), stbox.ReadOptions{MicroDVD: microdvd.ReadOptions{FrameRate: 25}})
	require.NoError(t, err)
	assert.Equal(t, stbox.MicroDVD, info.Format)
	first, _ := s.Cue(0)
	assert.Equal(t, []time.Duration{time.Second, 3 * time.Second}, []time.Duration{first.Start, first.End})

	var stored, explicit, other bytes.Buffer
	require.NoError(t, stbox.Write(&stored, s, stbox.MicroDVD, stbox.WriteOptions{}))
	require.NoError(t, stbox.Write(&explicit, s, stbox.MicroDVD, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: 25}}))
	assert.Equal(t, explicit.String(), stored.String())
	require.NoError(t, stbox.Write(&other, s, stbox.MicroDVD, stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: 50}}))
	assert.True(t, strings.HasPrefix(other.String(), "{50}{150}"), other.String())

	trip, _, err := stbox.Load(testfiles.Path(t, "cli/trip.srt"), stbox.ReadOptions{})
	require.NoError(t, err)
	err = stbox.Write(&bytes.Buffer{}, trip, stbox.MicroDVD, stbox.WriteOptions{})
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
