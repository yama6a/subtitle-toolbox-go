package dcp_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/dcp"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func newFile() *model.Subtitle {
	s := subtitle(
		model.Cue{Start: 5 * time.Second, End: 7125 * ms, Lines: []string{"The ferry leaves at nine,", "from the <i>north</i> pier."}},
		model.Cue{Start: 8500 * ms, End: 10 * time.Second, Lines: []string{"HARBOUR"}, Position: model.Position{Alignment: model.TopLeft}},
	)
	s.SetMetadata(model.MetadataTitle, "Harbour")
	_ = s.SetFormatData("dcp", model.NewData(model.Field{Key: "header", Value: model.NewData(model.Field{Key: "IssueDate", Value: "2026-10-12T00:00:00"})}))
	return s
}

func TestWriteNewFile(t *testing.T) {
	t.Parallel()

	out := write(t, newFile())

	assert.Contains(t, out, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n"+
		"<SubtitleReel xmlns=\"http://www.smpte-ra.org/schemas/428-7/2014/DCST\" xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n"+
		"  <Id>urn:uuid:7a3a33dc-cf39-3857-989e-c26afbf55e1b</Id>\n  <ContentTitleText>Harbour</ContentTitleText>\n  <IssueDate>2026-10-12T00:00:00</IssueDate>\n"+
		"  <ReelNumber>1</ReelNumber>\n  <Language>en</Language>\n  <EditRate>24 1</EditRate>\n  <TimeCodeRate>24</TimeCodeRate>\n  <StartTime>00:00:00:00</StartTime>\n"+
		"  <LoadFont ID=\"font\">urn:uuid:1197e585-039f-3bd0-b658-655a4645653e</LoadFont>\n")
	assert.Contains(t, out, "    <Font AspectAdjust=\"1.0\" Color=\"FFFFFFFF\" Effect=\"none\" EffectColor=\"FF000000\" ID=\"font\" Italic=\"no\" Script=\"normal\" Size=\"42\" Underline=\"no\" Weight=\"normal\">\n")
	assert.Contains(t, out,
		"      <Subtitle SpotNumber=\"1\" TimeIn=\"00:00:05:00\" TimeOut=\"00:00:07:03\" FadeUpTime=\"00:00:00:00\" FadeDownTime=\"00:00:00:00\">\n"+
			"        <Text Valign=\"bottom\" Vposition=\"15\">The ferry leaves at nine,</Text>\n"+
			"        <Text Valign=\"bottom\" Vposition=\"8\">from the <Font Italic=\"yes\">north</Font> pier.</Text>\n"+
			"      </Subtitle>\n")
	assert.Contains(t, out, "<Text Halign=\"left\" Valign=\"top\" Vposition=\"8\">HARBOUR</Text>")
	assert.Equal(t, out, write(t, newFile()))
}

