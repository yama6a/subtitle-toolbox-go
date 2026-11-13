package arib_test

import (
	"crypto/md5" //nolint:gosec // the hash names a glyph in a message
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/arib"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	cs      = "\x0C"
	apr     = "\x0D"
	ls0     = "\x0F"
	ls1     = "\x0E"
	ssz     = "\x88"
	msz     = "\x89"
	nsz     = "\x8A"
	yellow  = "\x83"
	setup   = "\x9B7 \x53\x9B170;30 \x5F\x9B620;480 \x56\x9B36;36 \x57\x9B4 \x58\x9B24 \x59"
	tenth   = 90000 / 10
	seconds = 90000
)

func position(row, column int) string {
	return "\x1C" + string(rune(0x40+row)) + string(rune(0x40+column))
}

func dots(x, y int) string { return "\x9B" + strconv.Itoa(x) + ";" + strconv.Itoa(y) + " \x61" }

func wait(tenths int) string { return "\x9D\x20" + string(rune(0x40+tenths)) }

// kanji returns JIS X 0208 codes in GL.
func kanji(text string) string {
	b, err := japanese.EUCJP.NewEncoder().Bytes([]byte(text))
	if err != nil {
		panic(err)
	}
	for i := range b {
		b[i] &= 0x7F
	}
	return string(b)
}

// hiragana returns codes of the hiragana set in GR.
func hiragana(text string) string {
	var sb strings.Builder
	for _, r := range text {
		sb.WriteByte(byte(0xA1 + r - 0x3041))
	}
	return sb.String()
}

// latin returns ASCII and, for other letters, ISO 8859-15 bytes for the Latin extension set in GR.
func latin(text string) string {
	var sb strings.Builder
	for _, r := range text {
		if r < 0x80 {
			sb.WriteRune(r)
			continue
		}
		b, err := charmap.ISO8859_15.NewEncoder().Bytes([]byte(string(r)))
		if err != nil {
			panic(err)
		}
		sb.Write(b)
	}
	return sb.String()
}

func uint24(n int) string { return string([]byte{byte(n >> 16), byte(n >> 8), byte(n)}) }

func unit(parameter byte, data string) string {
	return "\x1F" + string(parameter) + uint24(len(data)) + data
}

func text(codes string) string { return unit(0x20, codes) }

func dataGroup(id int, data string) string {
	group := string([]byte{byte(id << 2), 0, 0}) + string(binary.BigEndian.AppendUint16(nil, uint16(len(data)))) + data
	return "\x80\xFF\xF0" + group + "\x00\x00"
}

func statement(units string) string { return dataGroup(1, "\x3F"+uint24(len(units))+units) }

func management(utf8 bool, languages ...string) string {
	data := "\x3F" + string(rune(len(languages)))
	for i, l := range languages {
		tcs := byte(0)
		if utf8 {
			tcs = 0x04
		}
		data += string([]byte{byte(i << 5)}) + l + string([]byte{0x80 | tcs})
	}
	return dataGroup(0, data+uint24(0))
}

// drcs returns a DRCS unit of 1-byte codes. A glyph is {code, depth, width, height, pattern in hexadecimal}.
func drcs(glyphs ...[5]any) string {
	data := string(rune(len(glyphs)))
	for _, g := range glyphs {
		pattern, err := hex.DecodeString(g[4].(string))
		if err != nil {
			panic(err)
		}
		data += string(binary.BigEndian.AppendUint16(nil, uint16(g[0].(int)))) + "\x01\x01" +
			string([]byte{byte(g[1].(int) - 2), byte(g[2].(int)), byte(g[3].(int))}) + string(pattern)
	}
	return unit(0x30, data)
}

type packet struct {
	pts  float64
	data string
}

type stateRow struct {
	Start     float64
	Lines     []string
	Alignment model.Alignment
}

// decode feeds the packets to a decoder for jpn and returns its states without the empty one at time 0.
func decode(language string, packets ...packet) ([]stateRow, []arib.Warning) {
	d := arib.NewDecoder(language)
	for _, p := range packets {
		d.Add([]byte(p.data), int64(p.pts*seconds))
	}
	var rows []stateRow
	for _, s := range d.Finish() {
		if len(s.Lines) > 0 || s.Start > 0 {
			rows = append(rows, stateRow{float64(s.Start) / seconds, s.Lines, s.Alignment})
		}
	}
	return rows, d.Warnings()
}

