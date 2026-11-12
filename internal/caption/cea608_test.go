package caption_test

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/caption"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/timecode"
)

func line(t *testing.T, start int64, tokens string) cea608.CodeLine {
	t.Helper()
	l := cea608.CodeLine{Start: start}
	for _, tok := range strings.Fields(tokens) {
		if text, ok := strings.CutPrefix(tok, "t:"); ok {
			text = strings.ReplaceAll(text, "_", " ")
			for i := 0; i < len(text); i += 2 {
				second := byte(0)
				if i+1 < len(text) {
					second = text[i+1]
				}
				l.Words = append(l.Words, cea608.Word(text[i], second))
			}
			continue
		}
		w, err := strconv.ParseUint(tok, 16, 16)
		require.NoError(t, err)
		l.Words = append(l.Words, uint16(w))
	}
	return l
}

func TestCEA608Cues(t *testing.T) {
	t.Parallel()
	rollUp := func(t *testing.T) []cea608.CodeLine {
		t.Helper()
		return []cea608.CodeLine{
			line(t, 30, "9425 9425 94ad 94ad 9470 9470 t:ONE"),
			line(t, 60, "t:_TWO"),
			line(t, 90, "9425 9425 94ad 94ad 9470 9470 t:THREE"),
			line(t, 120, "9425 9425 94ad 94ad 9470 9470 t:FOUR"),
			line(t, 150, "942c 942c"),
		}
	}
	tests := []struct {
		name  string
		lines func(t *testing.T) []cea608.CodeLine
		opts  caption.CEA608Options
		want  []string // start-end alignment mode rows columns text
	}{
		{
			name: "pop-on caption, the last one lasts the last cue duration",
			lines: func(t *testing.T) []cea608.CodeLine {
				t.Helper()
				return []cea608.CodeLine{line(t, 42, "9420 9420 94ae 94ae 9452 9452 97a1 97a1 c8e5 ecec efa1 942f 942f")}
			},
			want: []string{"1768-6768 BottomCenter 14,6 pop-on [14] [5] Hello!"},
		},
		{
			name: "top row aligns to the top",
			lines: func(t *testing.T) []cea608.CodeLine {
				t.Helper()
				return []cea608.CodeLine{
					line(t, 30, "9420 9420 9470 9470 t:B 9140 9140 t:A 942f 942f"),
					line(t, 40, "942c 942c"),
					line(t, 40, "9420 9420 9470 9470 t:C 942f 942f"),
				}
			},
			want: []string{"1268-1335 TopCenter 1,1 pop-on [1 15] [0 0] A|B", "1502-6502 BottomCenter 15,1 pop-on [15] [0] C"},
		},
		{
			name: "row 4 aligns to the top and row 5 to the bottom",
			lines: func(t *testing.T) []cea608.CodeLine {
				t.Helper()
				return []cea608.CodeLine{
					line(t, 30, "9420 9420 9270 9270 t:A 942f 942f"),
					line(t, 60, "9420 9420 15d0 15d0 t:B 942f 942f"),
					line(t, 90, "942c 942c"),
				}
			},
			want: []string{"1168-2169 TopCenter 4,1 pop-on [4] [0] A", "2169-3003 BottomCenter 5,1 pop-on [5] [0] B"},
		},
		{
			name:  "roll-up by screen",
			lines: rollUp,
			want: []string{
				"1201-2002 BottomCenter 15,1 roll-up [15] [0] ONE",
				"2002-3070 BottomCenter 15,1 roll-up [15] [0] ONE TWO",
				"3070-4071 BottomCenter 14,1 roll-up [14 15] [0 0] ONE TWO|THREE",
				"4071-5005 BottomCenter 14,1 roll-up [14 15] [0 0] THREE|FOUR",
			},
		},
		{
			name:  "roll-up by rows",
			lines: rollUp,
			opts:  caption.CEA608Options{RollUp: caption.RollUpLines},
			want: []string{
				"1201-3070 BottomCenter 15,1 roll-up [15] [0] ONE TWO",
				"3203-4071 BottomCenter 15,1 roll-up [15] [0] THREE",
				"4204-5005 BottomCenter 15,1 roll-up [15] [0] FOUR",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			opts := tt.opts
			opts.Channel = 1
			cues := caption.CEA608Cues(tt.lines(t), opts, 5*time.Second)
			var got []string
			for _, c := range cues {
				d, ok := c.FormatData("scc")
				require.True(t, ok)
				mode, _ := d.Get("mode")
				rows, _ := d.Get("rows")
				columns, _ := d.Get("columns")
				got = append(got, strconv.FormatInt(c.Start.Milliseconds(), 10)+"-"+strconv.FormatInt(c.End.Milliseconds(), 10)+" "+
					c.Position.Alignment.String()+" "+strconv.Itoa(c.Position.Row)+","+strconv.Itoa(c.Position.Column)+" "+mode.(string)+" "+listString(rows)+" "+listString(columns)+" "+strings.Join(c.Lines, "|"))
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCEA608CuesClampTheLastEnd(t *testing.T) {
	t.Parallel()

	lines := []cea608.CodeLine{
		line(t, 30, "9425 9425 94ad 94ad 9470 9470 t:ONE"),
		line(t, 60, "9420 9420 9470 9470 t:TWO 942f 942f"),
	}
	for _, rollUp := range []caption.RollUp{caption.RollUpScreen, caption.RollUpLines} {
		for _, last := range []time.Duration{400000 * time.Hour, math.MaxInt64} {
			t.Run(string(rollUp)+" "+last.String(), func(t *testing.T) {
				t.Parallel()

				cues := caption.CEA608Cues(lines, caption.CEA608Options{Channel: 1, RollUp: rollUp}, last)
				require.NotEmpty(t, cues)
				for _, c := range cues {
					assert.Positive(t, c.End)
					assert.LessOrEqual(t, c.End, timecode.MaxValidTime)
				}
				assert.Equal(t, timecode.MaxValidTime, cues[len(cues)-1].End)
			})
		}
	}
}

func listString(v any) string {
	values := v.([]any)
	parts := make([]string, 0, len(values))
	for _, x := range values {
		parts = append(parts, strconv.FormatInt(x.(int64), 10))
	}
	return "[" + strings.Join(parts, " ") + "]"
}
