package cea608_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/cea608"
)

// words reads tokens of 4 hex digits as words and "t:text" as characters two to a word with parity.
// An underscore in the text is a space, and an odd character gets the filler 0x80.
func words(t *testing.T, tokens string) []uint16 {
	t.Helper()
	var out []uint16
	for _, tok := range strings.Fields(tokens) {
		if text, ok := strings.CutPrefix(tok, "t:"); ok {
			text = strings.ReplaceAll(text, "_", " ")
			for i := 0; i < len(text); i += 2 {
				second := byte(0)
				if i+1 < len(text) {
					second = text[i+1]
				}
				out = append(out, cea608.Word(text[i], second))
			}
			continue
		}
		w, err := strconv.ParseUint(tok, 16, 16)
		require.NoError(t, err, tok)
		out = append(out, uint16(w))
	}
	return out
}

// codeLines reads each entry as "start|tokens".
func codeLines(t *testing.T, entries []string) []cea608.CodeLine {
	t.Helper()
	lines := make([]cea608.CodeLine, 0, len(entries))
	for _, e := range entries {
		start, tokens, _ := strings.Cut(e, "|")
		n, err := strconv.ParseInt(start, 10, 64)
		require.NoError(t, err)
		lines = append(lines, cea608.CodeLine{Start: n, Words: words(t, tokens)})
	}
	return lines
}

func render(s cea608.State) string {
	parts := make([]string, 0, len(s.Lines))
	for _, l := range s.Lines {
		parts = append(parts, fmt.Sprintf("%d:%d:%s", l.Row, l.Column, l.Text))
	}
	if len(parts) == 0 {
		parts = []string{"-"}
	}
	return fmt.Sprintf("%s %s", s.Mode, strings.Join(parts, "|"))
}

func popOn(data string) string {
	return "30|94ae 94ae 9420 9420 " + data + " 942f 942f"
}

func rollUpScript(first string) []string {
	return []string{
		"30|" + first + " 94ad 94ad 9470 9470 t:ONE",
		"60|t:_TWO",
		"90|" + first + " 94ad 94ad 9470 9470 t:THREE",
		"120|" + first + " 94ad 94ad 9470 9470 t:FOUR",
		"150|942c 942c",
	}
}

