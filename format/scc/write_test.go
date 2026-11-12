package scc_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

func cue(start, end float64, lines ...string) model.Cue {
	return model.Cue{Start: time.Duration(start * float64(time.Second)), End: time.Duration(end * float64(time.Second)), Lines: lines}
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions, opts scc.WriteOptions) (string, scc.WriteReport) {
	t.Helper()

	var out bytes.Buffer
	report, err := scc.Write(&out, s, common, opts)
	require.NoError(t, err)
	return out.String(), report
}

func readBack(t *testing.T, content string) []model.Cue {
	t.Helper()

	s, _ := mustRead(t, content, model.CommonReadOptions{}, scc.ReadOptions{})
	return s.Cues()
}

func writeAndRead(t *testing.T, cues ...model.Cue) []model.Cue {
	t.Helper()

	out, _ := write(t, subtitle(cues...), model.CommonWriteOptions{}, scc.WriteOptions{})
	return readBack(t, out)
}

func storedInts(t *testing.T, c model.Cue, key string) []int64 {
	t.Helper()

	data, ok := c.FormatData("scc")
	require.True(t, ok)
	list, ok := data.List(key)
	require.True(t, ok)
	out := make([]int64, len(list))
	for i, v := range list {
		out[i], _ = v.(int64)
	}
	return out
}

func withData(c model.Cue, rows, columns []any) model.Cue {
	_ = c.SetFormatData("scc", model.NewData(model.Field{Key: "rows", Value: rows}, model.Field{Key: "columns", Value: columns}))
	return c
}

func atRow(c model.Cue, row int) model.Cue {
	c.Position = model.Position{Row: row, Column: 1, Grid: model.Grid{Rows: 15, Columns: 32}}
	return c
}

func topAt(c model.Cue, row int) model.Cue {
	c = atRow(c, row)
	c.Position.Alignment = model.TopCenter
	return c
}

func TestWriteLimits(t *testing.T) {
	t.Parallel()

	cjk := string(testfiles.Read(t, "scc/fit/own_long_cjk.txt"))
	han := string(rune(0x65e5))
	bakery := strings.TrimSpace(strings.Repeat("bakery ", 20))
	tests := []struct {
		name    string
		cues    []model.Cue
		fit     bool
		wantErr string
		wantIs  error
	}{
		{
			"5 lines",
			[]model.Cue{cue(1, 2, "Hi"), cue(3, 4, "1", "2", "3", "4", "5")},
			false,
			"scc: Cue #1 at 3 s has 5 lines, but SCC allows 4. Wrap the lines to 4 lines of 32 characters first.", scc.ErrWrapFirst,
		},
		{
			"33 characters in a styled line",
			[]model.Cue{cue(1, 2, "<i>"+strings.Repeat("a", 33)+"</i>")},
			false,
			"scc: Cue #0 at 1 s has a line with 33 characters, but SCC allows 32. Wrap the lines to 4 lines of 32 characters first.", scc.ErrWrapFirst,
		},
		{
			"text that wrapping cannot fit",
			[]model.Cue{cue(1, 8, bakery)},
			false,
			"scc: Cue #0 at 1 s has a line with 139 characters, but SCC allows 32. Split the cue into cues of 4 lines of 32 characters, then wrap the lines.", scc.ErrSplitFirst,
		},
		{
			"character without a code",
			[]model.Cue{cue(1, 2, han)},
			false,
			`scc: Cue #0 at 1 s has the character "` + han + `", which CEA-608 cannot show.`, nil,
		},
		{
			"fit keeps the failure for text that does not wrap",
			[]model.Cue{cue(1, 8, bakery)},
			true,
			"scc: Cue #0 at 1 s has a line with 139 characters, but SCC allows 32. Split the cue into cues of 4 lines of 32 characters, then wrap the lines.", scc.ErrSplitFirst,
		},
		{
			"fit keeps the failure for a character without replacement",
			[]model.Cue{cue(1, 2, "Š"+han)},
			true,
			`scc: Cue #0 at 1 s has the character "` + han + `", which CEA-608 cannot show.`, nil,
		},
		{
			"fit ends a long CJK cue with the split hint",
			[]model.Cue{cue(1, 8, cjk)},
			true,
			"scc: Cue #0 at 1 s has a line with 341 characters, but SCC allows 32. Split the cue into cues of 4 lines of 32 characters, then wrap the lines.", scc.ErrSplitFirst,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := scc.Write(&bytes.Buffer{}, subtitle(tt.cues...), model.CommonWriteOptions{}, scc.WriteOptions{Fit: tt.fit})
			require.ErrorIs(t, err, model.ErrUnwritable)
			var uerr *model.UnwritableError
			require.ErrorAs(t, err, &uerr)
			assert.Equal(t, tt.wantErr, err.Error())
			assert.True(t, errors.Is(err, tt.wantIs) || tt.wantIs == nil)
			if tt.wantIs == nil {
				assert.NoError(t, uerr.Err)
			}
		})
	}
}

