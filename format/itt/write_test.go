package itt_test

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/itt"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

var (
	rate23976 = model.FrameRate{Num: 2997, Den: 125}
	rate2997  = model.FrameRate{Num: 2997, Den: 100}
)

const noRateError = "stbox: invalid argument: WriteOptions.ITT.FrameRate: iTT output needs the frame rate of the video."

// cue is a cue with its times in seconds.
type cue struct {
	start, end float64
	text       string
}

func seconds(s float64) time.Duration {
	return time.Duration(s * float64(time.Second)).Round(time.Millisecond)
}

// subtitle returns a subtitle with the cues and the itt format data of stored, a list of key and value pairs.
func subtitle(t *testing.T, stored []string, cues ...cue) *model.Subtitle {
	t.Helper()

	s := &model.Subtitle{}
	if stored != nil {
		var fields []model.Field
		for pair := range slices.Chunk(stored, 2) {
			fields = append(fields, model.Field{Key: pair[0], Value: pair[1]})
		}
		require.NoError(t, s.SetFormatData("itt", model.NewData(fields...)))
	}
	for _, c := range cues {
		text := c.text
		if text == "" {
			text = "Hello"
		}
		s.AddCue(model.Cue{Start: seconds(c.start), End: seconds(c.end), Lines: []string{text}})
	}
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts itt.WriteOptions) (string, error) {
	t.Helper()

	var out bytes.Buffer
	_, err := itt.Write(&out, s, common, opts)
	return out.String(), err
}

