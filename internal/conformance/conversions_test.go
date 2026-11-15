package conformance_test

import (
	"bytes"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/stboxjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/subviewer"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

type structureTextCase struct {
	name    string
	lines   []string
	written string
	want    []string
}

// TestWriteCueTextThatReadsAsStructure writes cue text that the reader of the format would read as a line break, a
// cue or a header line. The output must read back in strict and in lenient mode with 3 cues and the expected lines.
func TestWriteCueTextThatReadsAsStructure(t *testing.T) {
	t.Parallel()

	sv1 := stbox.WriteOptions{SubViewer: subviewer.WriteOptions{Version: subviewer.V1}}
	sv2 := stbox.WriteOptions{SubViewer: subviewer.WriteOptions{Version: subviewer.V2}}
	fps := stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}}
	writers := []struct {
		name   string
		format stbox.Format
		opts   stbox.WriteOptions
		cases  []structureTextCase
	}{
		{"SubRip", stbox.SubRip, stbox.WriteOptions{}, []structureTextCase{
			{
				"timing line after a line break and a space",
				[]string{"x\n 00:00:03,500 --> 00:00:03,800"},
				"\nx\n\u2060 00:00:03,500",
				[]string{"x", "\u2060 00:00:03,500 --&gt; 00:00:03,800"},
			},
			{
				"timing line after a BOM",
				[]string{"\ufeff00:00:03,500 --> 00:00:03,800"},
				"\n\u2060\ufeff00:00:03,500",
				[]string{"\u2060\ufeff00:00:03,500 --&gt; 00:00:03,800"},
			},
			{"line of only a BOM", []string{"x", "\ufeff", "y"}, "\nx\ny\n", []string{"x", "y"}},
			{"line break inside a line", []string{"a\nb"}, "\na\nb\n", []string{"a", "b"}},
			{
				"timing line",
				[]string{"00:00:03,500 --> 00:00:03,800"},
				"\n\u206000:00:03,500 --> 00:00:03,800\n",
				[]string{"\u206000:00:03,500 --&gt; 00:00:03,800"},
			},
			{"timing line of lenient mode", []string{"10:30 -> 11:00"}, "\n\u206010:30 -> 11:00\n", []string{"\u206010:30 -&gt; 11:00"}},
			{
				"full-width timing line",
				[]string{"00\uff1a00\uff1a03,500 --> 00:00:03,800"},
				"\n\u206000\uff1a00",
				[]string{"\u206000\uff1a00\uff1a03,500 --&gt; 00:00:03,800"},
			},
			{
				"cue number and timing line",
				[]string{"x", "2", "00:00:03,500 --> 00:00:03,800", "y"},
				"\nx\n2\n\u206000:00:03,500",
				[]string{"x", "2", "\u206000:00:03,500 --&gt; 00:00:03,800", "y"},
			},
			{"arrow inside a line", []string{"a --> b"}, "\na --> b\n", []string{"a --&gt; b"}},
		}},
		{"WebVTT", stbox.WebVTT, stbox.WriteOptions{}, []structureTextCase{
			{"line of only a dropped tag", []string{"x", "<p></p>", "y"}, "\nx\ny\n", []string{"x", "y"}},
			{"line of only a font face", []string{"x", "<font face=\"Arial\"></font>", "y"}, "\nx\ny\n", []string{"x", "y"}},
			{"line of only a BOM", []string{"x", "\ufeff", "y"}, "\nx\ny\n", []string{"x", "y"}},
		}},
		{"WebVTT without tags", stbox.WebVTT, stbox.WriteOptions{StripTags: true}, []structureTextCase{
			{"line of only a tag", []string{"x", "<i></i>", "y"}, "\nx\ny\n", []string{"x", "y"}},
			{"line break inside a line", []string{"a\nb"}, "\na\nb\n", []string{"a", "b"}},
			{
				"timing line",
				[]string{"00:00:03.500 --> 00:00:03.800"},
				"\n00:00:03.500 --&gt; 00:00:03.800\n",
				[]string{"00:00:03.500 --&gt; 00:00:03.800"},
			},
		}},
		{"SBV", stbox.SBV, stbox.WriteOptions{}, []structureTextCase{
			{"timing line after a BOM", []string{"\ufeff0:00:03.500,0:00:03.800"}, "\n\u2060\ufeff0:00:03.500", []string{"\u2060\ufeff0:00:03.500,0:00:03.800"}},
			{"line of only a BOM", []string{"x", "\ufeff", "y"}, "\nx\ny\n", []string{"x", "y"}},
			{"line break inside a line", []string{"a\nb"}, "\na\nb\n", []string{"a", "b"}},
			{
				"timing line",
				[]string{"0:00:03.500,0:00:03.800"},
				"\n\u20600:00:03.500,0:00:03.800\n",
				[]string{"\u20600:00:03.500,0:00:03.800"},
			},
			{"start of a timing line", []string{"0:00:03.5,x"}, "\n\u20600:00:03.5,x\n", []string{"\u20600:00:03.5,x"}},
			{
				"timing line of lenient mode",
				[]string{"0:00:03.500 . 0:00:03.800"},
				"\n\u20600:00:03.500 . 0:00:03.800\n",
				[]string{"\u20600:00:03.500 . 0:00:03.800"},
			},
			{"comma inside a line", []string{"a,b"}, "\na,b\n", []string{"a,b"}},
		}},
		{"SubViewer 1", stbox.SubViewer, sv1, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "\na|b\n", []string{"a", "b"}},
			{"pipe", []string{"a|b"}, "\na\u00a6b\n", []string{"a\u00a6b"}},
			{"time line", []string{"[00:00:03]"}, "\n\u2060[00:00:03]\n", []string{"\u2060[00:00:03]"}},
			{"time and text", []string{"[00:00:03] x"}, "\n[00:00:03] x\n", []string{"[00:00:03] x"}},
		}},
		{"SubViewer 2", stbox.SubViewer, sv2, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "\na[br]b\n", []string{"a", "b"}},
			{"line break tag", []string{"a[br]b"}, "\na[\u2060br]b\n", []string{"a[\u2060br]b"}},
			{"line break tag in upper case", []string{"a[BR]b"}, "\na[BR]b\n", []string{"a[BR]b"}},
			{
				"timing line",
				[]string{"00:00:03.50,00:00:03.80"},
				"\n\u206000:00:03.50,\u206000:00:03.80\n",
				[]string{"\u206000:00:03.50,\u206000:00:03.80"},
			},
			{"time before a comma", []string{"0:00:03.5,x"}, "\n\u20600:00:03.5,x\n", []string{"\u20600:00:03.5,x"}},
			{"time after a comma", []string{"x", "a,00:00:03.80"}, "\nx[br]a,\u206000:00:03.80\n", []string{"x", "a,\u206000:00:03.80"}},
			{"style line", []string{"[COLF]&amp;HFFFFFF", "x"}, "\n\u2060[COLF]&HFFFFFF[br]x\n", []string{"\u2060[COLF]&amp;HFFFFFF", "x"}},
			{
				"script line",
				[]string{"******** START SCRIPT ********"},
				"\n\u2060******** START SCRIPT ********\n",
				[]string{"\u2060******** START SCRIPT ********"},
			},
		}},
		{"MicroDVD", stbox.MicroDVD, fps, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "}a|b\n", []string{"a", "b"}},
			{"line break entity", []string{"<i>a&#10;b</i>"}, "}{y:i}a|{y:i}b\n", []string{"<i>a</i>", "<i>b</i>"}},
			{"line break entity after italics", []string{"<i>a</i>&#10;b"}, "}{y:i}a|b\n", []string{"<i>a</i>", "b"}},
			{"pipe", []string{"a|b"}, "}a\u00a6b\n", []string{"a\u00a6b"}},
		}},
		{"MPL2", stbox.MPL2, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "]a|b\n", []string{"a", "b"}},
			{"line break inside italics", []string{"<i>a\nb</i>"}, "]/a|/b\n", []string{"<i>a</i>", "<i>b</i>"}},
			{"line break after italics", []string{"<i>a</i>\nb"}, "]/a|b\n", []string{"<i>a</i>", "b"}},
			{"pipe", []string{"a|b"}, "]a\u00a6b\n", []string{"a\u00a6b"}},
		}},
		{"TMPlayer", stbox.TMPlayer, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, ":a|b\n", []string{"a", "b"}},
			{"pipe", []string{"a|b"}, ":a\u00a6b\n", []string{"a\u00a6b"}},
		}},
		{"MPSub", stbox.MPSub, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "\na\nb\n", []string{"a", "b"}},
			{"timing line", []string{"x", "1 2"}, "\nx\n1 2\n", []string{"x", "1 2"}},
		}},
		{"LRC", stbox.LRC, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "] a b\n", []string{"a b"}},
			{"line break entity", []string{"a&#10;b"}, "] a b\n", []string{"a b"}},
			{"time tag after a line break", []string{"x\n[00:03.50]y"}, "] x [00:03.50]y\n", []string{"x [00:03.50]y"}},
		}},
		{"CSV", stbox.CSV, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, ",\"a\nb\"\n", []string{"a", "b"}},
		}},
		{"TSV", stbox.TSV, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "\t\"a\nb\"\n", []string{"a", "b"}},
		}},
		{"ASS", stbox.ASS, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, ",a\\Nb\n", []string{"a", "b"}},
			{"line break code", []string{`a\Nb`}, ",a\\\u2060Nb\n", []string{"a\\\u2060Nb"}},
			{"soft line break code", []string{`a\nb`}, ",a\\\u2060nb\n", []string{"a\\\u2060nb"}},
			{"hard space code", []string{`a\hb`}, ",a\\\u2060hb\n", []string{"a\\\u2060hb"}},
		}},
		{"QuickTime text", stbox.QuickTimeText, stbox.WriteOptions{}, []structureTextCase{
			{"line break inside a line", []string{"a\nb"}, "\na\nb\n", []string{"a", "b"}},
			{"timestamp line", []string{"x", "[00:00:03]y"}, "\nx\n\u2060[00:00:03]y\n", []string{"x", "\u2060[00:00:03]y"}},
			{"timestamp beyond the time scale", []string{"[00:00:03.50]"}, "\n\u2060[00:00:03.50]\n", []string{"\u2060[00:00:03.50]"}},
		}},
		{"SAMI", stbox.SAMI, stbox.WriteOptions{}, []structureTextCase{
			{"line feed entity", []string{"a&#10;b"}, ">a<br>b\n", []string{"a", "b"}},
			{"line feed entity in hex", []string{"a&#x0A;b"}, ">a<br>b\n", []string{"a", "b"}},
			{"carriage return entity", []string{"a&#13;b"}, ">a<br>b\n", []string{"a", "b"}},
			{"named line feed entity", []string{"a&NewLine;b"}, ">a<br>b\n", []string{"a", "b"}},
			{"line break inside a line", []string{"a\nb"}, ">a<br>b\n", []string{"a", "b"}},
		}},
	}
	for _, w := range writers {
		for _, tt := range w.cases {
			t.Run(w.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				s := &stbox.Subtitle{}
				s.AddCues(
					stbox.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"A"}},
					stbox.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: tt.lines},
					stbox.Cue{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"C"}},
				)
				var out bytes.Buffer
				_, err := stbox.Write(&out, s, w.format, w.opts)
				require.NoError(t, err)
				assert.Contains(t, out.String(), tt.written)
				for _, lenient := range []bool{false, true} {
					opts := stbox.ReadOptions{Format: w.format, Lenient: lenient, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}}
					back, _, err := parseBytes(out.Bytes(), opts)
					require.NoError(t, err, "lenient %v", lenient)
					require.Equal(t, 3, back.Len(), "lenient %v", lenient)
					c, _ := back.Cue(1)
					assert.Equal(t, tt.want, c.Lines, "lenient %v", lenient)
				}
			})
		}
	}
}

func TestOffsetRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		format     stbox.Format
		alignment  stbox.Alignment
		offset     stbox.Offset
		wantOffset *stbox.Offset
	}{
		{"TTML bottom center", stbox.TTML, stbox.BottomCenter, stbox.Offset{X: 50, Y: 95}, &stbox.Offset{X: 50, Y: 95}},
		{"TTML default anchor reads back as none", stbox.TTML, stbox.BottomCenter, stbox.Offset{X: 50, Y: 90}, nil},
		{"TTML top left", stbox.TTML, stbox.TopLeft, stbox.Offset{X: 10, Y: 5}, &stbox.Offset{X: 10, Y: 5}},
		{"TTML middle right", stbox.TTML, stbox.MiddleRight, stbox.Offset{X: 80, Y: 40}, &stbox.Offset{X: 80, Y: 40}},
		{"DCP bottom center", stbox.DCP, stbox.BottomCenter, stbox.Offset{X: 50, Y: 80}, &stbox.Offset{X: 50, Y: 80}},
		{"DCP bottom center in the usual place reads back as none", stbox.DCP, stbox.BottomCenter, stbox.Offset{X: 50, Y: 95}, nil},
		{"DCP top left", stbox.DCP, stbox.TopLeft, stbox.Offset{X: 20, Y: 25}, &stbox.Offset{X: 20, Y: 25}},
		{"DCP top left at the default anchor reads back as none", stbox.DCP, stbox.TopLeft, stbox.Offset{X: 10, Y: 8}, nil},
		{"DCP middle right", stbox.DCP, stbox.MiddleRight, stbox.Offset{X: 80, Y: 40}, &stbox.Offset{X: 80, Y: 40}},
		{"WebVTT bottom center", stbox.WebVTT, stbox.BottomCenter, stbox.Offset{X: 50, Y: 95}, &stbox.Offset{X: 50, Y: 95}},
		{"WebVTT top left keeps the column, not the height", stbox.WebVTT, stbox.TopLeft, stbox.Offset{X: 20, Y: 5}, &stbox.Offset{X: 20, Y: 0}},
		{"WebVTT top center keeps no offset", stbox.WebVTT, stbox.TopCenter, stbox.Offset{X: 50, Y: 10}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := stbox.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"One"}}
			c.Position = stbox.Position{Alignment: tt.alignment, Offset: &tt.offset}
			s := &stbox.Subtitle{}
			s.AddCues(c)

			var out bytes.Buffer
			_, err := stbox.Write(&out, s, tt.format, stbox.WriteOptions{})
			require.NoError(t, err)
			back, _, err := parseBytes(out.Bytes(), stbox.ReadOptions{Format: tt.format})
			require.NoError(t, err)
			got, ok := back.Cue(0)
			require.True(t, ok)
			assert.Equal(t, tt.alignment, got.Position.Alignment)
			assert.Equal(t, tt.wantOffset, got.Position.Offset)
		})
	}
}