func TestWritePositions(t *testing.T) {
	t.Parallel()

	ebu := func(a model.Alignment, row int) model.Cue {
		c := cue(2, 4, "One line")
		c.Position = model.Position{Alignment: a, Row: row, Grid: model.Grid{Rows: 23, Columns: 40}}
		return c
	}
	aligned := func(a model.Alignment) model.Cue {
		c := cue(2, 4, "Weather", "today")
		c.Position.Alignment = a
		return c
	}
	column := func(col int) model.Cue {
		c := aligned(model.BottomCenter)
		c.Position = model.Position{Alignment: model.BottomCenter, Row: 13, Column: col, Grid: model.Grid{Rows: 15, Columns: 32}}
		return c
	}
	tests := []struct {
		name        string
		cue         model.Cue
		rows        []int64
		columns     []int64
		alignment   model.Alignment
		wantNoStore bool
	}{
		{"default is bottom center", aligned(model.BottomCenter), []int64{14, 15}, []int64{12, 13}, model.BottomCenter, false},
		{"top left", aligned(model.TopLeft), []int64{1, 2}, []int64{0, 0}, model.TopCenter, false},
		{"top center", aligned(model.TopCenter), []int64{1, 2}, []int64{12, 13}, model.TopCenter, false},
		{"middle right", aligned(model.MiddleRight), []int64{7, 8}, []int64{25, 27}, model.BottomCenter, false},
		{"bottom left", aligned(model.BottomLeft), []int64{14, 15}, []int64{0, 0}, model.BottomCenter, false},
		{
			"stored rows and columns", atRow(withData(cue(2, 4, "Left", "Right"), []any{int64(10), int64(12)}, []any{int64(3), int64(27)}), 10),
			[]int64{10, 12},
			[]int64{3, 27},
			model.BottomCenter, false,
		},
		{
			"stored rows for another line count", withData(cue(2, 4, "One line"), []any{int64(10), int64(12)}, []any{int64(3), int64(27)}),
			[]int64{15},
			[]int64{12},
			model.BottomCenter, false,
		},
		{
			"stored rows that fall", withData(cue(2, 4, "Left", "Right"), []any{int64(12), int64(10)}, []any{int64(3), int64(27)}),
			[]int64{14, 15},
			[]int64{14, 13},
			model.BottomCenter, false,
		},
		{
			"stored values of the wrong type", withData(cue(2, 4, "Left", "Right"), []any{"10", int64(12)}, []any{int64(3), int64(27)}),
			[]int64{14, 15},
			[]int64{14, 13},
			model.BottomCenter, false,
		},
		{
			"stored rows with a changed Position.Row", topAt(withData(cue(2, 4, "Left", "Right"), []any{int64(10), int64(12)}, []any{int64(3), int64(27)}), 3),
			[]int64{3, 4},
			[]int64{0, 0},
			model.TopCenter, false,
		},
		{
			"stored rows without Position.Row", withData(cue(2, 4, "Left", "Right"), []any{int64(10), int64(12)}, []any{int64(3), int64(27)}),
			[]int64{14, 15},
			[]int64{14, 13},
			model.BottomCenter, false,
		},
		{"EBU STL top row on a 23-row grid", ebu(model.TopCenter, 2), []int64{2}, []int64{12}, model.TopCenter, false},
		{"EBU STL top row with a bottom alignment", ebu(model.BottomCenter, 2), []int64{15}, []int64{12}, model.BottomCenter, false},
		{"column of the cue", column(5), []int64{13, 14}, []int64{4, 4}, model.BottomCenter, false},
		{"column that does not fit", column(30), []int64{13, 14}, []int64{25, 27}, model.BottomCenter, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cues := writeAndRead(t, tt.cue)
			require.Len(t, cues, 1)
			assert.Equal(t, tt.rows, storedInts(t, cues[0], "rows"))
			assert.Equal(t, tt.columns, storedInts(t, cues[0], "columns"))
			assert.Equal(t, tt.alignment, cues[0].Position.Alignment)
		})
	}
}

