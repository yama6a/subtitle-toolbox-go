package stbox_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/csv"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/pac"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/v5/format/vobsub"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func TestUnsupportedFormatMessages(t *testing.T) {
	t.Parallel()

	type messages struct {
		format    stbox.Format
		wantRead  string
		wantWrite string
	}
	tests := make([]messages, 0, 2+len(stbox.Formats()))
	tests = append(tests,
		messages{"foo", `The format "foo" is unknown.`, `The format "foo" is unknown.`},
		messages{"", `The format "" is unknown.`, `The format "" is unknown.`},
	)
	for _, f := range stbox.Formats() {
		tt := messages{format: f}
		switch {
		case f.CanRead():
		case f.CanWrite():
			tt.wantRead = "The format " + string(f) + " can be written but not read."
		}
		switch {
		case f.CanWrite():
		case f.CanRead():
			tt.wantWrite = "The format " + string(f) + " can be read but not written."
		}
		tests = append(tests, tt)
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			t.Parallel()

			if tt.wantRead != "" && tt.format != "" {
				_, _, err := parseBytes([]byte("1\nHello\n"), stbox.ReadOptions{Format: tt.format})
				require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
				require.EqualError(t, err, "stbox: unsupported format: "+tt.wantRead)
			}
			if tt.wantWrite != "" {
				_, err := stbox.Write(&bytes.Buffer{}, &stbox.Subtitle{}, tt.format, stbox.WriteOptions{})
				require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
				require.EqualError(t, err, "stbox: unsupported format: "+tt.wantWrite)
			}
		})
	}
}

func TestLoadUnreadableFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tests := []struct {
		name string
		path string
	}{
		{"missing file", filepath.Join(dir, "missing.srt")},
		{"folder", dir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(tt.path, stbox.ReadOptions{})
			var pathErr *fs.PathError
			require.ErrorAs(t, err, &pathErr)
			assert.Equal(t, tt.path, pathErr.Path)
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestLoadUnsupportedFormat(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(path, []byte("1\nHello\n"), 0o600))

	s, info, err := stbox.Load(path, stbox.ReadOptions{Format: stbox.PlainText})
	require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
	assert.Nil(t, s)
	assert.Zero(t, info)
}

func TestParseUnsupportedFormat(t *testing.T) {
	t.Parallel()

	formats := []stbox.Format{"foo"}
	for _, f := range stbox.Formats() {
		if !f.CanRead() {
			formats = append(formats, f)
		}
	}
	for _, f := range formats {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			s, info, err := parseBytes([]byte("1\nHello\n"), stbox.ReadOptions{Format: f})
			require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
			require.NotErrorIs(t, err, stbox.ErrUnknownFormat)
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestParseNoFormatFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"cue number and text only", "1\nHello\n"},
		{"JSON object of no format", `{"words": []}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := parseBytes([]byte(tt.content), stbox.ReadOptions{})
			require.ErrorIs(t, err, stbox.ErrUnknownFormat)
			require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestLoadMissingFile(t *testing.T) {
	t.Parallel()

	s, info, err := stbox.Load(filepath.Join(t.TempDir(), "missing.srt"), stbox.ReadOptions{})
	require.ErrorIs(t, err, fs.ErrNotExist)
	assert.Nil(t, s)
	assert.Zero(t, info)
}

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
		{name: "JSON of no format in a .json file", file: "speech.json", content: []byte(`{"words": []}`), wantErr: stbox.ErrUnknownFormat},
		{name: "no fallback for .txt", file: "film.txt", content: []byte("plain words"), wantErr: stbox.ErrUnknownFormat},
		{name: "no fallback for .ffmeta", file: "film.ffmeta", content: []byte("plain words"), wantErr: stbox.ErrUnknownFormat},
		{name: "extension fallback for ASS", file: "film.ass", content: []byte("plain words"), want: stbox.ASS, wantErr: stbox.ErrParse},
		{name: "extension fallback for MicroDVD", file: "film.sub", content: []byte("plain words"), want: stbox.MicroDVD, wantErr: stbox.ErrParse},
		{name: "extension fallback for SAMI", file: "film.smi", content: []byte("plain words"), want: stbox.SAMI, wantErr: stbox.ErrParse},
		{name: "extension fallback for CSV", file: "film.csv", content: []byte("plain words"), want: stbox.CSV, wantErr: stbox.ErrParse},
		{name: "extension fallback for TSV", file: "film.tsv", content: []byte("plain words"), want: stbox.TSV, wantErr: stbox.ErrParse},
		{name: "extension fallback for srv3", file: "film.srv3", content: []byte("plain words"), want: stbox.YouTubeTimedText, wantErr: stbox.ErrParse},
		{name: "extension fallback for json3", file: "film.json3", content: []byte("plain words"), want: stbox.YouTubeTimedText, wantErr: stbox.ErrParse},
		{name: "extension fallback for srv1", file: "film.srv1", content: []byte("plain words"), want: stbox.YouTubeTimedText, wantErr: stbox.ErrParse},
		{name: "extension fallback for ytt", file: "film.ytt", content: []byte("plain words"), want: stbox.YouTubeTimedText, wantErr: stbox.ErrParse},
		{name: "json3 by content in a .json file", file: "captions.json", content: []byte(`{"events": [{"tStartMs": 0, "segs": [{"utf8": "Hi"}]}]}`), want: stbox.YouTubeTimedText},

		{name: "HTML transcript without speakers by .html", file: "talk.html", content: []byte("<time>0:01</time>\n<p>Hello</p>\n"), want: stbox.HTMLTranscript},
		{name: "HTML transcript without speakers by .htm", file: "TALK.HTM", content: []byte("<time>0:01</time>\n<p>Hello</p>\n"), want: stbox.HTMLTranscript},
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
		{"0 bytes, .srv3", "empty.srv3", "", stbox.YouTubeTimedText, "UTF-8"},
		{"0 bytes, .ytt", "empty.ytt", "", stbox.YouTubeTimedText, "UTF-8"},
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

// TestPACEncodingNamesTheCodePage covers decision D1: Encoding picks the PAC code page, and PAC.CodePage wins.
func TestPACEncodingNamesTheCodePage(t *testing.T) {
	t.Parallel()

	greek := testfiles.Read(t, "pac/real/weather_greek.pac")
	firstLine := func(t *testing.T, content []byte, opts stbox.ReadOptions) string {
		t.Helper()

		s, info, err := parseBytes(content, opts)
		require.NoError(t, err)
		assert.Equal(t, stbox.PAC, info.Format)
		assert.Empty(t, info.SourceEncoding)
		return s.Cues()[0].Lines[0]
	}
	detected := firstLine(t, greek, stbox.ReadOptions{})
	cyrillic := firstLine(t, greek, stbox.ReadOptions{PAC: pac.ReadOptions{CodePage: pac.Cyrillic}})
	require.NotEqual(t, detected, cyrillic)

	tests := []struct {
		name     string
		opts     stbox.ReadOptions
		want     string
		option   string
		errorMsg string
	}{
		{name: "Greek name", opts: stbox.ReadOptions{Format: stbox.PAC, Encoding: "ISO-8859-7"}, want: detected},
		{name: "alias without case", opts: stbox.ReadOptions{Encoding: "windows-1253"}, want: detected},
		{name: "Cyrillic name", opts: stbox.ReadOptions{Encoding: "ISO-8859-5"}, want: cyrillic},
		{name: "field wins over the encoding", opts: stbox.ReadOptions{Encoding: "ISO-8859-5", PAC: pac.ReadOptions{CodePage: pac.Greek}}, want: detected},
		{
			name: "encoding without a PAC code page", opts: stbox.ReadOptions{Encoding: "KOI8-R"},
			option: "ReadOptions.Encoding", errorMsg: `PAC files have no code page for the encoding "KOI8-R".`,
		},
		{
			name: "spelling that textenc does not know", opts: stbox.ReadOptions{Encoding: "TIS-620"},
			option: "ReadOptions.Encoding", errorMsg: `The encoding "TIS-620" is unknown.`,
		},
		{
			name: "unknown code page field", opts: stbox.ReadOptions{PAC: pac.ReadOptions{CodePage: "klingon"}},
			option: "ReadOptions.PAC.CodePage", errorMsg: `The PAC code page "klingon" is unknown.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.option == "" {
				assert.Equal(t, tt.want, firstLine(t, greek, tt.opts))
				return
			}
			_, _, err := parseBytes(greek, tt.opts)
			var ierr *stbox.InvalidArgumentError
			require.ErrorAs(t, err, &ierr)
			assert.Equal(t, tt.option, ierr.Option)
			assert.Equal(t, tt.errorMsg, ierr.Msg)
		})
	}
}

