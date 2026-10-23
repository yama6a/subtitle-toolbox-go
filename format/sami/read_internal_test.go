package sami

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

// phpTextLine is findTextLineWithoutCues of src/Parsers/SamiParser.php of subtitle-toolbox-php. Its regular
// expression takes quadratic time on many <HEAD tags without an end.
func phpTextLine(content string) (int, bool) {
	re := regexp.MustCompile(`(?s)<` + fold("head") + `\b.*?</` + fold("head") + `\s*>|<!--.*?-->|<[^>]*>|` + fold("&nbsp;"))
	stripped := re.ReplaceAllStringFunc(content, func(m string) string { return strings.Repeat("\n", strings.Count(m, "\n")) })
	text := strings.TrimLeft(stripped, markupcore.PHPTrimSet)
	if text == "" {
		return 0, false
	}
	return 1 + strings.Count(stripped[:len(stripped)-len(text)], "\n"), true
}

func TestTextLineMatchesPHP(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		"", "a", " \n\tb", "<HEAD>\n<TITLE>x</TITLE>\n</HEAD >\n\nc", "<HEADER>\nd", "<HEAD x\n</HEAD\n>\ne", "<HEAD>\n<HEAD>\nf",
		"<head>\n<!-- </head> -->\ng", "<!-- a\n", "<!--\n> h", "<a\n", "<a\n>\n<b>i", "&nbsp;&nbsp\nj", "&nbsp;\n&nbsp;",
		"<HEAD><HEAD></HEAD>\nk", "<!--<!-- -->\nl", "<\n>m<", "<HEAD\n", "x<HEAD>", "\x00\v <p>\n\n&amp;", "<>&nBsp;",
	} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			wantLine, wantOK := phpTextLine(in)
			gotLine, gotOK := (&reader{content: in}).textLine()
			assert.Equal(t, [2]any{wantLine, wantOK}, [2]any{gotLine, gotOK})
		})
	}
}

func FuzzTextLine(f *testing.F) {
	for _, seed := range []string{"<HEAD>\n</HEAD>\nx", "<!-- a\n", "<a\n>\n&nbsp;b", "<HEAD><HEAD></HEAD>\nk"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if anySyncRegexp.MatchString(in) {
			return
		}
		wantLine, wantOK := phpTextLine(in)
		gotLine, gotOK := (&reader{content: in}).textLine()
		if wantLine != gotLine || wantOK != gotOK {
			t.Fatalf("textLine(%q) = %d, %t, want %d, %t", in, gotLine, gotOK, wantLine, wantOK)
		}
	})
}

func TestStripCSSCommentsMatchesPHP(t *testing.T) {
	t.Parallel()

	re := regexp.MustCompile(`(?s)/\*.*?\*/|<!--|-->`)
	for _, in := range []string{
		"", "a", "/* x */b", "/* x\n*/ c /* y */", "/* open <!-- d -->", "<!--e-->", "/*/f", "/**/g", "--->h", "<!---->",
		"/* a */ /* b", "*/ i /*", "<!-- /* --> */ j",
	} {
		assert.Equal(t, re.ReplaceAllLiteralString(in, ""), stripCSSComments(in), in)
	}
}

func FuzzStripCSSComments(f *testing.F) {
	re := regexp.MustCompile(`(?s)/\*.*?\*/|<!--|-->`)
	for _, seed := range []string{"/* x */b", "/* open <!-- d -->", "<!-- /* --> */ j"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		if got, want := stripCSSComments(in), re.ReplaceAllLiteralString(in, ""); got != want {
			t.Fatalf("stripCSSComments(%q) = %q, want %q", in, got, want)
		}
	})
}
