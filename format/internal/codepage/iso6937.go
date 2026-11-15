package codepage

type composition struct {
	diacritic byte
	base      byte
}

type iso6937Codec struct {
	encode     map[rune]byte
	compose    map[composition]rune
	decompose  map[rune]composition
	markToByte map[rune]byte
}

//nolint:gochecknoglobals // fixed code page table
var iso6937 = newISO6937()

const (
	iso6937Currency = 0x24
	diacriticColumn = 0xC0
	lastDiacritic   = 0xCF
)

func newISO6937() *iso6937Codec {
	c := &iso6937Codec{
		encode:     map[rune]byte{'¤': iso6937Currency},
		compose:    make(map[composition]rune, len(iso6937Composed)),
		decompose:  make(map[rune]composition, len(iso6937Composed)),
		markToByte: make(map[rune]byte, len(iso6937Marks)),
	}
	for i, r := range iso6937Upper {
		if r != 0 {
			c.encode[r] = byte(0x80 + i)
		}
	}
	for i, r := range iso6937Marks {
		if r != 0 {
			c.markToByte[r] = byte(diacriticColumn + i)
		}
	}
	for _, p := range iso6937Composed {
		k := composition{p.diacritic, p.base}
		c.compose[k] = p.letter
		c.decompose[p.letter] = k
	}
	return c
}

func iso6937Character(b byte) rune {
	switch {
	case b == iso6937Currency:
		return '\u00A4'
	case b >= 0x20 && b < 0x7F:
		return rune(b)
	case b >= 0x80:
		return iso6937Upper[b-0x80]
	}
	return 0
}

func isDiacritic(b byte) bool {
	return b > diacriticColumn && b <= lastDiacritic && iso6937Marks[b-diacriticColumn] != 0
}

// DecodeISO6937 returns the text of b. It drops each undefined byte and each diacritic byte without a character after it.
func DecodeISO6937(b []byte) string {
	out := make([]rune, 0, len(b))
	var held byte
	for _, c := range b {
		if isDiacritic(c) {
			held = c
			continue
		}
		if r := iso6937Character(c); r != 0 {
			switch composed, ok := iso6937.compose[composition{held, c}]; {
			case held == 0:
				out = append(out, r)
			case ok:
				out = append(out, composed)
			default:
				out = append(out, r, iso6937Marks[held-diacriticColumn])
			}
		}
		held = 0
	}
	return string(out)
}

// EncodeISO6937 returns the bytes of s and writes Replacement for each code point outside the table.
func EncodeISO6937(s string) []byte {
	out, _ := EncodeISO6937Replaced(s)
	return out
}

// EncodeISO6937Replaced is EncodeISO6937 that also returns the code points that it wrote as Replacement, in order.
// A Replacement that s holds itself is not in the list.
func EncodeISO6937Replaced(s string) ([]byte, []rune) {
	out := make([]byte, 0, len(s))
	var replaced []rune
	letter := false
	for _, r := range s {
		wasLetter := letter
		letter = false
		c, inTable := iso6937.encode[r]
		k, composed := iso6937.decompose[r]
		m, isMark := iso6937.markToByte[r]
		switch {
		case inTable:
			out = append(out, c)
		case r >= 0x20 && r < 0x7F:
			out = append(out, byte(r))
			letter = isASCIILetter(byte(r))
		case composed:
			out = append(out, k.diacritic, k.base)
		case isMark && wasLetter:
			last := out[len(out)-1]
			out[len(out)-1] = m
			out = append(out, last)
		default:
			out = append(out, Replacement)
			replaced = append(replaced, r)
		}
	}
	return out, replaced
}

// EncodableISO6937 reports whether EncodeISO6937 writes r without Replacement when r stands alone.
// A combining mark is not encodable alone, because it needs a letter before it.
// A caller that counts replacements by code point must count a mark after an ASCII letter as encodable.
func EncodableISO6937(r rune) bool {
	_, inTable := iso6937.encode[r]
	_, composed := iso6937.decompose[r]
	return inTable || composed || (r >= 0x20 && r < 0x7F)
}

func isASCIILetter(b byte) bool {
	return b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z'
}
