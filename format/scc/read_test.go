package scc_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/scc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const popOnHello = header +
	"00:00:01;12\t9420 9420 94ae 94ae 9452 9452 97a1 97a1 c8e5 ecec efa1 942f 942f\n\n" +
	"00:00:04;00\t942c 942c\n"

func TestReadPopOn(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, popOnHello, model.CommonReadOptions{}, scc.ReadOptions{})
	require.Equal(t, 1, s.Len())
	assert.Empty(t, warnings)
	cue, _ := s.Cue(0)
	assert.Equal(t, []time.Duration{1768 * time.Millisecond, 4004 * time.Millisecond}, []time.Duration{cue.Start, cue.End})
	assert.Equal(t, []string{"Hello!"}, cue.Lines)
	assert.Equal(t, model.BottomCenter, cue.Position.Alignment)
	assert.Equal(t, [2]int{14, 6}, [2]int{cue.Position.Row, cue.Position.Column})

	data, ok := cue.FormatData("scc")
	require.True(t, ok)
	mode, _ := data.Get("mode")
	rows, _ := data.Get("rows")
	columns, _ := data.Get("columns")
	assert.Equal(t, []any{"pop-on", []any{int64(14)}, []any{int64(5)}}, []any{mode, rows, columns})
}

func TestReadTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		start   time.Duration
		end     time.Duration
		text    string
	}{
		{"non-drop after one hour", "01:00:00:00\t9420 9470 " + text("Hi") + " 942f", frameTime(108000 + 3), frameTime(108000+3) + 5*time.Second, "Hi"},
		{"drop-frame after one hour", "01:00:00;00\t9420 9470 " + text("Hi") + " 942f", frameTime(107892 + 3), frameTime(107892+3) + 5*time.Second, "Hi"},
		{"each byte pair takes one frame", "00:00:00:00\t9420 9470 " + text("A") + " 942f 8080 8080 942c", frameTime(3), frameTime(6), "A"},
		{"text is escaped", "00:00:00:00\t9420 9470 " + text("<a> & b") + " 942f", frameTime(6), frameTime(6) + 5*time.Second, "&lt;a&gt; &amp; b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, header+tt.content+"\n", model.CommonReadOptions{}, scc.ReadOptions{})
			require.Equal(t, 1, s.Len())
			cue, _ := s.Cue(0)
			assert.Equal(t, []time.Duration{tt.start, tt.end}, []time.Duration{cue.Start, cue.End})
			assert.Equal(t, []string{tt.text}, cue.Lines)
		})
	}
}

func TestReadFileData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		cues    int
		want    any // the dropFrame value, nil when the file has no format data
	}{
		{"drop-frame code line", popOnHello, 1, true},
		{"non-drop code line with CR LF", "Scenarist_SCC V1.0\r\n\r\n00:00:01:00\t942c 942c\r\n", 0, false},
		{"first code line decides", header + "00:00:01:00\t942c 942c\n00:00:02;00\t942c 942c\n", 0, false},
		{"no code line", "Scenarist_SCC V1.0\n", 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, tt.content, model.CommonReadOptions{}, scc.ReadOptions{})
			assert.Equal(t, tt.cues, s.Len())
			file, ok := s.FormatData("scc")
			if tt.want == nil {
				assert.False(t, ok)
				return
			}
			require.True(t, ok)
			dropFrame, _ := file.Get("dropFrame")
			assert.Equal(t, tt.want, dropFrame)
		})
	}
}

func TestReadChannel(t *testing.T) {
	t.Parallel()

	content := header + "00:00:01:00\t9420 9420 9470 9470 " + text("ONE") + " 1c20 1c20 1c70 1c70 " + text("TWO") + " 1c2f 1c2f 942f 942f\n"
	tests := []struct {
		name string
		opts scc.ReadOptions
		want string
	}{
		{"0 reads channel 1", scc.ReadOptions{}, "ONE"},
		{"1", scc.ReadOptions{Channel: 1}, "ONE"},
		{"2", scc.ReadOptions{Channel: 2}, "TWO"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, content, model.CommonReadOptions{}, tt.opts)
			require.Equal(t, 1, s.Len())
			cue, _ := s.Cue(0)
			assert.Equal(t, []string{tt.want}, cue.Lines)
		})
	}
}