func TestWebVTTHeightConvertsToDCPAndBack(t *testing.T) {
	t.Parallel()

	vtt := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000 line:80%,end\nOne\n"
	s, _, err := parseBytes([]byte(vtt), stbox.ReadOptions{})
	require.NoError(t, err)
	var out bytes.Buffer
	_, err = stbox.Write(&out, s, stbox.DCP, stbox.WriteOptions{})
	require.NoError(t, err)
	assert.Contains(t, out.String(), `<Text Valign="bottom" Vposition="20">One</Text>`)

	back, _, err := parseBytes(out.Bytes(), stbox.ReadOptions{})
	require.NoError(t, err)
	got, _ := back.Cue(0)
	assert.Equal(t, stbox.BottomCenter, got.Position.Alignment)
	assert.Equal(t, &stbox.Offset{X: 50, Y: 80}, got.Position.Offset)
}

// TestCappedLastCueEndReadsBack reads a file whose last cue end the reader caps at 99999:59:59.999, writes it in
// every format that has a writer and a reader, and reads that output.
func TestCappedLastCueEndReadsBack(t *testing.T) {
	t.Parallel()

	inputs := []struct {
		name    string
		format  stbox.Format
		content string
	}{
		{"SAMI start at the limit", stbox.SAMI, "<SAMI><BODY><SYNC Start=359999999999><P>last</BODY></SAMI>"},
		{"SAMI start 3 s before the limit", stbox.SAMI, "<SAMI><BODY><SYNC Start=359999997000><P>last</BODY></SAMI>"},
		{"TMPlayer start at 99999:59:59", stbox.TMPlayer, "99999:59:59:last\n"},
		{"TMPlayer start 3 s before the limit", stbox.TMPlayer, "99999:59:57:last\n"},
		// MPL2 and MPSub have no cue without an end, so their rows end just below the limit.
		{"LRC start near the limit", stbox.LRC, "[offset:-359999987000]\n[00:12.00]last\n"},
		{"MPL2 end at the limit", stbox.MPL2, "[3599999990][3599999999]last\n"},
		{"MPSub end at the limit", stbox.MPSub, "FORMAT=TIME\n\n359999999 0.999\nlast\n"},
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			t.Parallel()

			opts := stbox.ReadOptions{Format: in.format, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}}
			s, _, err := parseBytes([]byte(in.content), opts)
			require.NoError(t, err)
			require.Equal(t, 1, s.Len())
			if in.format != stbox.MPL2 && in.format != stbox.MPSub {
				c, _ := s.Cue(0)
				require.Equal(t, timecode.MaxValidTime, c.End)
			}

			written := 0
			for _, f := range stbox.Formats() {
				if !f.CanRead() || f == stbox.PGS {
					continue
				}
				var out bytes.Buffer
				_, err := stbox.Write(&out, s, f, stbox.WriteOptions{
					MicroDVD: microdvd.WriteOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}},
					ITT:      itt.WriteOptions{FrameRate: stbox.FrameRate25},
				})
				if errors.Is(err, stbox.ErrUnsupportedFormat) {
					continue
				}
				require.NoError(t, err, f)
				written++

				text := out.String()
				back, _, err := parseBytes(out.Bytes(), stbox.ReadOptions{Format: f, MicroDVD: microdvd.ReadOptions{FrameRate: stbox.FrameRate{Num: 25, Den: 1}}})
				require.NoError(t, err, "%s: %q", f, text)
				require.Equal(t, 1, back.Len(), f)
				c, _ := back.Cue(0)
				assert.Less(t, c.End, timecode.MaxTime, f)
				// SubRip, LRC and EBU STL clamp at the largest time they hold: 999:59:59.999, 999:59.99 and 255 hours.
				// Cavena 890 caps a frame number at FFFFFFh, which is 186 hours at 25 fps.
				// PAC clamps at 654 hours, because its time code holds hours * 100 + minutes in 2 bytes.
				// YouTube and OGM chapters have no end time, so the last chapter ends at its own start.
				if f != stbox.SubRip && f != stbox.LRC && f != stbox.EBUSTL && f != stbox.PAC && f != stbox.Cavena890 && f != stbox.YouTubeChapters && f != stbox.OGMChapters {
					assert.GreaterOrEqual(t, c.End, timecode.MaxValidTime-time.Second, f)
				}
			}
			assert.Positive(t, written)
		})
	}
}