func TestPACLoadByExtensionAndContent(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"show.pac", "show.fpc", "show.bin"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(writeTemp(t, file, testfiles.Read(t, "pac/real/bakery_latin.pac")), stbox.ReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, stbox.PAC, info.Format)
			assert.Positive(t, s.Len())
		})
	}
}

func TestLoadReadsPGSByContentAndExtension(t *testing.T) {
	t.Parallel()

	path := testfiles.Path(t, "pgs/shapes_576p.sup")
	s, info, err := stbox.Load(path, stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.PGS, info.Format)
	assert.Empty(t, info.SourceEncoding)
	assert.Equal(t, 2, s.Len())

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	s, info, err = parseBytes(content, stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.PGS, info.Format)
	assert.Equal(t, 2, s.Len())
}

// Ports the TSV and CSV rows of testLoadWithoutFormatReadsTheFormatOfTheContentOrTheExtension of tests/LoadSaveTest.php
// of subtitle-toolbox-php: a table has no signature, so Load takes the format from the extension.
func TestLoadCSVAndTSVByExtension(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		file string
		want stbox.Format
	}{
		{"csv/real/sheets_export.tsv", stbox.TSV},
		{"csv/real/excel_de_semicolon.csv", stbox.CSV},
	} {
		t.Run(tt.file, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(testfiles.Path(t, tt.file), stbox.ReadOptions{})
			require.NoError(t, err)
			assert.Equal(t, tt.want, info.Format)
			explicit, _, err := parseBytes(testfiles.Read(t, tt.file), stbox.ReadOptions{Format: tt.want})
			require.NoError(t, err)
			assert.Equal(t, explicit, s)
		})
	}
}

// Ports testReturnsNullForFrameTimecodeFixtures of tests/FormatDetectorTest.php and the CSV row of
// testDetectionFindsTheFormatAfterTwoBoms of tests/Parsers/RepeatedBomTest.php of subtitle-toolbox-php.
func TestParseDoesNotDetectCSV(t *testing.T) {
	t.Parallel()

	for _, content := range [][]byte{
		testfiles.Read(t, "csv/real/headerless_frame_times.csv"),
		testfiles.Read(t, "csv/real/mantas_done_shape.csv"),
		[]byte("\ufeff\ufeffstart,end,text\n00:00:01.000,00:00:02.000,The ferry leaves.\n00:00:03.000,00:00:04.000,It is late.\n"),
	} {
		_, ok := stbox.Detect(content)
		assert.False(t, ok)
		_, _, err := parseBytes(content, stbox.ReadOptions{})
		require.ErrorIs(t, err, stbox.ErrUnknownFormat)
	}
}