func TestWriteOptions(t *testing.T) {
	t.Parallel()

	const bad = "stbox: invalid argument: WriteOptions.ITT.FrameRate: The ITT formatter accepts the frame rates 23.976, 24, 25, 29.97 and 30, got "
	root := func(frameRate, multiplier string) string {
		return `ttp:timeBase="smpte" ttp:frameRate="` + frameRate + `" ttp:frameRateMultiplier="` + multiplier + `" ttp:dropMode="nonDrop"`
	}
	tests := []struct {
		name    string
		stored  []string
		rate    model.FrameRate
		cues    []cue
		common  model.CommonWriteOptions
		want    []string
		wantErr string
	}{
		{name: "round down to a frame", rate: model.FrameRate25, cues: []cue{{1.019, 2, ""}}, want: []string{`begin="00:00:01:00" end="00:00:02:00"`}},
		{name: "round up to a frame", rate: model.FrameRate25, cues: []cue{{1.021, 2, ""}}, want: []string{`begin="00:00:01:01" end="00:00:02:00"`}},
		{name: "round into the next second", rate: model.FrameRate25, cues: []cue{{1.99, 2.48, ""}}, want: []string{`begin="00:00:02:00" end="00:00:02:12"`}},
		{name: "a cue lasts one frame", rate: model.FrameRate25, cues: []cue{{1, 1.01, ""}}, want: []string{`begin="00:00:01:00" end="00:00:01:01"`}},
		{name: "an end before the start", rate: model.FrameRate25, cues: []cue{{1, 0.5, ""}}, want: []string{`begin="00:00:01:00" end="00:00:01:01"`}},
		{name: "a negative start", rate: model.FrameRate25, cues: []cue{{-1, 0.04, ""}}, want: []string{`begin="00:00:00:00" end="00:00:00:01"`}},
		{name: "23.976 fps", rate: rate23976, cues: []cue{{0.958, 1, ""}}, want: []string{`begin="00:00:00:23" end="00:00:01:00"`}},
		{name: "29.97 fps", rate: rate2997, cues: []cue{{3725.5, 3725.6, ""}}, want: []string{`begin="01:02:01:23" end="01:02:01:26"`}},
		{name: "24000/1001 fps", rate: model.FrameRate23976, cues: []cue{{0.5, 1, ""}}, want: []string{`begin="00:00:00:12" end="00:00:01:00"`}},
		{name: "23.976 row", rate: model.FrameRate23976, want: []string{root("24", "999 1000"), "<div/>"}},
		{name: "24 row", rate: model.FrameRate24, want: []string{root("24", "1 1"), "<div/>"}},
		{name: "25 row", rate: model.FrameRate25, want: []string{root("25", "1 1"), "<div/>"}},
		{name: "29.97 row", rate: model.FrameRate2997, want: []string{root("30", "999 1000"), "<div/>"}},
		{name: "30 row", rate: model.FrameRate30, want: []string{root("30", "1 1"), "<div/>"}},
		{name: "29.97 drop-frame writes nonDrop", rate: model.FrameRate2997DF, cues: []cue{{60, 61, ""}}, want: []string{root("30", "999 1000"), `begin="00:00:59:28"`}},
		{name: "25 drop-frame writes the 25 row", rate: model.FrameRate{Num: 25, Den: 1, DropFrame: true}, want: []string{root("25", "1 1")}},
		{
			name: "the stored rate", stored: []string{"frameRate", "25"}, cues: []cue{{1, 2, ""}},
			want: []string{root("25", "1 1"), `begin="00:00:01:00" end="00:00:02:00"`},
		},
		{
			name: "the option wins over the stored rate", stored: []string{"frameRate", "25"}, rate: model.FrameRate24,
			want: []string{root("24", "1 1")},
		},
		{
			name: "the option equal to the stored rate keeps the stored multiplier", rate: model.FrameRate23976,
			stored: []string{"frameRate", "24", "frameRateMultiplier", "1000 1001"}, want: []string{root("24", "1000 1001")},
		},
		{
			name: "an unsupported stored rate falls back to the option", rate: model.FrameRate25,
			stored: []string{"frameRate", "50", "frameRateMultiplier", "1 1"}, want: []string{root("25", "1 1")},
		},
		{
			name: "a stored multiplier counts as the TTML reader counts it", cues: []cue{{1, 2, ""}},
			stored: []string{"frameRate", "24", "frameRateMultiplier", "1000x 1001"},
			want:   []string{root("24", "1000x 1001"), `begin="00:00:01:00" end="00:00:02:00"`},
		},
		{name: "no rate", cues: []cue{{1, 2, ""}}, wantErr: noRateError},
		{name: "an unsupported stored rate and no option", stored: []string{"frameRate", "50"}, wantErr: noRateError},
		{name: "a stored rate that is not a number", stored: []string{"frameRate", "fast"}, wantErr: noRateError},
		{name: "a rate outside the table", rate: model.FrameRate50, wantErr: bad + "50."},
		{name: "a rate that is not valid", rate: model.FrameRate{Num: 25}, wantErr: bad + "0."},
		{name: "strip tags", rate: model.FrameRate25, cues: []cue{{1, 2, "<b>Hi</b> <i>you</i>"}}, common: model.CommonWriteOptions{StripTags: true}, want: []string{">Hi you</p>"}},
		{name: "keep tags", rate: model.FrameRate25, cues: []cue{{1, 2, "<b>Hi</b>"}}, want: []string{`<span tts:fontWeight="bold">Hi</span>`}},
		{name: "CRLF", rate: model.FrameRate25, cues: []cue{{1, 2, ""}}, common: model.CommonWriteOptions{LineEnding: model.CRLF}, want: []string{"?>\r\n<tt ", "</tt>\r\n"}},
		{name: "BOM", rate: model.FrameRate25, common: model.CommonWriteOptions{BOM: model.BOMAdd}, want: []string{"\ufeff<?xml"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := write(t, subtitle(t, tt.stored, tt.cues...), tt.common, itt.WriteOptions{FrameRate: tt.rate})
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Empty(t, out)
				var perr *model.InvalidArgumentError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, "WriteOptions.ITT.FrameRate", perr.Option)
				return
			}
			require.NoError(t, err)
			for _, want := range tt.want {
				assert.Contains(t, out, want)
			}
			assert.Equal(t, tt.common.BOM == model.BOMAdd, strings.HasPrefix(out, "\ufeff"))
		})
	}
}

