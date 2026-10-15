package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

// CaseMode is the target case of ChangeCase.
type CaseMode string

// The case modes. CaseMode has no default, because ChangeCase requires one.
const (
	// CaseUpper writes upper case.
	CaseUpper CaseMode = "upper"
	// CaseLower writes lower case.
	CaseLower CaseMode = "lower"
	// CaseSentence writes lower case, with an upper case letter at the start of each sentence.
	CaseSentence CaseMode = "sentence"
)

// String returns "upper", "lower" or "sentence".
func (m CaseMode) String() string {
	return string(m)
}

const (
	dottedCapitalI = "\u0130"
	dotlessSmallI  = "\u0131"
)

//nolint:gochecknoglobals // a replacer holds no state
var (
	turkicUpper = strings.NewReplacer("i", dottedCapitalI)
	turkicLower = strings.NewReplacer(dottedCapitalI, "i", "I", dotlessSmallI)
)

// ChangeCase changes the case of the text runs between tags, with the full Unicode case mapping, so U+00DF
// becomes SS in upper case. Language is a BCP 47 code such as "tr-TR", or "" for none. The primary subtags tr and
// az turn on the Turkic rules for i, U+0130, I and U+0131. In sentence case, "" and en also write the English
// pronoun I in upper case. A run of invalid UTF-8 changes its ASCII letters only. ChangeCase returns an error
// that matches ErrInvalidArgument for a mode other than the 3 constants.
func (s *Subtitle) ChangeCase(mode CaseMode, language string) error {
	primary := primaryLanguage(language)
	turkic := primary == "tr" || primary == "az"
	switch mode {
	case CaseUpper:
		s.mapRuns(func(text string, _ Cue, _ bool) string { return upperCase(text, turkic) })
	case CaseLower:
		s.mapRuns(func(text string, _ Cue, _ bool) string { return lowerCase(text, turkic) })
	case CaseSentence:
		s.sentenceCase(turkic, language == "" || primary == "en")
	default:
		return invalidArgument(fmt.Sprintf("The case mode %s is not upper, lower or sentence.", quote(string(mode))))
	}
	return nil
}

func (s *Subtitle) sentenceCase(turkic, english bool) {
	sc := &sentenceCaser{turkic: turkic, endsSentence: true, capitalizeNext: true}
	s.EditLines(func(_ int, c Cue) ([]string, bool) {
		started := false
		lines := markupcore.MapTextRuns(c.Lines, func(text string, first, _ bool) string {
			if !started {
				sc.startCue(c)
				started = true
			}
			return sc.run(text, first)
		})
		if english {
			for i, line := range lines {
				lines[i] = upperCaseEnglishI(line)
			}
		}
		return lines, true
	}, nil)
}

// upperCaseEnglishI runs markupcore.UpperCaseEnglishI on the decoded text runs of line.
func upperCaseEnglishI(line string) string {
	var runs []string
	markupcore.MapTextRuns([]string{line}, func(text string, _, _ bool) string {
		runs = append(runs, text)
		return text
	})
	upper := markupcore.UpperCaseEnglishI(runs, true)
	k := 0
	return markupcore.MapTextRuns([]string{line}, func(string, bool, bool) string {
		k++
		return upper[k-1]
	})[0]
}

// primaryLanguage returns the part of a BCP 47 code before the first "-" or "_", in lower case.
func primaryLanguage(code string) string {
	if i := strings.IndexAny(code, "-_"); i >= 0 {
		code = code[:i]
	}
	return asciiLower(code)
}

func upperCase(text string, turkic bool) string {
	if !utf8.ValidString(text) {
		return asciiUpper(text)
	}
	if turkic {
		text = turkicUpper.Replace(text)
	}
	return cases.Upper(language.Und).String(text)
}

func lowerCase(text string, turkic bool) string {
	if !utf8.ValidString(text) {
		return asciiLower(text)
	}
	if turkic {
		text = turkicLower.Replace(text)
	}
	return cases.Lower(language.Und).String(text)
}

// titleCase writes one character in title case, so U+00DF becomes Ss. A byte of invalid UTF-8 changes only when
// it is an ASCII letter.
func titleCase(char string, turkic bool) string {
	if !utf8.ValidString(char) {
		return asciiUpper(char)
	}
	if turkic && char == "i" {
		return dottedCapitalI
	}
	return cases.Title(language.Und).String(char)
}

// asciiLower lowers A to Z only, as PHP strtolower does. It keeps every other byte, also of invalid UTF-8.
func asciiLower(s string) string {
	return asciiMap(s, 'A', 'Z', 'a'-'A')
}

// asciiUpper raises a to z only, as PHP strtoupper does. It keeps every other byte, also of invalid UTF-8.
func asciiUpper(s string) string {
	return asciiMap(s, 'a', 'z', 'A'-'a')
}

func asciiMap(s string, from, to byte, delta int) string {
	b := []byte(s)
	for i, c := range b {
		if from <= c && c <= to {
			b[i] = byte(int(c) + delta)
		}
	}
	return string(b)
}
