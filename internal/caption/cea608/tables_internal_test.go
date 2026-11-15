package cea608

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParity(t *testing.T) {
	t.Parallel()
	var low, high []byte
	for b := byte(0x00); b <= 0x07; b++ {
		low = append(low, WithParity(b))
		high = append(high, WithParity(0x70+b))
	}
	assert.Equal(t, []byte{0x80, 0x01, 0x02, 0x83, 0x04, 0x85, 0x86, 0x07}, low)
	assert.Equal(t, []byte{0x70, 0xF1, 0xF2, 0x73, 0xF4, 0x75, 0x76, 0xF7}, high)
	assert.True(t, HasOddParity(0x94))
	assert.False(t, HasOddParity(0x14))
	assert.Equal(t, uint16(0x9470), Word(0x14, 0x70))
}

func TestPacRoundTrip(t *testing.T) {
	t.Parallel()
	roundTrip := func(p pac) pac {
		first, second := encodePac(p)
		got, ok := decodePac(first, second)
		require.True(t, ok)
		return got
	}
	for row := 1; row <= Rows; row++ {
		for column := 0; column <= 28; column += 4 {
			for _, underline := range []bool{false, true} {
				want := pac{row: row, column: column, underline: underline}
				assert.Equal(t, want, roundTrip(want))
			}
		}
		for color := range colors {
			assert.Equal(t, pac{row: row, color: color}, roundTrip(pac{row: row, color: color}))
		}
		assert.Equal(t, pac{row: row, italic: true}, roundTrip(pac{row: row, italic: true}))
	}

	for _, tt := range []struct {
		p    pac
		want [2]byte
	}{
		{pac{row: 11}, [2]byte{0x10, 0xD0}},
		{pac{row: 15, column: 28, underline: true}, [2]byte{0x94, 0x7F}},
		{pac{row: 3, color: 6, underline: true}, [2]byte{0x92, 0xCD}},
	} {
		first, second := encodePac(tt.p)
		assert.Equal(t, tt.want, [2]byte{WithParity(first), WithParity(second)})
	}
}

func TestCharacterCodes(t *testing.T) {
	t.Parallel()
	for c := byte(0x20); c <= 0x7E; c++ {
		r, ok := standardCharacter(c)
		require.True(t, ok)
		assert.Equal(t, code{hasByte: true, b: c}, characterCodes[r])
	}
	for second := byte(0x30); second <= 0x3F; second++ {
		r, _ := specialCharacter(second)
		if second != transparentSpace {
			assert.Equal(t, [2]byte{firstMidRow, second}, characterCodes[r].pair)
		}
	}
	for _, first := range []byte{0x12, 0x13} {
		for second := byte(0x20); second <= 0x3F; second++ {
			r, _ := extendedCharacter(first, second)
			cd := characterCodes[r]
			assert.Equal(t, [2]byte{first, second}, cd.pair)
			_, ok := standardCharacter(cd.b)
			assert.True(t, ok)
		}
	}
	assert.False(t, Encodable('\u20ac'))
}

func TestTransliterate(t *testing.T) {
	t.Parallel()
	count := 0
	for r := rune(0xA0); r <= 0x24FF; r++ {
		to, ok := Transliterate(r)
		if !ok {
			continue
		}
		count++
		assert.False(t, Encodable(r), "%q", r)
		for _, part := range to {
			assert.True(t, Encodable(part), "%q", part)
		}
	}
	assert.Greater(t, count, 100)
	_, ok := Transliterate('\u65e5')
	assert.False(t, ok)
	for r, want := range map[rune]string{'Š': "S", 'Œ': "OE", '\u2026': "...", '`': "'"} {
		got, _ := Transliterate(r)
		assert.Equal(t, want, got)
	}
}

func TestMidRow(t *testing.T) {
	t.Parallel()
	style, underline := decodeMidRow(0x29)
	assert.Equal(t, []any{4, true}, []any{style, underline})
	style, underline = decodeMidRow(0x2E)
	assert.Equal(t, []any{styleItalic, false}, []any{style, underline})
	assert.Equal(t, byte(0x2F), midRow(styleItalic, true))
	assert.Equal(t, byte(0x20), midRow(white, false))
}
