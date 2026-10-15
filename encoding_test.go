package stbox_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/internal/textenc"
)

func ExampleEncodings() {
	names := stbox.Encodings()
	fmt.Println(names[:4])
	fmt.Println(slices.Contains(names, "Windows-1252"), slices.Contains(names, "CP1125"))
	// Output:
	// [UTF-8 UTF-16LE UTF-16BE UTF-16]
	// true false
}

// Ports the decode failure of testLegacyFileParsesWithItsSourceEncoding of tests/EncodingTest.php of
// subtitle-toolbox-php: a decode failure is a parse error in both modes.
func TestParseDecodeFailure(t *testing.T) {
	t.Parallel()

	for _, lenient := range []bool{false, true} {
		t.Run(fmt.Sprintf("lenient %t", lenient), func(t *testing.T) {
			t.Parallel()

			s, info, err := stbox.Parse(bytes.NewReader([]byte("caf\xE9\x81")), stbox.ReadOptions{Encoding: "Windows-1252", Lenient: lenient})
			var perr *stbox.ParseError
			require.ErrorAs(t, err, &perr)
			require.ErrorIs(t, err, stbox.ErrParse)
			assert.Equal(t, "Cannot convert the content from Windows-1252 to UTF-8.", perr.Msg)
			assert.Equal(t, "stbox: Cannot convert the content from Windows-1252 to UTF-8.", err.Error())
			assert.Nil(t, s)
			assert.Zero(t, info)
		})
	}
}

func TestParseUnknownEncoding(t *testing.T) {
	t.Parallel()

	_, _, err := stbox.Parse(strings.NewReader("Hello"), stbox.ReadOptions{Encoding: "CP1125"})
	var aerr *stbox.InvalidArgumentError
	require.ErrorAs(t, err, &aerr)
	assert.Equal(t, "ReadOptions.Encoding", aerr.Option)
	assert.Equal(t, `The encoding "CP1125" is unknown.`, aerr.Msg)
}

// readerIssue names the issue that adds the reader of each format that the encoding fixtures need.
var readerIssue = map[stbox.Format]string{
	stbox.SubRip:    "#20",
	stbox.WebVTT:    "#21",
	stbox.ASS:       "#22",
	stbox.TTML:      "#23",
	stbox.SAMI:      "#25",
	stbox.SBV:       "#26",
	stbox.MicroDVD:  "#27",
	stbox.SubViewer: "#30",
	stbox.CSV:       "#33",
	"youtube":       "#102",
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

	data, err := os.ReadFile(filepath.Join("testdata", "encoding-reads", "reads.json"))
	require.NoError(t, err)
	var rows []encodingRead
	require.NoError(t, json.Unmarshal(data, &rows))
	require.Len(t, rows, 98)
	return rows
}

// Ports tests/EncodingTest.php of subtitle-toolbox-php for the decode step: each fixture with and without
// Encoding "Windows-1252" gives the source encoding and the decode warning of the PHP read.
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
			if tt.Error == "" && !f.CanRead() {
				t.Skip("needs the " + tt.Format + " reader of " + readerIssue[f])
			}
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
			assert.Len(t, info.Warnings, len(tt.Warnings))

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