// Ports testSaveSetsTheCsvDelimiterFromTheTarfindFormat of tests/LoadSaveTest.php of subtitle-toolbox-php: a TSV read
// written as CSV without CSV.Delimiter gets commas, and TSV output gets tabs.
func TestTSVWrittenAsCSV(t *testing.T) {
	t.Parallel()

	tsv, _, err := stbox.Load(testfiles.Path(t, "csv/real/sheets_export.tsv"), stbox.ReadOptions{})
	require.NoError(t, err)
	asCSV := writeAs(t, tsv, stbox.CSV)
	first, _, _ := strings.Cut(string(asCSV), "\n")
	assert.Contains(t, first, ",")
	assert.NotContains(t, string(asCSV), "\t")
	var comma bytes.Buffer
	_, err = stbox.Write(&comma, tsv, stbox.CSV, stbox.WriteOptions{CSV: csv.WriteOptions{Delimiter: csv.Comma}})
	require.NoError(t, err)
	assert.Equal(t, comma.String(), string(asCSV))
	first, _, _ = strings.Cut(string(writeAs(t, tsv, stbox.TSV)), "\n")
	assert.Contains(t, first, "\t")

	semicolons, _, err := stbox.Load(testfiles.Path(t, "csv/real/excel_de_semicolon.csv"), stbox.ReadOptions{})
	require.NoError(t, err)
	first, _, _ = strings.Cut(string(writeAs(t, semicolons, stbox.CSV)), "\n")
	assert.Contains(t, first, ";")
	first, _, _ = strings.Cut(string(writeAs(t, semicolons, stbox.TSV)), "\n")
	assert.Contains(t, first, "\t")

	var semicolon bytes.Buffer
	_, err = stbox.Write(&semicolon, tsv, stbox.CSV, stbox.WriteOptions{CSV: csv.WriteOptions{Delimiter: csv.Semicolon}})
	require.NoError(t, err)
	first, _, _ = strings.Cut(semicolon.String(), "\n")
	assert.Contains(t, first, ";")
}

// The .tsv cases of testRealFileFormatsToItsOwnBytes of tests/Parsers/CsvRealFilesTest.php of subtitle-toolbox-php,
// written as TSV.
func TestTSVRealFilesWriteTheirOwnBytes(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"csv/real/sheets_export.tsv", "csv/real/own_openai_whisper.tsv"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s, _, err := stbox.Load(testfiles.Path(t, file), stbox.ReadOptions{})
			require.NoError(t, err)
			var out bytes.Buffer
			_, err = stbox.Write(&out, s, stbox.TSV, stbox.WriteOptions{BOM: stbox.BOMRemove})
			require.NoError(t, err)
			assert.Equal(t, string(testfiles.Read(t, file)), out.String())
		})
	}
}

func TestTSVReadErrorsNameTSV(t *testing.T) {
	t.Parallel()

	_, _, err := parseBytes([]byte("start\tend\n1\t2\n"), stbox.ReadOptions{Format: stbox.TSV})
	var perr *stbox.ParseError
	require.ErrorAs(t, err, &perr)
	assert.Equal(t, &stbox.ParseError{Format: stbox.TSV, Line: 1, Msg: `The table has no column "text" for text.`}, perr)
}

