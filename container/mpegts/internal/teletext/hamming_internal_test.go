package teletext

import (
	"math/bits"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHamming84(t *testing.T) {
	t.Parallel()

	for v := range 16 {
		code := hamming84Code(v)
		assert.Equal(t, int8(v), hamming84[code], "code of %d", v)
		for i := range 8 {
			assert.Equal(t, int8(v), hamming84[code^1<<i], "code of %d with bit %d flipped", v, i)
			for j := i + 1; j < 8; j++ {
				assert.Equal(t, int8(-1), hamming84[code^1<<i^1<<j], "code of %d with bits %d and %d flipped", v, i, j)
			}
		}
	}
}

// encode2418 returns the 3 bytes of the Hamming 24/18 triplet with the 18 data bits of value.
func encode2418(value int) [3]byte {
	b := value&1<<2 | value>>1&0x07<<4 | value>>4&0x7F<<8 | value>>11&0x7F<<16
	for _, check := range []int{1, 2, 4, 8, 16} {
		ones := 0
		for pos := 1; pos <= 23; pos++ {
			if pos&check != 0 {
				ones += b >> (pos - 1) & 1
			}
		}
		if ones%2 == 0 {
			b |= 1 << (check - 1)
		}
	}
	if bits.OnesCount(uint(b))%2 == 0 {
		b |= 1 << 23
	}
	return [3]byte{byte(b), byte(b >> 8), byte(b >> 16)}
}

func flip(t [3]byte, positions ...int) [3]byte {
	for _, p := range positions {
		t[p/8] ^= 1 << (p % 8)
	}
	return t
}

func TestHamming2418(t *testing.T) {
	t.Parallel()

	for v := 0; v < 1<<18; v += 1009 {
		code := encode2418(v)
		got, ok := hamming2418(code[0], code[1], code[2])
		assert.True(t, ok)
		assert.Equal(t, v, got)
		for i := range 24 {
			e := flip(code, i)
			got, ok := hamming2418(e[0], e[1], e[2])
			assert.True(t, ok, "value %d, bit %d", v, i)
			assert.Equal(t, v, got, "value %d, bit %d", v, i)
			for j := i + 1; j < 24; j++ {
				e := flip(code, i, j)
				_, ok := hamming2418(e[0], e[1], e[2])
				assert.False(t, ok, "value %d, bits %d and %d", v, i, j)
			}
		}
	}
}

func FuzzDecoder(f *testing.F) {
	unit := func(packet []byte) []byte {
		out := make([]byte, 0, 4+len(packet))
		out = append(out, 0x03, 0x2C, 0xE7, 0xE4)
		for _, b := range packet {
			out = append(out, bits.Reverse8(b))
		}
		return out
	}
	header := make([]byte, 0, 42)
	header = append(header, hamming84Code(0), hamming84Code(0))
	for _, v := range []int{8, 8, 0, 8, 0, 8, 1, 4} {
		header = append(header, hamming84Code(v))
	}
	header = append(header, make([]byte, 32)...)
	row := append([]byte{hamming84Code(0), hamming84Code(11)}, make([]byte, 40)...)
	row[2], row[3] = 0x0B, 0xC1
	f.Add(append([]byte{0x10}, append(unit(header), unit(row)...)...))
	f.Add([]byte{0x10, 0x03, 0x2C})
	f.Fuzz(func(_ *testing.T, data []byte) {
		d := NewDecoder(0x088, 888, 0)
		d.Add(data, 90000)
		d.Add(data, 180000)
		_, _ = d.Finish()
	})
}