const headDocument = `<head>
    <styling>
      <style xml:id="normal" tts:fontFamily="sansSerif" tts:fontWeight="normal" tts:fontStyle="normal" tts:color="white" tts:fontSize="100%"/>
    </styling>
    <layout>
      <region xml:id="top" tts:origin="0% 0%" tts:extent="100% 15%" tts:textAlign="center" tts:displayAlign="before"/>
      <region xml:id="bottom" tts:origin="0% 85%" tts:extent="100% 15%" tts:textAlign="center" tts:displayAlign="after"/>
    </layout>
  </head>`

func TestWriteDocument(t *testing.T) {
	t.Parallel()

	input := `<?xml version="1.0" encoding="UTF-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:tts="http://www.w3.org/ns/ttml#styling" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" xml:lang="en"` +
		` ttp:timeBase="smpte" ttp:frameRate="24" ttp:frameRateMultiplier="999 1000" ttp:dropMode="nonDrop">
<body><div><p begin="00:00:01:12" end="00:00:04:00">Hello<br/><span tts:fontStyle="italic">world</span></p></div></body></tt>`
	example := `<?xml version="1.0" encoding="UTF-8"?>
<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttm="http://www.w3.org/ns/ttml#metadata" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" xmlns:tts="http://www.w3.org/ns/ttml#styling" xml:lang="en" ttp:timeBase="smpte" ttp:frameRate="24" ttp:frameRateMultiplier="999 1000" ttp:dropMode="nonDrop">
  ` + headDocument + `
  <body style="normal">
    <div>
      <p begin="00:00:01:12" end="00:00:04:00" region="bottom">Hello<br/><span tts:fontStyle="italic">world</span></p>
    </div>
  </body>
</tt>
`
	styled := func(t *testing.T) *model.Subtitle {
		t.Helper()

		s := subtitle(t, []string{"frameRate", "25"})
		s.AddCue(model.Cue{
			Start: time.Second, End: 2 * time.Second, Position: model.Position{Alignment: model.TopCenter},
			Lines: []string{`<s>gone</s> <v Ann>said</v> <c.yellow>yellow</c> <font color='#FF0000' size="5">red</font> <font face="x">plain</font> <lang en>kept</lang>`},
		})
		s.AddCue(model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Position: model.Position{Alignment: model.MiddleRight}, Lines: []string{"middle"}})
		return s
	}
	tests := []struct {
		name     string
		sub      func(t *testing.T) *model.Subtitle
		exact    string
		want     []string
		notWant  []string
		wantFail bool
	}{
		{
			name: "the example document", exact: example,
			sub: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s, _, err := itt.Read(strings.NewReader(input), model.CommonReadOptions{}, itt.ReadOptions{})
				require.NoError(t, err)
				return s
			},
		},
		{
			name: "the title goes first in the head",
			sub: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s := subtitle(t, []string{"frameRate", "25"}, cue{1, 2, ""})
				s.SetMetadata(model.MetadataTitle, "Bakery")
				return s
			},
			want: []string{"<head>\n    <ttm:title>Bakery</ttm:title>\n    <styling>"},
		},
		{
			name: "top and bottom regions and the reduced tag set",
			sub:  styled,
			want: []string{
				`region="top"`, `region="bottom"`, `<span tts:color="#ffff00">yellow</span>`, `<span tts:color="#ff0000">red</span>`,
				"gone said", "plain kept",
			},
			notWant: []string{"<s>", "<v", "Ann>", "<lang", "tts:fontSize=\"5\"", "middle</p>\n      <p"},
		},
		{
			name: "a forced cue declares itts",
			sub: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s := subtitle(t, []string{"frameRate", "25"}, cue{1, 2, "sign"}, cue{3, 4, "talk"})
				c, _ := s.Cue(0)
				c.Forced = true
				require.NoError(t, s.SetCue(0, c))
				return s
			},
			want: []string{`xmlns:itts="http://www.w3.org/ns/ttml/profile/imsc1#styling"`, `region="bottom" itts:forcedDisplay="true">sign</p>`},
		},
		{
			name: "no forced cue declares no itts",
			sub: func(t *testing.T) *model.Subtitle {
				t.Helper()
				return subtitle(t, []string{"frameRate", "25"}, cue{1, 2, ""})
			},
			notWant: []string{"itts"},
		},
		{
			name: "a cue ID equal to a head ID gets a suffix",
			sub: func(t *testing.T) *model.Subtitle {
				t.Helper()
				s := subtitle(t, []string{"frameRate", "25"}, cue{1, 2, ""})
				c, _ := s.Cue(0)
				c.ID = "top"
				require.NoError(t, s.SetCue(0, c))
				return s
			},
			want: []string{`xml:id="top_2"`, `<region xml:id="top" `},
		},
		{
			name: "text that is not UTF-8", wantFail: true,
			sub: func(t *testing.T) *model.Subtitle {
				t.Helper()
				return subtitle(t, []string{"frameRate", "25"}, cue{1, 2, "a\xffb"})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, err := write(t, tt.sub(t), model.CommonWriteOptions{}, itt.WriteOptions{})
			if tt.wantFail {
				var uerr *model.UnwritableError
				require.ErrorAs(t, err, &uerr)
				assert.Equal(t, model.ITT, uerr.Format)
				assert.Empty(t, out)
				return
			}
			require.NoError(t, err)
			if tt.exact != "" {
				assert.Equal(t, tt.exact, out)
			}
			for _, want := range tt.want {
				assert.Contains(t, out, want)
			}
			for _, notWant := range tt.notWant {
				assert.NotContains(t, out, notWant)
			}
		})
	}
}

