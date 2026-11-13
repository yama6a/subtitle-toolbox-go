package dcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	time1 = 1000 * ms
	time2 = 2000 * ms
)

type warningRow struct {
	Line    int
	Action  model.WarningAction
	Message string
}

func TestRead(t *testing.T) {
	t.Parallel()

	const badImage = `<Subtitle TimeIn="00:00:01:00" TimeOut="00:00:02:00"><Image Valign="bottom">urn:uuid:1</Image></Subtitle>`
	tests := []struct {
		name         string
		content      string
		lenient      bool
		want         []cueRow
		wantWarnings []warningRow
	}{
		{
			name:    "SMPTE at the EditRate when TimeCodeRate is missing",
			content: `<SubtitleReel xmlns="x"><EditRate>48 1</EditRate><SubtitleList><Subtitle TimeIn="00:00:01:24" TimeOut="00:00:02:00"><Text Valign="center">Rain</Text></Subtitle></SubtitleList></SubtitleReel>`,
			want:    []cueRow{{Start: 1500 * ms, End: 2000 * ms, Lines: []string{"Rain"}, Alignment: model.MiddleCenter}},
		},
		{
			name:    "a rate that is no whole number from 1 to 1000 falls back to 24",
			content: `<SubtitleReel xmlns="x"><TimeCodeRate>5000</TimeCodeRate><EditRate>1 3</EditRate><SubtitleList><Subtitle TimeIn="00:00:00:23" TimeOut="00:00:01:00"><Text Valign="bottom" Vposition="8">x</Text></Subtitle></SubtitleList></SubtitleReel>`,
			want:    []cueRow{{Start: 958 * ms, End: 1000 * ms, Lines: []string{"x"}, Alignment: model.BottomCenter}},
		},
		{
			name:    "StartTime is not subtracted",
			content: `<SubtitleReel xmlns="x"><TimeCodeRate>25</TimeCodeRate><StartTime>10:00:00:00</StartTime><SubtitleList><Subtitle TimeIn="00:00:13:00" TimeOut="00:00:15:00"><Text Valign="bottom" Vposition="8">Wind</Text></Subtitle></SubtitleList></SubtitleReel>`,
			want:    []cueRow{{Start: 13000 * ms, End: 15000 * ms, Lines: []string{"Wind"}, Alignment: model.BottomCenter}},
		},
		{
			name:    "Interop decimal times",
			content: interopReel(`<Subtitle TimeIn="01:12:42.5" TimeOut="01:12:44.25"><Text VAlign="bottom" VPosition="8">Rain</Text></Subtitle>`),
			want:    []cueRow{{Start: 4362500 * ms, End: 4364250 * ms, Lines: []string{"Rain"}, Alignment: model.BottomCenter}},
		},
		{
			name: "nested fonts pass their styles on, and the color is the alpha-free value",
			content: interopReel(`<Font Italic="YES"><Subtitle TimeIn="00:00:01:000" TimeOut="00:00:02:000"><Font Weight="bold" Color="FFFFFF00">` +
				`<Text VAlign="bottom" VPosition="8">Hi<Font Underlined="yes" Italic="no"> you</Font></Text></Font></Subtitle></Font>`),
			want: []cueRow{{
				Start: time1, End: time2, Alignment: model.BottomCenter,
				Lines: []string{`<font color="#ffff00"><b><i>Hi</i> <u>you</u></b></font>`},
			}},
		},
		{
			name: "lines run from the top, Space and Ruby make text, empty lines drop",
			content: smpteReel(`<Subtitle TimeIn="00:00:01:00" TimeOut="00:00:02:00">` +
				`<Text Valign="bottom" Vposition="8">low<Space/>line</Text><Text Valign="bottom" Vposition="15">A <Ruby><Rb>base</Rb><Rt>note</Rt></Ruby>` + "\n\t" + `</Text>` +
				`<Text Valign="top"> </Text></Subtitle>`),
			want: []cueRow{{
				Start: time1, End: time2, Alignment: model.BottomCenter,
				Lines: []string{"A base (note)", "low line"},
			}},
		},
		{
			name: "offsets from the anchor edge of the row and HPosition, none in the usual place",
			content: interopReel(`<Subtitle TimeIn="00:00:01:000" TimeOut="00:00:02:000"><Text HAlign="left" VAlign="top" VPosition="10">a</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:03:000" TimeOut="00:00:04:000"><Text HAlign="right" VAlign="center" VPosition="10">b</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:05:000" TimeOut="00:00:06:000"><Text VAlign="center" VPosition="-5">c</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:07:000" TimeOut="00:00:08:000"><Text VAlign="bottom" VPosition="-40">d</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:09:000" TimeOut="00:00:10:000"><Text VAlign="center" VPosition="x">e</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:11:000" TimeOut="00:00:12:000"><Text HAlign="left" HPosition="10" VAlign="top" VPosition="8">f</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:13:000" TimeOut="00:00:14:000"><Text HAlign="left" HPosition="20" VAlign="bottom" VPosition="30">g</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:15:000" TimeOut="00:00:16:000"><Text VAlign="bottom" VPosition="25">h2</Text><Text VAlign="bottom" VPosition="32">h1</Text></Subtitle>`),
			want: []cueRow{
				{Start: 1000 * ms, End: 2000 * ms, Lines: []string{"a"}, Alignment: model.TopLeft},
				{Start: 3000 * ms, End: 4000 * ms, Lines: []string{"b"}, Alignment: model.MiddleRight, Offset: &model.Offset{X: 90, Y: 60}},
				{Start: 5000 * ms, End: 6000 * ms, Lines: []string{"c"}, Alignment: model.MiddleCenter, Offset: &model.Offset{X: 50, Y: 45}},
				{Start: 7000 * ms, End: 8000 * ms, Lines: []string{"d"}, Alignment: model.BottomCenter},
				{Start: 9000 * ms, End: 10000 * ms, Lines: []string{"e"}, Alignment: model.MiddleCenter},
				{Start: 11000 * ms, End: 12000 * ms, Lines: []string{"f"}, Alignment: model.TopLeft},
				{Start: 13000 * ms, End: 14000 * ms, Lines: []string{"g"}, Alignment: model.BottomLeft, Offset: &model.Offset{X: 20, Y: 70}},
				{Start: 15000 * ms, End: 16000 * ms, Lines: []string{"h1", "h2"}, Alignment: model.BottomCenter, Offset: &model.Offset{X: 50, Y: 75}},
			},
		},
		{
			name: "lenient mode skips an image and a bad time",
			content: smpteReel(badImage +
				`<Subtitle TimeIn="00:00:03:25" TimeOut="00:00:04:00"><Text>Late</Text></Subtitle>` +
				`<Subtitle TimeIn="00:00:05:00" TimeOut="00:00:06:00"><Text Valign="bottom" Vposition="8">Text</Text></Subtitle>`),
			lenient: true,
			want:    []cueRow{{Start: 5000 * ms, End: 6000 * ms, Lines: []string{"Text"}, Alignment: model.BottomCenter}},
			wantWarnings: []warningRow{
				{1, model.Skipped, `The subtitle at 00:00:01:00 is an image. The parser reads only text subtitles.`},
				{1, model.Skipped, `The time "00:00:03:25" is not a DCP time code at 25 units per second.`},
			},
		},
		{
			name:         "lenient mode swaps an end before the start",
			content:      smpteReel(`<Subtitle TimeIn="00:00:02:00" TimeOut="00:00:01:00"><Text Valign="bottom" Vposition="8">Back</Text></Subtitle>`),
			lenient:      true,
			want:         []cueRow{{Start: time1, End: time2, Lines: []string{"Back"}, Alignment: model.BottomCenter}},
			wantWarnings: []warningRow{{1, model.Repaired, "The cue ends at 1 s, before it starts at 2 s. The parser swapped the times."}},
		},
		{
			name:         "lenient mode skips text before the XML",
			content:      "Credits\n" + smpteReel(`<Subtitle TimeIn="00:00:01:00" TimeOut="00:00:02:00"><Text Valign="bottom" Vposition="8">x</Text></Subtitle>`),
			lenient:      true,
			want:         []cueRow{{Start: time1, End: time2, Lines: []string{"x"}, Alignment: model.BottomCenter}},
			wantWarnings: []warningRow{{1, model.Repaired, `The file has text before the XML: "Credits". The parser skipped it.`}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings, err := read(tt.content, tt.lenient)
			require.NoError(t, err)
			assert.Equal(t, tt.want, cueRows(s))
			var got []warningRow
			for _, w := range warnings {
				got = append(got, warningRow{w.Line, w.Action, w.Message})
			}
			assert.Equal(t, tt.wantWarnings, got)
		})
	}
}