func TestWrite(t *testing.T) {
	t.Parallel()

	moved := func(file string, edit func(c *model.Cue)) *model.Subtitle {
		s := readFile(t, file)
		c, _ := s.Cue(0)
		edit(&c)
		require.NoError(t, s.SetCue(0, c))
		return s
	}
	interop := func(subtitles string) *model.Subtitle { return mustRead(t, interopReel(subtitles)) }

	tests := []struct {
		name string
		s    *model.Subtitle
		want []string
	}{
		{
			name: "Interop keeps its ticks, header and stored attributes",
			s:    moved("monal_interop_shape.xml", func(c *model.Cue) { c.End = 52600 * ms }),
			want: []string{
				"<DCSubtitle Version=\"1.0\">\n  <SubtitleID>",
				`TimeIn="00:00:51:000" TimeOut="00:00:52:150"`,
				`<Text HAlign="center" HPosition="0.0" VAlign="bottom" VPosition="14"><Font Italic="yes">Every night at dusk</Font></Text>`,
			},
		},
		{
			name: "a changed alignment writes positions from the alignment, middle lines around 0",
			s:    moved("dcpomatic_interop_shape.xml", func(c *model.Cue) { c.Position.Alignment = model.MiddleCenter }),
			want: []string{`<Text VAlign="center" VPosition="-3.5">The ferry leaves at nine,</Text>` + "\n      " + `<Text VAlign="center" VPosition="3.5">from the north pier.</Text>`},
		},
		{
			name: "a top row counts down from the top, a right column writes HAlign",
			s:    moved("dcpomatic_interop_shape.xml", func(c *model.Cue) { c.Position.Alignment = model.TopRight }),
			want: []string{`<Text HAlign="right" VAlign="top" VPosition="8">The ferry leaves at nine,</Text>` + "\n      " + `<Text HAlign="right" VAlign="top" VPosition="15">from the north pier.</Text>`},
		},
		{
			name: "an offset moves the anchor edge of the row, and the other lines stay on the screen",
			s: moved("dcpomatic_interop_shape.xml", func(c *model.Cue) {
				c.Position = model.Position{Alignment: model.TopCenter, Offset: &model.Offset{X: 50, Y: 20}}
			}),
			want: []string{`<Text VAlign="top" VPosition="20">The ferry leaves at nine,</Text>` + "\n      " + `<Text VAlign="top" VPosition="27">from the north pier.</Text>`},
		},
		{
			name: "a bottom edge offset writes the bottom line at the edge, an X offset writes HPosition",
			s: moved("dcpomatic_interop_shape.xml", func(c *model.Cue) {
				c.Position = model.Position{Alignment: model.BottomRight, Offset: &model.Offset{X: 90, Y: 100}}
			}),
			want: []string{`<Text HAlign="right" HPosition="10" VAlign="bottom" VPosition="7">The ferry leaves at nine,</Text>` + "\n      " + `<Text HAlign="right" HPosition="10" VAlign="bottom" VPosition="0">from the north pier.</Text>`},
		},
		{
			name: "white text in a colored Font writes its color, so it stays white",
			s: func() *model.Subtitle {
				s := mustRead(t, interopReel(`<Font Color="FFEBEBEB"><Subtitle TimeIn="00:00:01:000" TimeOut="00:00:02:000"><Text VAlign="bottom" VPosition="8">a <Font Color="FFFFFFFF">white</Font></Text></Subtitle></Font>`))
				return s
			}(),
			want: []string{`<Text VAlign="bottom" VPosition="8">a<Font Color="FFFFFFFF"> white</Font></Text>`},
		},
		{
			name: "an offset outside the cell of the alignment is ignored",
			s: moved("dcpomatic_interop_shape.xml", func(c *model.Cue) {
				c.Position = model.Position{Alignment: model.TopCenter, Offset: &model.Offset{X: 50, Y: 90}}
			}),
			want: []string{`<Text VAlign="top" VPosition="8">The ferry leaves at nine,</Text>`},
		},
		{
			name: "the stored position wins over an offset while the alignment stays",
			s: moved("dcpomatic_interop_shape.xml", func(c *model.Cue) {
				c.Position.Offset = &model.Offset{X: 50, Y: 70}
			}),
			want: []string{`<Text VAlign="bottom" VPosition="15">The ferry leaves at nine,</Text>`},
		},
		{
			name: "runs write only the style that differs from the Font around the subtitles",
			s: func() *model.Subtitle {
				s := mustRead(t, interopReel(`<Font Italic="yes" Color="FFEBEBEB"><Subtitle TimeIn="00:00:01:000" TimeOut="00:00:02:000"><Text VAlign="bottom" VPosition="8">Sung</Text></Subtitle></Font>`))
				c, _ := s.Cue(0)
				c.Lines = []string{`<font color="#ebebeb"><i>Sung</i> spoken</font>`}
				require.NoError(t, s.SetCue(0, c))
				return s
			}(),
			want: []string{`<Text VAlign="bottom" VPosition="8">Sung<Font Italic="no"> spoken</Font></Text>`},
		},
		{
			name: "runs write color, italic, weight and underline in this order, and escape the text",
			s: moved("dcpomatic_interop_shape.xml", func(c *model.Cue) {
				c.Lines = []string{`<font color="#ffff00"><b><i><u>A &amp; &lt;B&gt; "C"</u></i></b></font>`}
			}),
			want: []string{`<Font Color="FFFFFF00" Italic="yes" Weight="bold" Underlined="yes">A &amp; &lt;B&gt; &quot;C&quot;</Font>`},
		},
		{
			name: "a stored subtitle without fade times gets them",
			s:    interop(`<Subtitle SpotNumber="9" TimeIn="00:00:01:000" TimeOut="00:00:02:000"><Text VAlign="bottom" VPosition="8">x</Text></Subtitle>`),
			want: []string{`<Subtitle SpotNumber="1" TimeIn="00:00:01:000" TimeOut="00:00:02:000" FadeUpTime="20" FadeDownTime="20">`},
		},
		{
			name: "a time just below the limit stops at the last unit, so the file reads back",
			s:    subtitle(model.Cue{Start: timecode.MaxValidTime - time.Second, End: timecode.MaxValidTime, Lines: []string{"x"}}),
			want: []string{`TimeIn="99999:59:59:00" TimeOut="99999:59:59:23"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := write(t, tt.s)
			for _, want := range tt.want {
				assert.Contains(t, out, want)
			}
		})
	}
}

func TestWriteMetadataAndHeader(t *testing.T) {
	t.Parallel()

	smpte := readFile(t, "subtitle_edit_smpte2014_shape.xml")
	smpte.SetMetadata(model.MetadataTitle, "Le Phare 2")
	smpte.SetMetadata(model.MetadataLanguage, "de")
	out := write(t, smpte)
	assert.Contains(t, out, "<dcst:ContentTitleText>Le Phare 2</dcst:ContentTitleText>")
	assert.Contains(t, out, "<dcst:Language>de</dcst:Language>")

	interop := mustRead(t, interopReel(`<ReelNumber>1</ReelNumber><Subtitle TimeIn="00:00:01:000" TimeOut="00:00:02:000"><Text VAlign="bottom" VPosition="8">x</Text></Subtitle>`))
	interop.SetMetadata(model.MetadataTitle, "Not stored")
	interop.SetMetadata(model.MetadataLanguage, "de")
	out = write(t, interop)
	assert.NotContains(t, out, "Not stored", "Interop writes MovieTitle only when the header has it")
	assert.NotContains(t, out, "<Language>")
}

func TestRoundTripKeepsWhatWasWritten(t *testing.T) {
	t.Parallel()

	for name, s := range map[string]*model.Subtitle{
		"new file": subtitle(
			model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<b>Bold</b> and <u>under</u>", `<font color="#ffff00">yellow</font>`}, Position: model.Position{Alignment: model.MiddleRight}},
			model.Cue{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"top"}, Position: model.Position{Alignment: model.TopLeft, Offset: &model.Offset{X: 10, Y: 12.5}}},
		),
		"Interop":  readFile(t, "monal_interop_shape.xml"),
		"prefixed": readFile(t, "subtitle_edit_smpte2014_shape.xml"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			want, got := cueRows(s), cueRows(mustRead(t, write(t, s)))
			for i := range want {
				want[i].Offset, got[i].Offset = nil, nil
			}
			assert.Equal(t, want, got)
		})
	}
}

func TestWriteErrors(t *testing.T) {
	t.Parallel()

	bad := newFile()
	c, _ := bad.Cue(1)
	c.Lines = []string{"bad \xff"}
	require.NoError(t, bad.SetCue(1, c))
	var out bytes.Buffer
	_, err := dcp.Write(&out, bad, model.CommonWriteOptions{}, dcp.WriteOptions{})
	var uerr *model.UnwritableError
	require.ErrorAs(t, err, &uerr)
	assert.Equal(t, 1, uerr.CueIndex)
	assert.Equal(t, "The output cannot hold text that is not valid UTF-8. Read a file in another encoding with its source encoding.", uerr.Reason)
	require.ErrorIs(t, err, model.ErrInvalidUTF8)

	broken := newFile()
	require.NoError(t, broken.SetFormatData("dcp", model.NewData(model.Field{Key: "header", Value: model.NewData(model.Field{Key: "Bad Name", Value: "x"})})))
	_, err = dcp.Write(&out, broken, model.CommonWriteOptions{}, dcp.WriteOptions{})
	require.ErrorAs(t, err, &uerr)
	assert.Equal(t, "The formatter wrote XML that is not well-formed.", uerr.Reason)
}

func TestAlignedCueWithoutOffsetReadsBackWithoutOffset(t *testing.T) {
	t.Parallel()

	for _, a := range []model.Alignment{model.TopLeft, model.BottomRight, model.MiddleLeft, model.TopCenter} {
		s := subtitle(model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"x"}, Position: model.Position{Alignment: a}})
		got := cueRows(mustRead(t, write(t, s)))
		assert.Equal(t, []cueRow{{Start: time.Second, End: 2 * time.Second, Lines: []string{"x"}, Alignment: a}}, got, a.String())
	}
}
