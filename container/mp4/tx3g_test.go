package mp4_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v3/container/mp4"
)

// styl is a styl box with one record for each run: start character, end character and face style flags.
func styl(count int, runs ...[3]uint16) []byte {
	data := u16(uint16(count))
	for _, r := range runs {
		data = append(data, cat(u16(r[0]), u16(r[1]), u16(1), []byte{byte(r[2]), 16}, u32(0xFFFFFFFF))...)
	}
	return box("styl", data)
}

func TestTx3gText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		sample []byte
		want   []string
	}{
		{
			"italic, bold and underline after a 2-byte character",
			textSample("Café au lait\nnaïve test", styl(3, [3]uint16{5, 12, 2}, [3]uint16{13, 18, 1}, [3]uint16{19, 23, 4})),
			[]string{"Café <i>au lait</i>", "<b>naïve</b> <u>test</u>"},
		},
		{
			"bold italic, and italic over a line break",
			textSample("both plain over\nlines", styl(2, [3]uint16{0, 4, 3}, [3]uint16{11, 21, 2})),
			[]string{"<b><i>both</i></b> plain <i>over", "lines</i>"},
		},
		{
			"CR LF counts as 2 characters",
			textSample("one\r\ntwo", styl(1, [3]uint16{5, 8, 2})),
			[]string{"one", "<i>two</i>"},
		},
		{
			"a run past the text and an overlapping run",
			textSample("left right", styl(2, [3]uint16{0, 6, 1}, [3]uint16{3, 99, 2})),
			[]string{"<b>left r</b><i>ight</i>"},
		},
		{
			"a record count above the records in the box",
			textSample("left right", styl(9, [3]uint16{0, 4, 1})),
			[]string{"<b>left</b> right"},
		},
		{
			"a face style without bold, italic and underline",
			textSample("left right", styl(1, [3]uint16{0, 4, 0x08})),
			[]string{"left right"},
		},
		{
			"a damaged box before the styl box",
			textSample("left right", cat(u32(3), []byte("hlit"), styl(1, [3]uint16{0, 4, 1}))),
			[]string{"left right"},
		},
		{
			"a box before the styl box",
			textSample("left right", box("hclr", zeros(8)), styl(1, [3]uint16{5, 10, 1})),
			[]string{"left <b>right</b>"},
		},
		{
			"markup characters stay text",
			textSample("a < b & <i>c</i>", styl(1, [3]uint16{0, 1, 1})),
			[]string{"<b>a</b> &lt; b &amp; &lt;i&gt;c&lt;/i&gt;"},
		},
		{
			"UTF-16 text with a byte order mark",
			textSample("\xFE\xFF\x00C\x00a\x00f\x00\xE9\x00 \x20\xAC"),
			[]string{"Café €"},
		},
		{
			"little endian UTF-16 text",
			textSample("\xFF\xFEC\x00a\x00"),
			[]string{"Ca"},
		},
		{"UTF-8 text with a byte order mark", textSample("\xEF\xBB\xBFHi"), []string{"Hi"}},
		{"two CRs before an LF, and a lone CR", textSample("a\r\r\nb\rc"), []string{"a", "b", "c"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := extract(t, openBytes(t, single(trackSpec{}, tt.sample)), 1, mp4.ExtractOptions{})
			if assert.Len(t, s.Cues(), 1) {
				assert.Equal(t, tt.want, s.Cues()[0].Lines)
			}
		})
	}
}

func TestTx3gGaps(t *testing.T) {
	t.Parallel()

	spec := trackSpec{samples: []sampleSpec{
		{1000, textSample("")},
		{1000, []byte{0}},
		{1000, nil},
		{1000, textSample("Hi")},
		{1000, textSample("\xFE\xFF")},
	}}
	s := extract(t, openBytes(t, single(spec, nil)), 1, mp4.ExtractOptions{})
	assert.Equal(t, []cueRow{{ms(3000), ms(4000), []string{"Hi"}, false}}, rows(s))
}