// pairFS returns a file system with the VobSub pair two-tracks-pal under the names of idx and sub.
func pairFS(t *testing.T, idx, sub string) fstest.MapFS {
	t.Helper()

	return fstest.MapFS{
		idx: {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")},
		sub: {Data: testfiles.Read(t, "vobsub/two-tracks-pal.sub")},
	}
}

func TestLoadFSFindsThePairOfAVobSubFile(t *testing.T) {
	t.Parallel()

	windows1252 := []byte("# VobSub index file, v7\nsize: 720x576\npalette: 000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000, " +
		"000000, 000000, 000000, 000000, 000000, 000000, 000000, 000000\nid: Fr\xe9, index: 0\ntimestamp: 00:00:01:000, filepos: 000000000\n")
	sub := testfiles.Read(t, "vobsub/one-packet-pal.sub")
	tests := []struct {
		name         string
		fsys         fs.FS
		path         string
		opts         stbox.ReadOptions
		wantCues     int
		wantLanguage string
		wantErr      error
		wantMessage  string
	}{
		{"an .idx name looks for .sub", pairFS(t, "movie.idx", "movie.sub"), "movie.idx", stbox.ReadOptions{}, 5, "en", nil, ""},
		{"an .idx name looks for .SUB", pairFS(t, "movie.idx", "movie.SUB"), "movie.idx", stbox.ReadOptions{}, 5, "en", nil, ""},
		{"upper-case extensions", pairFS(t, "MOVIE.IDX", "MOVIE.SUB"), "MOVIE.IDX", stbox.ReadOptions{}, 5, "en", nil, ""},
		{"a .sub name with the format looks for .idx", pairFS(t, "movie.idx", "movie.sub"), "movie.sub", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"a .sub name with the format looks for .IDX", pairFS(t, "movie.IDX", "movie.sub"), "movie.sub", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"a name without an extension gets a dot", pairFS(t, "movie.idx", "movie."), "movie.", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"a name without a dot", pairFS(t, "movie.idx", "movie"), "movie", stbox.ReadOptions{Format: stbox.VobSub}, 5, "en", nil, ""},
		{"the track option", pairFS(t, "movie.idx", "movie.sub"), "movie.idx", stbox.ReadOptions{VobSub: vobsub.ReadOptions{Language: "de"}}, 2, "de", nil, ""},
		{"the encoding decodes the .idx file", fstest.MapFS{"a.idx": {Data: windows1252}, "a.sub": {Data: sub}}, "a.idx", stbox.ReadOptions{Encoding: "Windows-1252"}, 1, "Fr\u00e9", nil, ""},
		{
			"a missing .sub file",
			fstest.MapFS{"movie.idx": {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")}},
			"movie.idx",
			stbox.ReadOptions{},
			0, "",
			fs.ErrNotExist, "load subtitle: VobSub needs the file movie.sub next to movie.idx: file does not exist",
		},
		{
			"a missing .idx file",
			fstest.MapFS{"movie.sub": {Data: sub}},
			"movie.sub",
			stbox.ReadOptions{Format: stbox.VobSub},
			0, "",
			fs.ErrNotExist, "load subtitle: VobSub needs the file movie.idx next to movie.sub: file does not exist",
		},
		{
			"a directory is no partner",
			fstest.MapFS{"movie.idx": {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")}, "movie.sub/x": {}},
			"movie.idx",
			stbox.ReadOptions{},
			0, "",
			fs.ErrNotExist, "load subtitle: VobSub needs the file movie.sub next to movie.idx: file does not exist",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.LoadFS(tt.fsys, tt.path, tt.opts)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.EqualError(t, err, tt.wantMessage)
				return
			}
			require.NoError(t, err)
			language, _ := s.Metadata("language")
			assert.Equal(t, [4]any{tt.wantCues, tt.wantLanguage, stbox.VobSub, ""}, [4]any{s.Len(), language, info.Format, info.SourceEncoding})
		})
	}
}

func TestLoadVobSubFromFiles(t *testing.T) {
	t.Parallel()

	idx := testfiles.Path(t, "vobsub/two-tracks-pal.idx")
	sub := testfiles.Path(t, "vobsub/two-tracks-pal.sub")
	for _, tt := range []struct {
		name, path string
		opts       stbox.ReadOptions
	}{
		{"the .idx file", idx, stbox.ReadOptions{}},
		{"the .sub file with the format", sub, stbox.ReadOptions{Format: stbox.VobSub}},
		{"the .idx file with a wrong IDX option", idx, stbox.ReadOptions{VobSub: vobsub.ReadOptions{IDX: "wrong"}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Load(tt.path, tt.opts)
			require.NoError(t, err)
			assert.Equal(t, [3]any{5, stbox.VobSub, stbox.Container("")}, [3]any{s.Len(), info.Format, info.Container})
		})
	}
}

func TestLoadOfAMicroDVDSubFileNeverLooksForAnIndex(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"a.sub": {Data: testfiles.Read(t, "microdvd/valid.sub")},
		"a.idx": {Data: testfiles.Read(t, "vobsub/two-tracks-pal.idx")},
	}
	_, info, err := stbox.LoadFS(fsys, "a.sub", stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, stbox.MicroDVD, info.Format)
}

func TestParseVobSub(t *testing.T) {
	t.Parallel()

	idx := testfiles.Read(t, "vobsub/two-tracks-pal.idx")
	sub := testfiles.Read(t, "vobsub/two-tracks-pal.sub")
	read := func(opts stbox.ReadOptions) (int, error) {
		s, _, err := stbox.Parse(bytes.NewReader(sub), int64(len(sub)), opts)
		if err != nil {
			return 0, err
		}
		return s.Len(), nil
	}

	n, err := read(stbox.ReadOptions{Format: stbox.VobSub, VobSub: vobsub.ReadOptions{IDX: string(idx), Index: new(1)}})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	_, err = read(stbox.ReadOptions{Format: stbox.VobSub})
	var aerr *stbox.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "ReadOptions.VobSub.IDX", aerr.Option)

	_, err = read(stbox.ReadOptions{Format: stbox.VobSub, VobSub: vobsub.ReadOptions{IDX: string(idx), Index: new(-1)}})
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "ReadOptions.VobSub.Index", aerr.Option)
}