// Ports testAegisubFileKeepsMetadataCommentsAndFormatData of tests/Parsers/JsonRealFilesTest.php of
// subtitle-toolbox-php. Both reads detect the format.
func TestAegisubJSONEqualsTheASSFile(t *testing.T) {
	t.Parallel()

	fromJSON, info, err := parseBytes(testfiles.Read(t, "json/real/own_aegisub.json"), stbox.ReadOptions{})
	require.NoError(t, err)
	require.Equal(t, stbox.JSON, info.Format)
	original, info, err := parseBytes(testfiles.Read(t, "ass/real/own_aegisub.ass"), stbox.ReadOptions{})
	require.NoError(t, err)
	require.Equal(t, stbox.ASS, info.Format)

	library := func(s *stbox.Subtitle) string {
		var out bytes.Buffer
		_, err := stbox.Write(&out, s, stbox.JSON, stbox.WriteOptions{JSON: stboxjson.WriteOptions{Version: 1}})
		require.NoError(t, err)
		return out.String()
	}
	assert.Equal(t, library(original), library(fromJSON))
	title, _ := fromJSON.Metadata(stbox.MetadataTitle)
	assert.Equal(t, "Morning train to the coast", title)
	assert.Len(t, fromJSON.Comments(), 3)
	d, _ := fromJSON.FormatData("ass")
	section, _ := d.String("stylesSection")
	assert.Equal(t, "V4+ Styles", section)
}

