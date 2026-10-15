package linewrap_test

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/linewrap"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

func FuzzWrap(f *testing.F) {
	f.Add("<i>This is a very long <b>subtitle line</b> that does not fit</i> on the screen", uint8(42), uint8(2))
	f.Add("- Are you coming? - Yes.\n- No.\n-20 degrees", uint8(10), uint8(1))
	f.Add("\u6211\u4eec\u3001\u300a\u7ea2\u300b\u30fc&amp;<i>\u4e1c</i><b>\u4eac</b> WhatsApp", uint8(2), uint8(3))
	f.Add("Tokyo\xff \u6211\u4eec \u05d0\u05d1 \u202b\u05d0\u202c", uint8(5), uint8(60))

	f.Fuzz(func(t *testing.T, text string, cpl, maxLines uint8) {
		// The break search takes maxLines * parts^2 steps, so a long input slows the fuzzer and finds nothing new.
		if len(text) > 512 {
			return
		}
		lines := strings.Split(text, "\n")
		width, count := 1+int(cpl)%60, 1+int(maxLines)%60
		wrapped := linewrap.Wrap(lines, width, count, false)
		unwrapped := linewrap.UnwrapTurns(lines)
		linewrap.WrapTurns(lines, width, count)
		linewrap.WrapToFit(lines, width, count)
		linewrap.FewestLines(lines, width)

		if strings.ContainsAny(text, "\u202b\u202c") {
			return
		}
		want := visibleWithoutSpace(lines)
		if got := visibleWithoutSpace(wrapped); got != want {
			t.Fatalf("Wrap(%q, %d, %d) = %q changes the visible text %q to %q", lines, width, count, wrapped, want, got)
		}
		if got := visibleWithoutSpace(unwrapped); got != want {
			t.Fatalf("UnwrapTurns(%q) = %q changes the visible text %q to %q", lines, unwrapped, want, got)
		}
	})
}

// visibleWithoutSpace returns the plain text of the lines joined with spaces, without white space. It keeps the
// bytes of invalid UTF-8.
func visibleWithoutSpace(lines []string) string {
	text := markupcore.PlainText(strings.Join(lines, " "))
	var b strings.Builder
	for text != "" {
		r, size := utf8.DecodeRuneInString(text)
		if !unicode.IsSpace(r) {
			b.WriteString(text[:size])
		}
		text = text[size:]
	}
	return b.String()
}

// randomText writes words of ASCII letters, Han, kana and CJK punctuation, with a fixed seed.
type randomText struct {
	r *rand.Rand
}

func newRandomText(seed uint64) randomText {
	return randomText{r: rand.New(rand.NewPCG(seed, seed))} //nolint:gosec // test text needs no secure random numbers
}

func (g randomText) intn(n int) int { return g.r.IntN(n) }

func (g randomText) text(words int) string {
	cjk := []rune{0x3001, 0x3002, 0x300a, 0x300b, 0x300c, 0x300d, 0x3000, 0x30fc, 0x3063, 0xff0c, 0xff1f}
	var b strings.Builder
	for i := range 1 + g.r.IntN(words) {
		if i > 0 {
			b.WriteByte(' ')
		}
		for range 1 + g.r.IntN(12) {
			switch g.r.IntN(4) {
			case 0:
				b.WriteByte(byte('a' + g.r.IntN(26)))
			case 1:
				b.WriteRune(rune(0x4e00 + g.r.IntN(0x5000)))
			case 2:
				b.WriteRune(rune(0x3041 + g.r.IntN(0x56)))
			default:
				b.WriteRune(cjk[g.r.IntN(len(cjk))])
			}
		}
	}
	return b.String()
}