// Ports the decode failure of testLegacyFileParsesWithItsSourceEncoding of tests/EncodingTest.php of
// subtitle-toolbox-php: a decode failure is a parse error in both modes.
func TestParseDecodeFailure(t *testing.T) {
	t.Parallel()

	for _, lenient := range []bool{false, true} {
		t.Run(fmt.Sprintf("lenient %t", lenient), func(t *testing.T) {
			t.Parallel()

			s, info, err := parseBytes([]byte("caf\xE9\x81"), stbox.ReadOptions{Encoding: "Windows-1252", Lenient: lenient})
			var perr *stbox.ParseError
			require.ErrorAs(t, err, &perr)
			require.ErrorIs(t, err, stbox.ErrParse)
			assert.Equal(t, "Cannot convert the content from Windows-1252 to UTF-8.", perr.Msg)
			var cerr *textenc.ConvertError
			require.ErrorAs(t, err, &cerr)
			assert.Equal(t, "Windows-1252", cerr.Encoding)
			assert.Equal(t, "stbox: Cannot convert the content from Windows-1252 to UTF-8.", err.Error())
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestParseUnknownEncoding(t *testing.T) {
	t.Parallel()

	_, _, err := parseBytes([]byte("Hello"), stbox.ReadOptions{Encoding: "CP1125"})
	var aerr *stbox.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "ReadOptions.Encoding", aerr.Option)
	assert.Equal(t, `The encoding "CP1125" is unknown.`, aerr.Msg)
}

type encodingRead struct {
	File           string   `json:"file"`
	Encoding       string   `json:"encoding"`
	Format         string   `json:"format"`
	SourceEncoding string   `json:"sourceEncoding"`
	Cues           int      `json:"cues"`
	Warnings       []string `json:"warnings"`
	Error          string   `json:"error"`
	StrictError    string   `json:"strictError"`
}

func encodingReads(t *testing.T) []encodingRead {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "encoding", "reads", "reads.json"))
	require.NoError(t, err)
	var rows []encodingRead
	require.NoError(t, json.Unmarshal(data, &rows))
	require.Len(t, rows, 98)
	return rows
}

// Ports tests/EncodingTest.php of subtitle-toolbox-php for the decode step: each fixture with and without
// Encoding "Windows-1252" gives the source encoding and the decode warning.
func TestEncodingFixturesDecode(t *testing.T) {
	t.Parallel()

	for _, tt := range encodingReads(t) {
		t.Run(tt.File+", "+tt.Encoding, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join("testdata", "encoding", filepath.FromSlash(tt.File)))
			require.NoError(t, err)
			got, err := textenc.Decode(data, tt.Encoding)
			if tt.Error != "" {
				require.EqualError(t, err, tt.Error)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.SourceEncoding, got.Encoding)
			var decodeWarnings []string
			for _, w := range tt.Warnings {
				if strings.HasPrefix(w, textenc.UTF16WarningPrefix) || strings.HasPrefix(w, textenc.DetectedWarningPrefix) {
					decodeWarnings = append(decodeWarnings, w)
				}
			}
			if got.Warning == "" {
				assert.Empty(t, decodeWarnings)
			} else {
				assert.Equal(t, []string{got.Warning}, decodeWarnings)
			}
		})
	}
}

