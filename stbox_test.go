package stbox_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/plaintext"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

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
		{"JSON object of no format", `{"segments": []}`},
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
	s.AddCue(stbox.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{`<font color="#123456">dark</font>`}})
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
	s.AddCues(
		stbox.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<i>Hello</i>", "world."}},
		stbox.Cue{Start: 2500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"Where are you going?"}},
		stbox.Cue{Start: 6 * time.Second, End: 7 * time.Second, Lines: []string{"Home &amp; bed."}},
	)
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

func TestZeroOptionsAreValid(t *testing.T) {
	t.Parallel()

	require.NoError(t, stbox.ReadOptions{}.Validate())
	require.NoError(t, stbox.WriteOptions{}.Validate())
}

func TestSentinelsAreTheModelSentinels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		root, model error
	}{
		{stbox.ErrParse, model.ErrParse},
		{stbox.ErrUnsupportedFormat, model.ErrUnsupportedFormat},
		{stbox.ErrUnknownFormat, model.ErrUnknownFormat},
		{stbox.ErrInvalidArgument, model.ErrInvalidArgument},
		{stbox.ErrCueNotFound, model.ErrCueNotFound},
		{stbox.ErrUnwritable, model.ErrUnwritable},
	}
	for _, tt := range tests {
		t.Run(tt.root.Error(), func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, fmt.Errorf("read movie.srt: %w", tt.model), tt.root)
			require.ErrorIs(t, fmt.Errorf("read movie.srt: %w", tt.root), tt.model)
		})
	}
}

func ExampleFormatFromPath() {
	for _, path := range []string{"movie.SUB", "notes.txt", "clip.xml", "movie"} {
		f, ok := stbox.FormatFromPath(path)
		fmt.Printf("%s: %q %t\n", path, f, ok)
	}
	// Output:
	// movie.SUB: "microdvd" true
	// notes.txt: "txt" true
	// clip.xml: "ttml" true
	// movie: "" false
}

func ExampleFormats() {
	for _, f := range stbox.Formats() {
		fmt.Println(f, f.Extensions())
	}
	// Output:
	// ass [ass ssa]
	// cavena890 [890]
	// csv [csv]
	// itt [itt]
	// json [json]
	// lrc [lrc]
	// mcc [mcc]
	// microdvd [sub]
	// mpsub [mpsub]
	// pac [pac fpc]
	// pgs [sup]
	// sami [smi sami]
	// sbv [sbv]
	// scc [scc]
	// srt [srt]
	// stl [stl]
	// spruce-stl [stl]
	// subviewer [sub]
	// ttml [ttml dfxp xml]
	// dcp [xml]
	// tsv [tsv]
	// txt [txt]
	// mpl2 [txt]
	// tmplayer [txt]
	// qttext [txt]
	// vobsub [idx]
	// vtt [vtt]
	// youtube [srv3 json3 srv1 ytt]
	// youtube-chapters [txt]
}

func TestBinaryWriteIgnoresLineEndingAndBOM(t *testing.T) {
	t.Parallel()

	s := &stbox.Subtitle{}
	c := stbox.Cue{Start: time.Second, End: 2 * time.Second}
	c.AddLine("Hello")
	s.AddCue(c)
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
