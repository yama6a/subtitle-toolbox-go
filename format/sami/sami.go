// Package sami reads and writes SAMI, the HTML-like caption format of Windows Media Player.
//
// A SAMI file can hold several languages, one CSS class each. The reader reads one class, the writer writes one.
// A tag scanner of this package reads the content of each SYNC block in place of an HTML parser.
package sami

import (
	"regexp"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	formatDataKey = string(model.SAMI)
	phpTrimSet    = " \t\n\r\x00\v"
	nbsp          = "\u00A0"
)

// ReadOptions holds the settings of the SAMI reader.
type ReadOptions struct {
	// LanguageClass is the class to read, such as "ENUSCC", in any case.
	// Empty picks the first class of the STYLE block that a <P> uses.
	LanguageClass string
}

// Validate reports a LanguageClass of only white space.
func (o ReadOptions) Validate() error {
	if o.LanguageClass != "" && strings.Trim(o.LanguageClass, phpTrimSet) == "" {
		return &model.InvalidArgumentError{
			Option: "ReadOptions.SAMI.LanguageClass",
			Msg:    "The language class must not be empty.",
		}
	}
	return nil
}

// WriteOptions holds the settings of the SAMI writer. It has no fields yet.
type WriteOptions struct{}

// signatureRegexp matches the XML prolog, then <SAMI and white space or >, in any case. It does not use
// xmlraw.Prolog, because the PHP signature of SAMI also reads <!doctype in lower case and that of TTML does not.
var signatureRegexp = regexp.MustCompile(`\A(?s:[\t\n\v\f\r ]|<\?.*?\?>|<!--.*?-->|<!` + fold("doctype") + `[^>]*>)*<` +
	fold("sami") + `[\t\n\v\f\r >]`)

// Detect reports whether content has the SAMI signature of stbox.Detect: the XML prolog, then <SAMI and white space
// or >, in any case. The content must have no leading BOM or white space and LF line endings, as stbox.Detect passes it.
func Detect(content []byte) bool {
	return signatureRegexp.Match(content)
}

// fold makes the ASCII letters of a pattern match in any case. The (?i) flag of regexp also matches U+017F for s and
// U+212A for k, which PCRE without the u flag does not.
func fold(pattern string) string {
	var b strings.Builder
	for i := range len(pattern) {
		c := pattern[i]
		switch {
		case 'a' <= c && c <= 'z':
			b.WriteString("[" + string(c-'a'+'A') + string(c) + "]")
		case 'A' <= c && c <= 'Z':
			b.WriteString("[" + string(c) + string(c-'A'+'a') + "]")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func asciiLower(s string) string {
	return strings.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}
		return r
	}, s)
}

func asciiUpper(s string) string {
	return strings.Map(func(r rune) rune {
		if 'a' <= r && r <= 'z' {
			return r - 'a' + 'A'
		}
		return r
	}, s)
}