func TestReadErrors(t *testing.T) {
	t.Parallel()

	good := `<Subtitle TimeIn="00:00:01:00" TimeOut="00:00:02:00"><Text Valign="bottom" Vposition="8">x</Text></Subtitle>`
	tests := []struct {
		name    string
		content string
		line    int
		msg     string
		// both is true when lenient mode fails too. Else lenient mode skips the subtitle or the text.
		both bool
	}{
		{"image subtitle", smpteReel(`<Subtitle TimeIn="00:00:01:00" TimeOut="00:00:02:00"><Image Valign="bottom">urn:uuid:1</Image></Subtitle>`), 1, "The subtitle at 00:00:01:00 is an image. The parser reads only text subtitles.", false},
		{"frame at the rate", smpteReel(`<Subtitle TimeIn="00:00:01:25" TimeOut="00:00:02:00"><Text>x</Text></Subtitle>`), 1, `The time "00:00:01:25" is not a DCP time code at 25 units per second.`, false},
		{"Interop tick at 250", interopReel(`<Subtitle TimeIn="00:00:01:250" TimeOut="00:00:02:000"><Text>x</Text></Subtitle>`), 1, `The time "00:00:01:250" is not a DCP time code at 250 units per second.`, false},
		{"decimal time in SMPTE", smpteReel(`<Subtitle TimeIn="00:00:01.5" TimeOut="00:00:02:00"><Text>x</Text></Subtitle>`), 1, `The time "00:00:01.5" is not a DCP time code at 25 units per second.`, false},
		{"time at 100000 hours", smpteReel(`<Subtitle TimeIn="100000:00:00:00" TimeOut="100000:00:01:00"><Text>x</Text></Subtitle>`), 1, `The time "100000:00:00:00" is not below 100000 hours.`, false},
		{"huge hours", smpteReel(`<Subtitle TimeIn="99999999999999999999:00:00:00" TimeOut="0:0:1:0"><Text>x</Text></Subtitle>`), 1, `The time "99999999999999999999:00:00:00" is not below 100000 hours.`, false},
		{"end before start", smpteReel(`<Subtitle TimeIn="00:01:00:00" TimeOut="00:00:01:00"><Text>x</Text></Subtitle>`), 1, "The cue ends at 1 s, before it starts at 60 s.", false},
		{"line of a later subtitle", smpteReel(good + "\n" + `<Subtitle TimeIn="x" TimeOut="y"/>`), 2, `The time "x" is not a DCP time code at 25 units per second.`, false},
		{"text before the XML", "Credits\n" + smpteReel(good), 1, `The file has text before the XML: "Credits".`, false},
		{"other root", `<tt xmlns="x"/>`, 0, "The root element must be <DCSubtitle> or <SubtitleReel>, not <tt>.", true},
		{"prefixed other root", `<d:Root xmlns:d="x"/>`, 0, "The root element must be <DCSubtitle> or <SubtitleReel>, not <d:Root>.", true},
		{"malformed XML", `<DCSubtitle><Subtitle></DCSubtitle>`, 0, "The content is not well-formed XML.", true},
		{"invalid UTF-8", "<DCSubtitle><!-- \xff --></DCSubtitle>", 0, "The content is not well-formed XML.", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			modes := []bool{false}
			if tt.both {
				modes = append(modes, true)
			}
			for _, lenient := range modes {
				_, _, err := read(tt.content, lenient)
				var perr *model.ParseError
				require.ErrorAs(t, err, &perr)
				assert.Equal(t, model.DCP, perr.Format)
				assert.Equal(t, tt.line, perr.Line)
				assert.Equal(t, tt.msg, perr.Msg)
			}
		})
	}
}