func TestWriteScheduler(t *testing.T) {
	t.Parallel()

	type span struct{ start, end int64 }
	ms := func(a, b int64) span { return span{a, b} }
	tests := []struct {
		name  string
		cues  []model.Cue
		spans []span
		lines []string
	}{
		{"load before the start", []model.Cue{cue(1, 3, "Hello!")}, []span{ms(1001, 3003)}, []string{"Hello!"}},
		{
			"short cues that crowd the load (pycaption 352)",
			[]model.Cue{
				cue(2.2, 2.359, "so,"), cue(2.4, 3.76, "the bus was late again today."), cue(4.7, 5.169, "And then,"), cue(5.21, 5.52, "oh"),
			},
			[]span{ms(2202, 2369), ms(3103, 3770), ms(4705, 5205), ms(5205, 5506)},
			[]string{"so,", "the bus was late again today.", "And then,", "oh"},
		},
		{
			"overlapping cue replaces the shown one",
			[]model.Cue{cue(1, 5, "First"), cue(3, 4, "Second")},
			[]span{ms(1001, 3003), ms(3003, 4004)},
			[]string{"First", "Second"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, report := write(t, subtitle(tt.cues...), model.CommonWriteOptions{}, scc.WriteOptions{})
			assert.Empty(t, report.Changes)
			var labels []string
			for _, l := range strings.Split(out, "\n") {
				if len(l) > 11 && l[2] == ':' {
					labels = append(labels, l[:11])
				}
			}
			assert.IsIncreasing(t, labels)

			got := readBack(t, out)
			require.Len(t, got, len(tt.spans))
			for i, c := range got {
				assert.Equal(t, span{c.Start.Milliseconds(), c.End.Milliseconds()}, tt.spans[i], "cue %d", i)
				assert.Equal(t, []string{tt.lines[i]}, c.Lines)
			}
		})
	}
}