// TestLibraryJSONVersion2KeepsTheNativeBytes sends each fixture with words or a speaker through library JSON version 2
// and back to its own format. The bytes equal the direct write of the fixture.
func TestLibraryJSONVersion2KeepsTheNativeBytes(t *testing.T) {
	t.Parallel()

	root := testfiles.Path(t, ".")
	var paths []string
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case d.IsDir() && d.Name() == "expected":
			return filepath.SkipDir
		case d.Type().IsRegular() && !strings.Contains(d.Name(), ".out."):
			paths = append(paths, path)
		}
		return nil
	}))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		require.NoError(t, err)
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			t.Parallel()

			s, info, err := parseBytes(testfiles.Read(t, filepath.ToSlash(rel)), stbox.ReadOptions{})
			if err != nil || info.Format == stbox.JSON || !hasWordsOrSpeaker(s) {
				t.Skip("no words or speaker")
			}
			var direct bytes.Buffer
			if _, err := stbox.Write(&direct, s, info.Format, stbox.WriteOptions{}); err != nil {
				t.Skip("the format does not write the file")
			}
			var library bytes.Buffer
			_, err = stbox.Write(&library, s, stbox.JSON, stbox.WriteOptions{JSON: stboxjson.WriteOptions{Version: 2}})
			require.NoError(t, err)
			back, _, err := parseBytes(library.Bytes(), stbox.ReadOptions{})
			require.NoError(t, err)
			var viaJSON bytes.Buffer
			_, err = stbox.Write(&viaJSON, back, info.Format, stbox.WriteOptions{})
			require.NoError(t, err)
			assert.Equal(t, direct.String(), viaJSON.String())
		})
	}
}

func hasWordsOrSpeaker(s *stbox.Subtitle) bool {
	return slices.ContainsFunc(s.Cues(), func(c stbox.Cue) bool { return len(c.Words) > 0 || c.Speaker != "" })
}

// A cue read from version 2 writes the same version 1 bytes as the cue before: the tags keep their place.
func TestLibraryJSONVersion2KeepsTheVersion1Bytes(t *testing.T) {
	t.Parallel()

	s, _, err := parseBytes(testfiles.Read(t, "json/real/own_aegisub.json"), stbox.ReadOptions{})
	require.NoError(t, err)
	write := func(s *stbox.Subtitle, version int) []byte {
		var out bytes.Buffer
		_, err := stbox.Write(&out, s, stbox.JSON, stbox.WriteOptions{JSON: stboxjson.WriteOptions{Version: version}})
		require.NoError(t, err)
		return out.Bytes()
	}
	back, _, err := parseBytes(write(s, 2), stbox.ReadOptions{})
	require.NoError(t, err)
	assert.Equal(t, string(write(s, 1)), string(write(back, 1)))
}