func TestDecoderText(t *testing.T) {
	t.Parallel()

	frame := md5.Sum(mustHex(t, glyphFrame)) //nolint:gosec // see the import
	glyphs := drcs([5]any{0x4121, 4, 18, 36, glyphFF5F}, [5]any{0x4122, 4, 18, 36, glyphFF60}, [5]any{0x4123, 4, 36, 36, glyph51DC},
		[5]any{0x4124, 2, 16, 18, glyphFrame})
	const base = cs + setup
	tests := []struct {
		name     string
		language string
		packets  []packet
		want     []stateRow
		warnings []string
	}{
		{
			"kanji, kana, colors and middle size", "jpn",
			[]packet{{1, statement(text(base + position(6, 3) + kanji("\u4eca\u65e5\u306f") + hiragana("\u3044\u3044") +
				kanji("\u5929\u6c17\u3067\u3059\u3002") + apr + yellow + kanji("\u660e\u65e5\u3082") + msz + ls1 + "OK!" + ls0 + kanji("\u3001") + nsz + ls1 + "OK" + ls0))}},
			[]stateRow{{1, []string{"\u4eca\u65e5\u306f\u3044\u3044\u5929\u6c17\u3067\u3059\u3002", "<font color=\"#ffff00\">\u660e\u65e5\u3082OK!\uff64\uff2f\uff2b</font>"}, model.BottomCenter}},
			nil,
		},
		{
			"small text above a row is ruby of the characters below", "jpn",
			[]packet{{0, statement(text(base + position(6, 3) +
				kanji("\u4eca\u65e5\u306f\u3044\u3044\u5929\u6c17\u3067\u3059\u3002") + ssz + dots(500, 390) + hiragana("\u3066\u3093\u304d") + dots(300, 390) + hiragana("\u304d\u3087\u3046")))}},
			[]stateRow{{0, []string{"<ruby>\u4eca\u65e5<rt>\u304d\u3087\u3046</rt></ruby>\u306f\u3044\u3044<ruby>\u5929\u6c17<rt>\u3066\u3093\u304d</rt></ruby>\u3067\u3059\u3002"}, model.BottomCenter}},
			nil,
		},
		{
			"furigana without a row below is a line", "jpn",
			[]packet{{0, statement(text(base + ssz + dots(500, 420) + hiragana("\u3066\u3093\u304d")))}},
			[]stateRow{{0, []string{"\u3066\u3093\u304d"}, model.BottomCenter}},
			nil,
		},
		{"TIME waits and CS end the text", "jpn", []packet{
			{1, statement(text(base + position(7, 0) + kanji("\u306f\u3044") + wait(25) + cs))},
			{5, statement(text(base + position(0, 0) + kanji("\u4e0a")))},
			{6, statement(text(cs))},
		}, []stateRow{
			{1, []string{"\u306f\u3044"}, model.BottomLeft},
			{3.5, nil, model.BottomCenter},
			{5, []string{"\u4e0a"}, model.TopLeft},
			{6, nil, model.BottomCenter},
		}, nil},
		{
			"known DRCS glyphs map and unknown ones warn once", "jpn",
			[]packet{{2, statement(glyphs + text(base+position(6, 3)+
				"\x1B\x29\x20\x41"+ls1+"\x21\x23\x22\x24\x25\x24"+ls0))}},
			[]stateRow{{2, []string{"\uff5f\u51dc\uff60\u3013\u3013\u3013"}, model.BottomCenter}},
			[]string{
				fmt.Sprintf("The glyph of the character 0x24 of the set DRCS-1 (16x18 dots, MD5 %x) has no known character. The reader wrote U+3013 for it.", frame),
				"The character 0x25 of the set DRCS-1 has no glyph in the stream. The reader wrote U+3013 for it.",
			},
		},
		{
			"a profile C glyph that is not in the table", "jpn",
			[]packet{{0, statement(drcs([5]any{0x4121, 2, 16, 18, strings.Repeat("00", 36)}) + text("\x1B\x29\x20\x41\x0E\x21"))}},
			[]stateRow{{0, []string{"\u3013"}, model.TopLeft}},
			[]string{"The glyph of the character 0x21 of the set DRCS-1 (16x18 dots, MD5 " + hashOf(t, strings.Repeat("00", 36)) + ") has no known character. The reader wrote U+3013 for it."},
		},
		{
			"a known glyph moved 2 dots", "jpn",
			[]packet{{0, statement(drcs([5]any{0x4121, 4, 18, 36, shiftLeft(glyphFF5F, 9)}) + text("\x1B\x29\x20\x41\x0E\x21"))}},
			[]stateRow{{0, []string{string(rune(0xFF5F))}, model.TopLeft}},
			nil,
		},
		{
			"JIS X 0201 katakana stays full width in a Latin stream at normal size", "por",
			[]packet{{0, management(false, "por")}, {1, statement(text(base + position(6, 3) + nsz + "\x1B\x2B\x49\x1D\x41"))}},
			[]stateRow{{1, []string{string(rune(0x30C1))}, model.BottomLeft}},
			nil,
		},
		{
			"rows 85 to 94", "jpn",
			[]packet{{0, statement(text(base + position(6, 3) + "\x75\x21\x7A\x21\x7C\x21\x7E\x21"))}},
			[]stateRow{{0, []string{"\u3402\u26cc\u27a1\u2160"}, model.BottomCenter}},
			nil,
		},
		{
			"other sets and position moves", "jpn",
			[]packet{{0, statement(text(base + position(6, 3) +
				"\x1B\x2B\x31\x1B\x7C\xA2\xA4\x08\xA6\x16\x42\x1B\x2A\x49\x19\x31" + "\x90\x20\x41\x90\x48" + kanji("\u8272")))}},
			[]stateRow{{0, []string{"\u30a2\u30a6 \u30a2<font color=\"#00ffaa\">\u8272</font>"}, model.BottomCenter}},
			nil,
		},
		{
			"default macros", "jpn",
			[]packet{{0, statement(text(base + position(6, 3) + ls1 + "A" + "\x1D\x60" + kanji("\u5b57")))}},
			[]stateRow{{0, []string{"\uff21\u5b57"}, model.BottomCenter}},
			nil,
		},
		{
			"RPC repeats a character", "jpn",
			[]packet{{0, statement(text(base + position(6, 3) + "\x98\x43" + kanji("\u30fc") + kanji("\uff01")))}},
			[]stateRow{{0, []string{"\u30fc\u30fc\u30fc\uff01"}, model.BottomCenter}},
			nil,
		},
		{
			"JIS X 0208 cells that x/text maps differently", "jpn",
			[]packet{{0, statement(text(base + position(6, 3) + "\x21\x41\x21\x42\x21\x5D\x21\x71\x22\x4C"))}},
			[]stateRow{{0, []string{"\u301c\u2016\u2212\u00a2\u00ac"}, model.BottomCenter}},
			nil,
		},
		{
			"JIS X 0213 plane 2 and plane 1 cells give U+FFFD", "jpn",
			[]packet{{0, statement(text(base + position(6, 3) +
				"\x1B\x24\x2A\x3A\xA1\xA1" + "\x1B\x24\x2A\x39\xAF\xA1"))}},
			[]stateRow{{0, []string{"\ufffd\ufffd"}, model.BottomCenter}},
			nil,
		},
		{
			"Latin sets of Brazilian captions", "por",
			[]packet{{0, management(false, "por")}, {1, statement(text(base + position(6, 3) +
				"\x1D\x21 " + latin("Ação já, señor.") + apr + "\x91\x40" + latin("Até logo!")))}},
			[]stateRow{{1, []string{"\u266a Ação já, señor.", "Até logo!"}, model.BottomCenter}},
			nil,
		},
		{
			"UTF-8 captions of the UCS coding", "tgl",
			[]packet{{0, management(true, "tgl")}, {1, statement(text(
				strings.ReplaceAll(base, "\x9B", "\xC2\x9B") + position(6, 0) + "Salamat. \xC2\x83Oo"))}},
			[]stateRow{{1, []string{"Salamat. <font color=\"#ffff00\">Oo</font>"}, model.BottomCenter}},
			nil,
		},
		{"a repeated management group and the second language are skipped", "jpn", []packet{
			{0, management(false, "jpn")},
			{0.5, management(false, "por")},
			{1, statement(text(base + position(6, 7) + kanji("\u65e5")))},
			{2, strings.Replace(statement(text(kanji("\u82f1"))), "\x04\x00\x00", "\x08\x00\x00", 1)},
		}, []stateRow{{1, []string{"\u65e5"}, model.BottomCenter}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, warnings := decode(tt.language, tt.packets...)
			assert.Equal(t, tt.want, got)
			var messages []string
			for _, w := range warnings {
				messages = append(messages, w.Message)
			}
			assert.Equal(t, tt.warnings, messages)
		})
	}
}