func TestReadRollUpLines(t *testing.T) {
	t.Parallel()

	content := string(testfiles.Read(t, "scc/real/rollup_news_ndf.scc"))
	s, _ := mustRead(t, content, model.CommonReadOptions{}, scc.ReadOptions{RollUp: scc.RollUpLines})
	got := make([][3]any, 0, s.Len())
	for _, c := range s.Cues() {
		got = append(got, [3]any{c.Start, c.End, c.Lines[0]})
	}
	want := [][3]any{
		{200 * time.Millisecond, 2069 * time.Millisecond, "&gt;&gt; GOOD MORNING. HERE IS THE"},
		{2202 * time.Millisecond, 3570 * time.Millisecond, "WEATHER FOR TODAY."},
		{3704 * time.Millisecond, 5072 * time.Millisecond, "CLOUDS IN THE MORNING,"},
		{5205 * time.Millisecond, 6740 * time.Millisecond, "SUN IN THE AFTERNOON."},
		{6874 * time.Millisecond, 8075 * time.Millisecond, "&gt;&gt; NOW THE TRAFFIC."},
		{8208 * time.Millisecond, 10010 * time.Millisecond, "THE BRIDGE IS OPEN."},
	}
	assert.Equal(t, want, got)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts scc.ReadOptions
		want *model.InvalidArgumentError
	}{
		{"zero", scc.ReadOptions{}, nil},
		{"channel 2 and lines", scc.ReadOptions{Channel: 2, RollUp: scc.RollUpLines}, nil},
		{"channel 3", scc.ReadOptions{Channel: 3}, &model.InvalidArgumentError{Option: "ReadOptions.SCC.Channel", Msg: "The SCC data channel must be 1 or 2, got 3."}},
		{"channel -1", scc.ReadOptions{Channel: -1}, &model.InvalidArgumentError{Option: "ReadOptions.SCC.Channel", Msg: "The SCC data channel must be 1 or 2, got -1."}},
		{"unknown roll-up", scc.ReadOptions{RollUp: "rows"}, &model.InvalidArgumentError{Option: "ReadOptions.SCC.RollUp", Msg: `The SCC roll-up mode "rows" is not screen or lines.`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.want == nil {
				require.NoError(t, err)
				return
			}
			assert.Equal(t, tt.want, err)
			_, _, readErr := read(t, popOnHello, model.CommonReadOptions{}, tt.opts)
			assert.Equal(t, tt.want, readErr)
		})
	}
}