// Ports tests/EncodingTest.php of subtitle-toolbox-php through Load: format, source encoding, cue count and
// warnings in lenient mode, and the error in strict mode.
func TestEncodingFixturesLoad(t *testing.T) {
	t.Parallel()

	for _, tt := range encodingReads(t) {
		t.Run(tt.File+", "+tt.Encoding, func(t *testing.T) {
			t.Parallel()

			f := stbox.Format(tt.Format)
			path := filepath.Join("testdata", "encoding", filepath.FromSlash(tt.File))
			opts := stbox.ReadOptions{Format: f, Encoding: tt.Encoding, Lenient: true}
			if tt.Error != "" {
				opts.Format = ""
			}
			s, info, err := stbox.Load(path, opts)
			if tt.Error != "" {
				var perr *stbox.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.Error, perr.Msg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, f, info.Format)
			assert.Equal(t, tt.SourceEncoding, info.SourceEncoding)
			assert.Equal(t, tt.Cues, s.Len())
			messages := make([]string, 0, len(info.Warnings))
			for _, w := range info.Warnings {
				messages = append(messages, w.Message)
				assert.Equal(t, stbox.Repaired, w.Action)
			}
			assert.Equal(t, tt.Warnings, messages)

			opts.Lenient = false
			_, strict, err := stbox.Load(path, opts)
			if tt.StrictError != "" {
				var perr *stbox.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, tt.StrictError, perr.Msg)
				return
			}
			require.NoError(t, err)
			assert.Empty(t, strict.Warnings)
			assert.Equal(t, tt.SourceEncoding, strict.SourceEncoding)
		})
	}
}

// Ports testTextReadWithItsEncodingIsWritten of tests/Formatters/XmlOutputTest.php of subtitle-toolbox-php, with the
// TTML row. The iTT row comes with the iTT writer.
func TestTextReadWithItsEncodingIsWrittenAsTTML(t *testing.T) {
	t.Parallel()

	s, _, err := stbox.Load("testdata/ass/own_windows_1252.ass", stbox.ReadOptions{Format: stbox.ASS, Encoding: "Windows-1252"})
	require.NoError(t, err)
	var out bytes.Buffer
	_, err = stbox.Write(&out, s, stbox.TTML, stbox.WriteOptions{})
	require.NoError(t, err)
	assert.Contains(t, out.String(), "Le café ouvre à sept heures.")

	back, _, err := parseBytes(out.Bytes(), stbox.ReadOptions{Format: stbox.TTML})
	require.NoError(t, err)
	assert.Equal(t, 3, back.Len())
}

func TestWriteUnsupportedFormat(t *testing.T) {
	t.Parallel()

	formats := []stbox.Format{"foo", ""}
	for _, f := range stbox.Formats() {
		if !f.CanWrite() {
			formats = append(formats, f)
		}
	}
	for _, f := range formats {
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			_, err := stbox.Write(&out, &stbox.Subtitle{}, f, stbox.WriteOptions{})
			require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
			assert.Zero(t, out.Len())
		})
	}
}