func TestDecoderStopsAtDamage(t *testing.T) {
	t.Parallel()

	short := statement(text(cs + setup + position(6, 7) + kanji("\u5b57")))
	tests := []struct{ name, data string }{
		{"ESC", "\x1B"},
		{"ESC 24", "\x1B\x24"},
		{"CSI without its end", "\x9B\x31\x32"},
		{"COL", "\x90"},
		{"APS", "\x1C\x40"},
		{"SS2", "\x19"},
		{"macro definition", "\x95\x40"},
		{"TIME", "\x9D\x29\x31"},
		{"half a kanji", kanji("\u5b57")[:1]},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, _ := decode("jpn", packet{0, statement(text(tt.data))})
			assert.Empty(t, got)
		})
	}
	t.Run("a group that ends early", func(t *testing.T) {
		t.Parallel()

		got, _ := decode("jpn", packet{0, short[:len(short)-4]})
		assert.Empty(t, got)
	})
	t.Run("a unit that ends early", func(t *testing.T) {
		t.Parallel()

		got, _ := decode("jpn", packet{0, strings.Replace(short, "\x1F\x20\x00", "\x1F\x20\xFF", 1)})
		assert.Empty(t, got)
	})
}

func TestLanguage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "por", arib.Language([]byte(management(false, "por"))))
	assert.Empty(t, arib.Language([]byte(statement(""))))
}