func TestReadInvalidFiles(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		line    int
		msg     string
	}{
		{"no header", "\n00:00:01:00\t942c 942c\n", 2, `An SCC file must start with the line "Scenarist_SCC V1.0".`},
		{"other header", "Scenarist_SCC V2.0\n\n00:00:01:00\t942c 942c\n", 1, `An SCC file must start with the line "Scenarist_SCC V1.0".`},
		{"no time code", "Scenarist_SCC V1.0\n\n942c 942c\n", 3, `The SCC line "942c 942c" does not start with a time code.`},
		{"short byte pair", "Scenarist_SCC V1.0\n\n00:00:01:00\t942c 94c\n", 3, `The SCC line has the invalid byte pair "94c".`},
		{"byte pair not hex", "Scenarist_SCC V1.0\r\n\r\n00:00:01:00\t942c\n\n00:00:02:00\t94zz\n", 5, `The SCC line has the invalid byte pair "94zz".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := read(t, tt.content, model.CommonReadOptions{}, scc.ReadOptions{})
			assert.Equal(t, &model.ParseError{Format: model.SCC, Line: tt.line, Msg: tt.msg}, err)
		})
	}
}

func TestReadTimeLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		line  string
		clean bool
		msg   string
	}{
		{"non-drop 6-digit hour", "100001:00:00:00\t942c 942c", false, `The time "100001:00:00:00" is not below 100000 hours.`},
		{"non-drop label below 100000 hours", "99999:59:59:00\t942c 942c", false, `The time "99999:59:59:00" is not below 100000 hours.`},
		{"non-drop time below the limit", "99900:00:00:00\t942c 942c", true, ""},
		{"drop-frame label 100000 below the limit", "100000:05:58;29\t942c 942c", true, ""},
		{"drop-frame label 100000 at the limit", "100000:06:00;29\t942c 942c", false, `The time "100000:06:00;29" is not below 100000 hours.`},
		{"drop-frame label below the limit, last byte pair at it", "100000:05:59;29\t9420 9420 94ae 94ae 9452 9452 c8e9 942f 942f", false, `The time "100000:05:59;29" is not below 100000 hours.`},
		{"drop-frame 999999 hours", "999999:59:59;29\t942c 942c", false, `The time "999999:59:59;29" is not below 100000 hours.`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			content := header + "00:00:01;00\t942c 942c\n\n" + tt.line + "\n\n00:00:02;00\t942c 942c\n"
			_, warnings, err := read(t, content, model.CommonReadOptions{Lenient: true}, scc.ReadOptions{})
			require.NoError(t, err)
			_, _, strictErr := read(t, content, model.CommonReadOptions{}, scc.ReadOptions{})
			if tt.clean {
				require.NoError(t, strictErr)
				assert.Empty(t, warnings)
				return
			}
			assert.Equal(t, &model.ParseError{Format: model.SCC, Line: 5, Msg: tt.msg}, strictErr)
			assert.Equal(t, []model.ParseWarning{{
				Message: tt.msg, Line: 5, Block: 2, Lines: []string{tt.line}, Action: model.Skipped,
			}}, warnings)
		})
	}
}

func TestReadLenient(t *testing.T) {
	t.Parallel()

	full := string(testfiles.Read(t, "scc/real/popon_broadcast_df.scc"))
	cut := full[:len(full)*9/10]
	replaced := func(pairs ...string) string {
		return strings.NewReplacer(pairs...).Replace(full)
	}
	damaged := replaced("01:00:00;00\t94ae 94ae 9420", "01:00:00;00\t94ae 94ae 94x0", "01:00:03;00\t942c", "01:00:03;0\t942c")
	padded := replaced("01:00:00;00\t94ae 94ae 9420", "01:00:00;00\t94ae 94ae 8080", "01:00:03;00\t942c 942c", "")
	lenient := model.CommonReadOptions{Lenient: true}
	fullCues, _ := mustRead(t, full, model.CommonReadOptions{}, scc.ReadOptions{})
	paddedCues, _ := mustRead(t, padded, lenient, scc.ReadOptions{})

	t.Run("cut-off last line", func(t *testing.T) {
		t.Parallel()

		s, warnings := mustRead(t, cut, lenient, scc.ReadOptions{})
		assert.Equal(t, fullCues.Cues()[:9], s.Cues())
		assert.Equal(t, []model.ParseWarning{{
			Message: `The SCC line has the invalid byte pair "9".`, Line: 35, Block: 17,
			Lines: []string{"01:00:03;00\t942c 9"}, Action: model.Skipped,
		}}, warnings)

		_, _, err := read(t, cut, model.CommonReadOptions{}, scc.ReadOptions{})
		assert.Equal(t, &model.ParseError{Format: model.SCC, Line: 35, Msg: `The SCC line has the invalid byte pair "9".`}, err)
	})
	t.Run("bad pair and cut-off time code", func(t *testing.T) {
		t.Parallel()

		s, warnings := mustRead(t, damaged, lenient, scc.ReadOptions{})
		assert.Equal(t, paddedCues.Cues(), s.Cues())
		require.Len(t, warnings, 2)
		assert.Equal(t, [2]any{33, `The SCC line has the invalid byte pair "94x0".`}, [2]any{warnings[0].Line, warnings[0].Message})
		assert.Equal(t, [2]any{35, `The SCC line "01:00:03;0	942c 942c" does not start with a time code.`}, [2]any{warnings[1].Line, warnings[1].Message})
	})
	t.Run("no header", func(t *testing.T) {
		t.Parallel()

		_, _, err := read(t, "00:00:01:00\t942c 942c\n", lenient, scc.ReadOptions{})
		assert.Equal(t, &model.ParseError{Format: model.SCC, Line: 1, Msg: `An SCC file must start with the line "Scenarist_SCC V1.0".`}, err)
	})
}
