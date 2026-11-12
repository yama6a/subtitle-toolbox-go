package cea608_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
)

func hexWords(words []uint16) string {
	parts := make([]string, len(words))
	for i, w := range words {
		parts[i] = fmt.Sprintf("%04x", w)
	}
	return strings.Join(parts, " ")
}

func TestRowWords(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		line        string
		row, column int
		want        string
	}{
		{name: "plain text", line: "Hi", row: 15, want: "9470 9470 c8e9"},
		{name: "tab offset of 1", line: "Hi", row: 15, column: 5, want: "94f2 94f2 97a1 97a1 c8e9"},
		{name: "tab offset of 2", line: "Hi", row: 15, column: 6, want: "94f2 94f2 97a2 97a2 c8e9"},
		{name: "tab offset of 3", line: "Hi", row: 15, column: 7, want: "94f2 94f2 9723 9723 c8e9"},
		{name: "mid-row code replaces a space", line: "A <i>B</i>", row: 15, want: "9470 9470 c180 91ae 91ae c280"},
		{name: "leading code at column 0 moves into the PAC", line: `<font color="#ffff00">Hi</font>`, row: 15, want: "94ea 94ea c8e9"},
		{
			name: "leading code at column 2 stays a code", line: `<font color="#ffff00">Hi</font>`, row: 15, column: 2,
			want: "9470 9470 97a1 97a1 912a 912a c8e9",
		},
		{name: "italics plus a color at column 0", line: `<font color="#ffff00"><i>Hi</i></font>`, row: 15, want: "94ea 94ea 91ae 91ae c8e9"},
		{name: "lone standard character before a special character", line: "A\u266a", row: 15, want: "9470 9470 c180 9137 9137"},
		{name: "extended character writes its fallback first", line: "É", row: 15, want: "9470 9470 4580 92a1 92a1"},
		{name: "row outside the screen", line: "Hi", row: 16, want: ""},
		{name: "column outside the screen", line: "Hi", row: 1, column: 32, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			words := cea608.RowWords(tt.row, tt.column, cea608.Cells(cea608.StyledCharacters(tt.line)))
			assert.Equal(t, tt.want, hexWords(words))
		})
	}
}

func TestCellsDropStylesAbove32Cells(t *testing.T) {
	t.Parallel()
	line := `<i>` + strings.Repeat("a", 31) + `</i> b`
	cells := cea608.Cells(cea608.StyledCharacters(line))
	assert.Len(t, cells, 33)
	for _, c := range cells {
		assert.Zero(t, c.MidRow)
	}
}

func TestStyledCharacters(t *testing.T) {
	t.Parallel()
	got := cea608.StyledCharacters(`  <c.yellow>a</c>&amp;<font color="red">b</font><font color="#00ff00"><u>c</u></font>&nbsp;`)
	want := []cea608.Char{
		{R: 'a', Color: 5},
		{R: '&'},
		{R: 'b', Color: 4},
		{R: 'c', Color: 1, Underline: true},
	}
	assert.Equal(t, want, got)
}

func FuzzEncode(f *testing.F) {
	for _, s := range []string{"Hi", "A <i>B</i> C", `<font color="#ff0000">x</font> y`, "Été \u266a", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		chars := cea608.StyledCharacters(s)
		if len(chars) > cea608.Columns {
			return
		}
		for _, c := range chars {
			if !cea608.Encodable(c.R) {
				return
			}
		}
		var plain strings.Builder
		for _, c := range cea608.Cells(chars) {
			if c.MidRow != 0 {
				plain.WriteByte(' ')
				continue
			}
			plain.WriteRune(c.R)
		}
		want := strings.Trim(plain.String(), " ")
		if want == "" {
			return
		}
		eoc := cea608.Word(0x14, 0x2f)
		rcl := cea608.Word(0x14, 0x20)
		words := slices.Concat([]uint16{rcl, rcl}, cea608.RowWords(15, 0, cea608.Cells(chars)), []uint16{eoc, eoc})
		states := cea608.NewDecoder(1).Decode([]cea608.CodeLine{{Words: words}})
		if len(states) == 0 || len(states[len(states)-1].Lines) != 1 {
			t.Fatalf("no caption for %q", s)
		}
		got := markupcore.DecodeEntities(markupcore.StripAllTags(states[len(states)-1].Lines[0].Text))
		if got != want {
			t.Fatalf("decoded %q, want %q", got, want)
		}
	})
}

func FuzzDecode(f *testing.F) {
	f.Add([]byte{0, 0x94, 0x20, 0, 0x94, 0x70, 0, 0xc8, 0xe9, 0, 0x94, 0x2f}, false)
	f.Add([]byte{0, 0x94, 0x25, 0xff, 0x94, 0xad, 0, 0x92, 0x2f}, true)
	f.Fuzz(func(t *testing.T, data []byte, second bool) {
		var lines []cea608.CodeLine
		start := int64(0)
		for len(data) >= 3 {
			gap, w := data[0], uint16(data[1])<<8|uint16(data[2])
			data = data[3:]
			if gap > 200 || len(lines) == 0 {
				start += int64(gap)
				lines = append(lines, cea608.CodeLine{Start: start})
			}
			lines[len(lines)-1].Words = append(lines[len(lines)-1].Words, w)
		}
		channel := 1
		if second {
			channel = 2
		}
		d := cea608.NewDecoder(channel)
		for _, s := range d.Decode(lines) {
			for _, l := range s.Lines {
				if l.Row < 1 || l.Row > cea608.Rows || l.Column < 0 || l.Column >= cea608.Columns {
					t.Fatalf("line out of the screen: %+v", l)
				}
			}
		}
		for _, r := range d.RollUpLines() {
			if r.Row < 1 || r.Row > cea608.Rows {
				t.Fatalf("roll-up row out of the screen: %+v", r)
			}
		}
	})
}