func TestColors(t *testing.T) {
	t.Parallel()

	want := strings.Split(strings.TrimSpace(string(testfiles.Read(t, "mpegts/arib_colors.txt"))), "\n")
	require.Len(t, want, 128)
	for i, c := range want {
		if c == "-" {
			c = ""
		}
		assert.Equal(t, c, arib.Color(i/16, i%16), "palette %d index %d", i/16, i%16)
	}
}

func mustHex(tb testing.TB, s string) []byte {
	tb.Helper()

	b, err := hex.DecodeString(s)
	require.NoError(tb, err)
	return b
}

func hashOf(tb testing.TB, pattern string) string {
	tb.Helper()

	return fmt.Sprintf("%x", md5.Sum(mustHex(tb, pattern))) //nolint:gosec // see the import
}

// shiftLeft drops the first nibble of each row of a pattern in hexadecimal and ends the row with a 0 nibble.
// A nibble holds 2 dots.
func shiftLeft(pattern string, nibbles int) string {
	var sb strings.Builder
	for row := 0; row+nibbles <= len(pattern); row += nibbles {
		sb.WriteString(pattern[row+1 : row+nibbles])
		sb.WriteByte('0')
	}
	return sb.String()
}

// TestDecoderStateLimit feeds a full screen and then 8,000 changes of one character, each with a TIME control.
func TestDecoderStateLimit(t *testing.T) {
	t.Parallel()

	codes := cs + "\x9B1;1 \x57\x9B0 \x58\x9B0 \x59" + ls1 + strings.Repeat("\x98\x40A", 600) +
		strings.Repeat("\x1C\x40\x40B"+wait(0)+"\x1C\x40\x40A"+wait(0), 4000)
	d := arib.NewDecoder("jpn")
	d.Add([]byte(statement(text(codes))), 0)
	assert.Less(t, len(d.Finish()), 1000)
	require.Len(t, d.Warnings(), 2)
	assert.True(t, d.Warnings()[0].Skipped)
	assert.Equal(t, "The caption screen holds more than 4096 characters. The reader dropped the characters past that.", d.Warnings()[0].Message)
	assert.Equal(t, "The caption states of one PES packet are longer than 32 KiB plus 32 times the packet. The reader dropped the states past that.", d.Warnings()[1].Message)
}