func TestWriteReportIsSetForWebVTTSCCAndMCCOnly(t *testing.T) {
	t.Parallel()

	s := &stbox.Subtitle{}
	require.NoError(t, s.AddCue(stbox.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{`<font color="#123456">dark</font>`}}))
	for _, f := range stbox.Formats() {
		if !f.CanWrite() || f == stbox.MicroDVD || f == stbox.PGS {
			continue
		}
		t.Run(string(f), func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			report, err := stbox.Write(&out, s, f, stbox.WriteOptions{ITT: itt.WriteOptions{FrameRate: stbox.FrameRate25}})
			require.NoError(t, err)
			assert.Equal(t, f == stbox.SCC, report.SCC != nil)
			assert.Equal(t, f == stbox.MCC, report.MCC != nil)
			if f != stbox.WebVTT {
				assert.Nil(t, report.WebVTT)
				return
			}
			require.NotNil(t, report.WebVTT)
			assert.Len(t, report.WebVTT.DroppedColors, 1)
		})
	}
}

func TestWritePlainText(t *testing.T) {
	t.Parallel()

	s := &stbox.Subtitle{}
	require.NoError(t, s.AddCues(
		stbox.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<i>Hello</i>", "world."}},
		stbox.Cue{Start: 2500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"Where are you going?"}},
		stbox.Cue{Start: 6 * time.Second, End: 7 * time.Second, Lines: []string{"Home &amp; bed."}},
	))
	var out bytes.Buffer
	_, err := stbox.Write(&out, s, stbox.PlainText, stbox.WriteOptions{})
	require.NoError(t, err)
	assert.Equal(t, "Hello world. Where are you going?\n\nHome & bed.\n", out.String())

	out.Reset()
	opts := stbox.WriteOptions{LineEnding: stbox.CRLF, PlainText: plaintext.WriteOptions{WithTimes: true, ParagraphGap: new(plaintext.NoParagraphBreaks)}}
	_, err = stbox.Write(&out, s, stbox.PlainText, opts)
	require.NoError(t, err)
	assert.Equal(t, "[00:00:01] Hello world. Where are you going? Home & bed.\r\n", out.String())

	out.Reset()
	_, err = stbox.Write(&out, s, stbox.PlainText, stbox.WriteOptions{PlainText: plaintext.WriteOptions{ParagraphGap: new(-time.Second)}})
	require.ErrorIs(t, err, stbox.ErrInvalidArgument)
	assert.Zero(t, out.Len())
}

func TestParsePlainTextIsUnsupported(t *testing.T) {
	t.Parallel()

	_, _, err := parseBytes([]byte("Hello\n"), stbox.ReadOptions{Format: stbox.PlainText})
	require.ErrorIs(t, err, stbox.ErrUnsupportedFormat)
	require.EqualError(t, err, "stbox: unsupported format: The format txt can be written but not read.")
}

func TestBinaryWriteIgnoresLineEndingAndBOM(t *testing.T) {
	t.Parallel()

	s := &stbox.Subtitle{}
	c := stbox.Cue{Start: time.Second, End: 2 * time.Second}
	c.AddLine("Hello")
	require.NoError(t, s.AddCue(c))
	gsi := model.NewData(model.Field{Key: "CD", Value: "261012"}, model.Field{Key: "RD", Value: "261012"})
	require.NoError(t, s.SetFormatData("stl", model.NewData(model.Field{Key: "gsi", Value: gsi})))
	write := func(opts stbox.WriteOptions) []byte {
		var out bytes.Buffer
		_, err := stbox.Write(&out, s, stbox.EBUSTL, opts)
		require.NoError(t, err)
		return out.Bytes()
	}
	assert.Equal(t, write(stbox.WriteOptions{}), write(stbox.WriteOptions{LineEnding: stbox.CRLF, BOM: stbox.BOMAdd}))
}