// frame is the longest frame of the 5 iTT rates, rounded up.
const frame = 42 * time.Millisecond

var timesRegexp = regexp.MustCompile(`<p begin="([^"]+)" end="([^"]+)"`)

// TestRoundTrip reads a file, writes it at its own rate, and reads the output again. The cue times and text stay.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	hours := subtitle(t, nil, cue{3600, 3700, "one hour"}, cue{7200, 7300, "two hours"})
	tests := []struct {
		name string
		file string
		sub  *model.Subtitle
		rate model.FrameRate
	}{
		{name: "avscript", file: "itt/real/avscript_testing.itt"},
		{name: "Final Cut Pro", file: "itt/real/fcp_23976_styles.itt"},
		{name: "29.97 fps with a BOM", file: "itt/real/bom_2997_crlf.itt"},
		{name: "forced cues", file: "forced/forced_signs_2398.itt"},
		{name: "2 hours at 29.97 fps", sub: hours, rate: model.FrameRate2997},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := tt.sub
			if tt.file != "" {
				s = readFile(t, tt.file)
			}
			out, err := write(t, s, model.CommonWriteOptions{}, itt.WriteOptions{FrameRate: tt.rate})
			require.NoError(t, err)
			again, _, err := itt.Read(strings.NewReader(out), model.CommonReadOptions{}, itt.ReadOptions{})
			require.NoError(t, err)
			require.Equal(t, s.Len(), again.Len())
			for i, c := range s.All() {
				got, _ := again.Cue(i)
				assert.InDelta(t, float64(c.Start), float64(got.Start), float64(frame), "start of cue %d", i)
				assert.InDelta(t, float64(max(c.End, c.Start)), float64(got.End), 2*float64(frame), "end of cue %d", i)
				assert.Equal(t, c.Text(), got.Text(), "text of cue %d", i)
			}
		})
	}
	hoursOut, err := write(t, hours, model.CommonWriteOptions{}, itt.WriteOptions{FrameRate: model.FrameRate2997})
	require.NoError(t, err)
	labels := make([]string, 0, 4)
	for _, m := range timesRegexp.FindAllStringSubmatch(hoursOut, -1) {
		labels = append(labels, m[1], m[2])
	}
	assert.Equal(t, []string{"00:59:56:12", "01:01:36:09", "01:59:52:24", "02:01:32:21"}, labels)
}
