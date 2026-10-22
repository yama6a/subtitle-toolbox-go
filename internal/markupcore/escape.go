package markupcore

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

var (
	entityRegexp            = regexp.MustCompile(EntityPattern)
	entityOrAmpersandRegexp = regexp.MustCompile(EntityPattern + `|&`)

	escaper   = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;") //nolint:gochecknoglobals // a replacer holds no state
	unescaper = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&") //nolint:gochecknoglobals // a replacer holds no state
)

// EscapeText escapes &, < and > as &amp;, &lt; and &gt;. It keeps every other byte, also of invalid UTF-8.
func EscapeText(text string) string {
	return escaper.Replace(text)
}

// UnescapeText decodes &lt;, &gt; and &amp;, the entities that EscapeText writes, and keeps all other entities.
func UnescapeText(text string) string {
	return unescaper.Replace(text)
}

// EscapeTextLike escapes a changed text run as EscapeText does, but in the form of its raw run.
// It keeps & and > raw where raw has them raw, as WebVTT text does. An entity such as &nbsp; keeps the form
// that raw holds it in, also after a case change. Otherwise its & gets escaped.
func EscapeTextLike(text, raw string) string {
	bare := strings.Contains(entityRegexp.ReplaceAllString(raw, ""), "&")
	text = entityOrAmpersandRegexp.ReplaceAllStringFunc(text, func(m string) string {
		switch {
		case m != "&":
			if form, ok := rawEntity(m, raw); ok {
				return form
			}
			return "&amp;" + m[1:]
		case bare:
			return "&"
		default:
			return "&amp;"
		}
	})
	text = strings.ReplaceAll(text, "<", "&lt;")
	if strings.Contains(raw, ">") {
		return text
	}
	return strings.ReplaceAll(text, ">", "&gt;")
}

// rawEntity returns the form in which raw holds entity as an entity, such as &nbsp; for &NBSP;. It returns false
// when raw holds it as escaped text such as &amp;nbsp;. Decoded text holds &lt;, &gt; and &amp; only as text.
func rawEntity(entity, raw string) (string, bool) {
	lower := asciiLower(entity)
	if lower == "&lt;" || lower == "&gt;" || lower == "&amp;" || strings.Contains(asciiLower(raw), "&amp;"+lower[1:]) {
		return "", false
	}
	var forms []string
	for _, form := range entityRegexp.FindAllString(raw, -1) {
		if asciiLower(form) == lower && !slices.Contains(forms, form) {
			forms = append(forms, form)
		}
	}
	switch {
	case slices.Contains(forms, entity):
		return entity, true
	case len(forms) == 1:
		return forms[0], true
	}
	return "", false
}

// DecodeEntities decodes the entities of text as PHP html_entity_decode does with ENT_QUOTES and ENT_HTML5.
// A named entity needs its ";". A numeric entity decodes only to a code point that HTML5 allows in a document.
func DecodeEntities(text string) string {
	i := strings.IndexByte(text, '&')
	if i < 0 {
		return text
	}
	var b strings.Builder
	b.Grow(len(text))
	for i >= 0 {
		b.WriteString(text[:i])
		text = text[i:]
		if s, n, ok := decodeEntity(text); ok {
			b.WriteString(s)
			text = text[n:]
		} else {
			b.WriteByte('&')
			text = text[1:]
		}
		i = strings.IndexByte(text, '&')
	}
	b.WriteString(text)
	return b.String()
}

// decodeEntity decodes the entity at the start of s, which starts with "&". It returns the text and the entity length.
func decodeEntity(s string) (string, int, bool) {
	if strings.HasPrefix(s, "&#") {
		return decodeNumericEntity(s)
	}
	end := len(s) - len(strings.TrimLeft(s[1:], alnum))
	if end == 1 || !isASCIILetter(s[1]) || !strings.HasPrefix(s[end:], ";") {
		return "", 0, false
	}
	v, ok := entities()[s[1:end]]
	return v, end + 1, ok
}

const (
	decimalDigits = "0123456789"
	hexDigits     = decimalDigits + "abcdefABCDEF"
	alnum         = decimalDigits + "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

func decodeNumericEntity(s string) (string, int, bool) {
	digits, base, start := decimalDigits, 10, 2
	if len(s) > 2 && s[2]|0x20 == 'x' {
		digits, base, start = hexDigits, 16, 3+hexPrefixLength(s[3:])
	}
	end := len(s) - len(strings.TrimLeft(s[start:], digits))
	if end == start || !strings.HasPrefix(s[end:], ";") {
		return "", 0, false
	}
	cp, err := strconv.ParseUint(s[start:end], base, 32)
	if err != nil || cp > unicode.MaxRune || !allowedCodePoint(rune(cp)) {
		return "", 0, false
	}
	return string(rune(cp)), end + 1, true
}

// hexPrefixLength returns 2 when s starts with 0x and a hex digit. PHP reads hex digits with strtol, which skips
// such a prefix, so &#x0x41; decodes to A.
func hexPrefixLength(s string) int {
	if len(s) > 2 && s[0] == '0' && s[1]|0x20 == 'x' && strings.IndexByte(hexDigits, s[2]) >= 0 {
		return 2
	}
	return 0
}

// allowedCodePoint reports whether HTML5 allows r in a document, as PHP html_entity_decode checks with ENT_HTML5.
func allowedCodePoint(r rune) bool {
	switch {
	case r == '\t', r == '\n', r == '\f':
		return true
	case r < 0x20, 0x7f <= r && r < 0xa0, 0xd800 <= r && r < 0xe000, 0xfdd0 <= r && r <= 0xfdef:
		return false
	}
	return r&0xfffe != 0xfffe
}

// NamedEntity returns the text of the HTML5 named entity name, given without "&" and ";", such as "eacute".
func NamedEntity(name string) (string, bool) {
	v, ok := entities()[name]
	return v, ok
}
