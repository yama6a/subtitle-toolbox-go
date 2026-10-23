package stbox_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go"
	"github.com/yama6a/subtitle-toolbox-go/format/microdvd"
	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
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
	fps := stbox.WriteOptions{MicroDVD: microdvd.WriteOptions{FrameRate: 25}}
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
				require.NoError(t, stbox.Write(&out, s, w.format, w.opts))
				assert.Contains(t, out.String(), tt.written)
				for _, lenient := range []bool{false, true} {
					opts := stbox.ReadOptions{Format: w.format, Lenient: lenient, MicroDVD: microdvd.ReadOptions{FrameRate: 25}}
					back, _, err := stbox.Parse(bytes.NewReader(out.Bytes()), opts)
					require.NoError(t, err, "lenient %v", lenient)
					require.Equal(t, 3, back.Len(), "lenient %v", lenient)
					c, _ := back.Cue(1)
					assert.Equal(t, tt.want, c.Lines, "lenient %v", lenient)
				}
			})
		}
	}
}