func TestWriteFit(t *testing.T) {
	t.Parallel()

	ferry := func(t *testing.T) *model.Subtitle {
		t.Helper()

		s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, "scc/fit/own_ferry.srt")), model.CommonReadOptions{}, srt.ReadOptions{})
		require.NoError(t, err)
		return s
	}

	t.Run("own_ferry", func(t *testing.T) {
		t.Parallel()

		out, report := write(t, ferry(t), model.CommonWriteOptions{}, scc.WriteOptions{Fit: true})
		assert.Equal(t, string(testfiles.Read(t, "scc/fit/own_ferry.scc")), out)
		type change struct {
			index   int
			action  scc.FitAction
			message string
		}
		got := make([]change, 0, len(report.Changes))
		for _, c := range report.Changes {
			got = append(got, change{c.CueIndex, c.Action, c.Message})
		}
		assert.Equal(t, []change{
			{0, scc.Transliterated, "Cue #0 at 1 s: replaced \"Š\" with \"S\", \"…\" with \"...\"."},
			{0, scc.Wrapped, "Cue #0 at 1 s: wrapped the text into 3 lines of at most 32 characters."},
			{0, scc.Delayed, "Cue #0 at 1 s shows at 1.969 s. The frames before its start cannot hold its caption data."},
			{2, scc.Transliterated, "Cue #2 at 4.3 s: replaced \"–\" with \"-\"."},
			{2, scc.Wrapped, "Cue #2 at 4.3 s: wrapped the text into 2 lines of at most 32 characters."},
			{2, scc.Delayed, "Cue #2 at 4.3 s shows at 5.172 s. The frames before its start cannot hold its caption data."},
			{3, scc.Dropped, "Cue #3 at 5 s is left out. Its caption data needs the frames up to 5.839 s, but the cue ends at 5.4 s."},
		}, got)

		cues := readBack(t, out)
		require.Len(t, cues, 3)
		assert.Equal(t, []string{"The ferry to Split leaves", "from the old harbour at nine.", "Sibenik is the second stop..."}, cues[0].Lines)
		assert.Equal(t, []time.Duration{1969 * time.Millisecond, 4004 * time.Millisecond}, []time.Duration{cues[0].Start, cues[0].End})
		assert.Equal(t, []string{"Tickets cost twelve", "euros - cash only."}, cues[2].Lines)
	})

	t.Run("without fit the writer fails and reports nothing", func(t *testing.T) {
		t.Parallel()

		_, err := scc.Write(&bytes.Buffer{}, ferry(t), model.CommonWriteOptions{}, scc.WriteOptions{})
		require.ErrorIs(t, err, scc.ErrWrapFirst)

		_, report := write(t, subtitle(cue(1, 3, "Hello!"), cue(3, 3.1, "The bus was late again today.")), model.CommonWriteOptions{}, scc.WriteOptions{})
		assert.Empty(t, report.Changes)
	})

	t.Run("entities and tags", func(t *testing.T) {
		t.Parallel()

		out, report := write(t, subtitle(
			cue(1, 3, "<i>Café “Zagreb”</i>", `<font color="red">&lt;`+"Š"+`&gt;</font>`),
			cue(5, 7, "Wait&hellip;"),
		), model.CommonWriteOptions{}, scc.WriteOptions{Fit: true})
		require.Len(t, report.Changes, 2)
		assert.Equal(t, "Cue #0 at 1 s: replaced \"Š\" with \"S\".", report.Changes[0].Message)
		assert.Equal(t, scc.Transliterated, report.Changes[1].Action)
		assert.Equal(t, "Cue #1 at 5 s: replaced \"…\" with \"...\".", report.Changes[1].Message)
		cues := readBack(t, out)
		assert.Equal(t, []string{"<i>Café “Zagreb”</i>", `<font color="#ff0000">&lt;S&gt;</font>`}, cues[0].Lines)
		assert.Equal(t, []string{"Wait..."}, cues[1].Lines)
	})
}

func TestWriteOutputForm(t *testing.T) {
	t.Parallel()

	hi := subtitle(cue(1, 3, "Hi"))
	long := subtitle(cue(58, 61, "Hi"))
	colon, _ := mustRead(t, header+"00:00:01:00\t9420 9420 9470 9470 c8e9 942f 942f\n\n00:00:03:00\t942c 942c\n", model.CommonReadOptions{}, scc.ReadOptions{})
	tests := []struct {
		name     string
		s        *model.Subtitle
		common   model.CommonWriteOptions
		opts     scc.WriteOptions
		equal    string
		contains string
		prefix   string
		suffix   string
	}{
		{name: "header only for a subtitle without text", s: subtitle(cue(1, 2, "<i></i>", " ")), equal: "Scenarist_SCC V1.0\n"},
		{name: "header only for an empty subtitle", s: &model.Subtitle{}, equal: "Scenarist_SCC V1.0\n"},
		{
			name:  "hello example",
			s:     subtitle(cue(1, 3, "Hello!")),
			equal: header + "00:00:00;19\t94ae 94ae 9420 9420 9476 9476 97a1 97a1 c8e5 ecec efa1 942f 942f\n\n00:00:03;00\t942c 942c\n",
		},
		{name: "drop-frame by default", s: long, contains: "\n00:01:01;00\t942c 942c\n"},
		{name: "drop-frame off", s: long, opts: scc.WriteOptions{DropFrame: model.DropFrameOff}, contains: "\n00:01:00:28\t942c 942c\n"},
		{name: "stored colon labels", s: colon, contains: "\n00:00:03:00\t942c 942c\n"},
		{name: "drop-frame on over stored colon labels", s: colon, opts: scc.WriteOptions{DropFrame: model.DropFrameOn}, contains: "\n00:00:03;00\t942c 942c\n"},
		{
			name:   "CRLF and BOM",
			s:      hi,
			common: model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd},
			prefix: "\xEF\xBB\xBFScenarist_SCC V1.0\r\n\r\n00:00:00;21\t",
			suffix: "\r\n\r\n00:00:03;00\t942c 942c\r\n",
		},
		{name: "no BOM by default", s: hi, prefix: header},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, report := write(t, tt.s, tt.common, tt.opts)
			assert.Empty(t, report.Changes)
			if tt.equal != "" {
				assert.Equal(t, tt.equal, out)
			}
			assert.Contains(t, out, tt.contains)
			assert.True(t, strings.HasPrefix(out, tt.prefix))
			assert.True(t, strings.HasSuffix(out, tt.suffix))
		})
	}

	_, err := scc.Write(&bytes.Buffer{}, hi, model.CommonWriteOptions{}, scc.WriteOptions{DropFrame: 9})
	require.ErrorIs(t, err, model.ErrInvalidArgument)
}