func TestDecode(t *testing.T) {
	t.Parallel()
	const erased = "pop-on -"
	tests := []struct {
		name    string
		channel int
		lines   []string
		want    []string
		frames  []int64  // checked when not nil
		rolls   []string // start-end row:column:text, checked when not nil
	}{
		{
			name:   "pop-on caption with its row and column",
			lines:  []string{"42|9420 9420 94ae 94ae 9452 9452 97a1 97a1 c8e5 ecec efa1 942f 942f", "120|942c 942c"},
			want:   []string{"pop-on 14:5:Hello!", erased},
			frames: []int64{53, 120},
		},
		{name: "doubled control acts once", lines: []string{popOn("9470 9470 t:ABC 94a1 94a1")}, want: []string{"pop-on 15:0:AB"}},
		{name: "third copy acts again", lines: []string{popOn("9470 9470 t:ABCD 94a1 94a1 94a1")}, want: []string{"pop-on 15:0:AB"}},
		{
			name:   "parity error in the second control byte is ignored and the copy acts",
			lines:  []string{"0|9420 9470 t:Hi 94af", "150|9420 9470 t:Hi 94af 942f"},
			want:   []string{"pop-on 15:0:Hi"},
			frames: []int64{154},
		},
		{name: "redundant copy with a parity error in the first byte", lines: []string{popOn("9470 9470 t:ABC 94a1 14a1")}, want: []string{"pop-on 15:0:AB"}},
		{name: "character with a parity error is dropped", lines: []string{popOn("9470 9470 48e9 c8e9")}, want: []string{"pop-on 15:0:iHi"}},
		{
			name:  "special and extended characters",
			lines: []string{popOn("9470 9470 9137 9137 a280 92ae 92ae 2afe 7c80 4580 92a1 92a1 4f80 13b3 13b3 a280 922f 922f")},
			want:  []string{"pop-on 15:0:\u266a\u201cáñ\u00f7Éö\u201d"},
		},
		{name: "transparent space and filler", lines: []string{popOn("9470 9470 c180 91b9 91b9 c280 8080")}, want: []string{"pop-on 15:0:A B"}},
		{
			name:  "mid-row codes",
			lines: []string{popOn("9470 9470 t:A 91ae 91ae t:B 9120 9120 t:C 912a 912a t:D 91ab 91ab t:E 9129 9129 91ae 91ae t:F")},
			want:  []string{`pop-on 15:0:A <i>B</i> C <font color="#ffff00">D <u>E</u></font>  <font color="#ff0000"><i>F</i></font>`},
		},
		{name: "row 1 white", lines: []string{popOn("9140 9140 t:Hi")}, want: []string{"pop-on 1:0:Hi"}},
		{name: "row 2 green", lines: []string{popOn("9162 9162 t:Hi")}, want: []string{`pop-on 2:0:<font color="#00ff00">Hi</font>`}},
		{name: "row 3 blue underline", lines: []string{popOn("9245 9245 t:Hi")}, want: []string{`pop-on 3:0:<font color="#0000ff"><u>Hi</u></font>`}},
		{name: "row 4 cyan", lines: []string{popOn("92e6 92e6 t:Hi")}, want: []string{`pop-on 4:0:<font color="#00ffff">Hi</font>`}},
		{name: "row 5 red", lines: []string{popOn("15c8 15c8 t:Hi")}, want: []string{`pop-on 5:0:<font color="#ff0000">Hi</font>`}},
		{name: "row 6 yellow", lines: []string{popOn("15ea 15ea t:Hi")}, want: []string{`pop-on 6:0:<font color="#ffff00">Hi</font>`}},
		{name: "row 7 magenta", lines: []string{popOn("164c 164c t:Hi")}, want: []string{`pop-on 7:0:<font color="#ff00ff">Hi</font>`}},
		{name: "row 8 italics", lines: []string{popOn("166e 166e t:Hi")}, want: []string{"pop-on 8:0:<i>Hi</i>"}},
		{name: "row 9 italics underline", lines: []string{popOn("974f 974f t:Hi")}, want: []string{"pop-on 9:0:<i><u>Hi</u></i>"}},
		{name: "row 10 indent 28", lines: []string{popOn("97fe 97fe t:Hi")}, want: []string{"pop-on 10:28:Hi"}},
		{name: "row 11 indent 4", lines: []string{popOn("1052 1052 t:Hi")}, want: []string{"pop-on 11:4:Hi"}},
		{name: "row 12 indent 8 underline", lines: []string{popOn("13d5 13d5 t:Hi")}, want: []string{"pop-on 12:8:<u>Hi</u>"}},
		{name: "row 13 indent 12", lines: []string{popOn("1376 1376 t:Hi")}, want: []string{"pop-on 13:12:Hi"}},
		{name: "row 14 indent 16", lines: []string{popOn("9458 9458 t:Hi")}, want: []string{"pop-on 14:16:Hi"}},
		{name: "row 15 indent 20", lines: []string{popOn("947a 947a t:Hi")}, want: []string{"pop-on 15:20:Hi"}},
		{name: "two rows in row order", lines: []string{popOn("9470 9470 t:B 9140 9140 t:A")}, want: []string{"pop-on 1:0:A|15:0:B"}},
		{name: "tab offset", lines: []string{popOn("9470 9470 9723 9723 t:Hi")}, want: []string{"pop-on 15:3:Hi"}},
		{name: "characters after column 32 replace the last column", lines: []string{popOn("94fe 94fe t:ABCDEFGH")}, want: []string{"pop-on 15:28:ABCH"}},
		{
			name:  "backspace and delete to end of row",
			lines: []string{popOn("9470 9470 t:ABCDEF 94a1 94a1 9470 9470 9723 9723 94a4 94a4 t:X")},
			want:  []string{"pop-on 15:0:ABCX"},
		},
		{name: "text is escaped", lines: []string{popOn("9470 9470 t:<a>_&_b")}, want: []string{"pop-on 15:0:&lt;a&gt; &amp; b"}},
		{
			name: "pop-on replaces the shown caption without erase",
			lines: []string{
				"30|9420 9420 9470 9470 t:ONE 942f 942f",
				"90|94ae 94ae 9420 9420 9470 9470 t:TWO 942f 942f",
				"150|942c 942c",
			},
			want: []string{"pop-on 15:0:ONE", "pop-on 15:0:TWO", erased},
		},
		{
			name:  "roll-up gives one state per code line",
			lines: rollUpScript("9425 9425"),
			want: []string{
				"roll-up -", "roll-up 15:0:ONE", "roll-up 15:0:ONE TWO", "roll-up 14:0:ONE TWO|15:0:THREE",
				"roll-up 14:0:THREE|15:0:FOUR", "roll-up -",
			},
			rolls: []string{"36-92 15:0:ONE TWO", "96-122 15:0:THREE", "126-150 15:0:FOUR"},
		},
		{
			name: "roll-up window follows the PAC",
			lines: []string{
				"30|9426 9426 94ad 94ad 9470 9470 t:ONE",
				"60|1370 1370",
				"90|9426 9426 94ad 94ad 1370 1370 t:THREE",
				"120|942c 942c",
			},
			want: []string{
				"roll-up -", "roll-up 15:0:ONE", "roll-up 13:0:ONE", "roll-up 12:0:ONE|13:0:THREE", "roll-up -",
			},
			rolls: []string{"36-92 13:0:ONE", "96-120 13:0:THREE"},
		},
		{
			name: "roll-up window moves to the row of a new PAC",
			lines: []string{
				"30|9426 9426 94ad 94ad 9470 9470 t:ONE",
				"60|9426 9426 94ad 94ad 9470 9470 t:TWO",
				"90|9426 9426 94ad 94ad 1370 1370 t:THREE",
			},
			want: []string{
				"roll-up -", "roll-up 15:0:ONE", "roll-up 14:0:ONE|15:0:TWO", "roll-up 11:0:ONE|12:0:TWO|13:0:THREE",
			},
		},
		{
			name: "roll-up row ends at the end of caption and a roll-up command erases pop-on",
			lines: []string{
				"30|9420 9420 9470 9470 t:POP 942f 942f",
				"90|9425 9425 94ad 94ad 9470 9470 t:ROLL",
				"150|9420 9420 9470 9470 t:NEXT 942f 942f",
				"210|942c 942c",
			},
			want:   []string{"pop-on 15:0:POP", "roll-up -", "roll-up 15:0:ROLL", "pop-on 15:0:NEXT", erased},
			frames: []int64{36, 90, 96, 156, 210},
			rolls:  []string{"96-156 15:0:ROLL"},
		},
		{
			name: "roll-up row without end",
			lines: []string{
				"30|9425 9425 94ad 94ad 9470 9470 t:ONE",
				"60|9425 9425 94ad 94ad 9470 9470 t:TWO",
			},
			want:  []string{"roll-up -", "roll-up 15:0:ONE", "roll-up 14:0:ONE|15:0:TWO"},
			rolls: []string{"36-62 15:0:ONE", "66- 15:0:TWO"},
		},
		{
			name:  "roll-up rows that roll above row 1 leave the screen",
			lines: []string{"30|94a7 94a7 9140 9140 t:A 94ad 94ad t:B 94ad 94ad t:C"},
			want:  []string{"roll-up -", "roll-up 1:0:A", "roll-up -", "roll-up 1:0:B", "roll-up -", "roll-up 1:0:C"},
		},
		{
			name:   "paint-on gives one state per code line",
			lines:  []string{"30|9429 9429 9470 9470 t:ONE", "60|t:_TWO", "90|942c 942c"},
			want:   []string{"paint-on 15:0:ONE", "paint-on 15:0:ONE TWO", "paint-on -"},
			frames: []int64{34, 60, 90},
		},
		{
			name:  "channel 1 skips the text of channel 2",
			lines: []string{"30|9420 9420 9470 9470 t:ONE 1c20 1c20 1c70 1c70 t:TWO 1c2f 1c2f 942f 942f"},
			want:  []string{"pop-on 15:0:ONE"},
		},
		{
			name:    "channel 2",
			channel: 2,
			lines:   []string{"30|9420 9420 9470 9470 t:ONE 1c20 1c20 1c70 1c70 t:TWO 1c2f 1c2f 942f 942f"},
			want:    []string{"pop-on 15:0:TWO"},
		},
		{
			name:  "extended data service packets and text mode are skipped",
			lines: []string{popOn("9470 9470 t:A 0105 t:XDS 8f10 942a 942a t:TEXT 9420 9420 t:B")},
			want:  []string{"pop-on 15:0:AB"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			channel := max(tt.channel, 1)
			d := cea608.NewDecoder(channel)
			states := d.Decode(codeLines(t, tt.lines))
			var got []string
			var frames []int64
			for _, s := range states {
				got = append(got, render(s))
				frames = append(frames, s.Frame)
			}
			assert.Equal(t, tt.want, got)
			if tt.frames != nil {
				assert.Equal(t, tt.frames, frames)
			}
			if tt.rolls != nil {
				var rolls []string
				for _, r := range d.RollUpLines() {
					end := ""
					if r.HasEnd {
						end = strconv.FormatInt(r.End, 10)
					}
					rolls = append(rolls, fmt.Sprintf("%d-%s %d:%d:%s", r.Start, end, r.Row, r.Column, r.Text))
				}
				assert.Equal(t, tt.rolls, rolls)
			}
		})
	}
}