func TestWriteMarkupAndCharacters(t *testing.T) {
	t.Parallel()

	styled := []string{`<i>Rain</i> and <font color="#ffff00">sun <u>later</u></font>`, `<font color="#ff0000"><i>Red</i></font> sky`}
	tests := []struct {
		name  string
		cue   model.Cue
		words []string
		lines []string
	}{
		{
			"filler and doubled control codes", cue(2, 4, "Hi!", "♪ A"),
			[]string{"94ae 94ae 9420 9420 94d6 94d6 97a2 97a2 c8e9 a180 9476 9476 97a2 97a2 9137 9137 20c1 942f 942f"},
			nil,
		},
		{
			"extended characters follow their fallback", cue(2, 4, "Café “Ö” ¡Sí!"),
			[]string{"4361 e6dc 20a2 92ae 92ae 4f80 1332 1332 a280 922f 922f 20a1 92a7 92a7 d35e a180"},
			[]string{"Café “Ö” ¡Sí!"},
		},
		{
			"mid-row codes and PAC styles", func() model.Cue { c := cue(2, 4, styled...); c.Position.Alignment = model.BottomLeft; return c }(),
			[]string{"94ce 94ce 5261 e96e 9120 9120 616e 6480 912a 912a 7375 6e80 91ab 91ab", "9468 9468 91ae 91ae 52e5 6480 9120 9120 736b 7980"},
			styled,
		},
		{
			"single-quoted color", cue(2, 4, "Rain and <font color='#ffff00'>sun</font>"), nil,
			[]string{`Rain and <font color="#ffff00">sun</font>`},
		},
		{"tags that CEA-608 cannot show", cue(2, 4, `<b>Bold</b> <v Ann>and <font color="#123456">grey</font>`), nil, []string{"Bold and grey"}},
		{"text that looks like a tag", cue(2, 4, "a &lt; b &gt; c"), []string{"6120 bc20 6220 3e20 e380"}, []string{"a &lt; b &gt; c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out, _ := write(t, subtitle(tt.cue), model.CommonWriteOptions{}, scc.WriteOptions{})
			for _, w := range tt.words {
				assert.Contains(t, out, w)
			}
			if tt.lines != nil {
				cues := readBack(t, out)
				require.Len(t, cues, 1)
				assert.Equal(t, tt.lines, cues[0].Lines)
			}
		})
	}
}

func TestWriteReadsBackRealFiles(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob(testfiles.Path(t, "scc/real") + "/*.scc")
	require.NoError(t, err)
	require.Len(t, files, 9)
	for _, file := range files {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			first, _ := mustRead(t, string(testfiles.Read(t, "scc/real/"+name)), model.CommonReadOptions{}, scc.ReadOptions{})
			out, _ := write(t, first, model.CommonWriteOptions{}, scc.WriteOptions{})
			second, _ := mustRead(t, out, model.CommonReadOptions{}, scc.ReadOptions{})
			require.Equal(t, first.Len(), second.Len())
			again, _ := write(t, second, model.CommonWriteOptions{}, scc.WriteOptions{})
			assert.Equal(t, out, again, "the second write equals the first")
			for i, a := range first.Cues() {
				b, _ := second.Cue(i)
				assert.Equal(t, a.Lines, b.Lines, "cue %d", i)
				assert.GreaterOrEqual(t, b.Start, a.Start, "cue %d starts earlier", i)
				if name == "rollup_news_ndf.scc" || name == "live_rollup_ru2.scc" {
					continue
				}
				assert.Equal(t, a.End, b.End, "cue %d", i)
			}
		})
	}
}
