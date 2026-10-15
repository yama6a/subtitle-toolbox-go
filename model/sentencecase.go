package model

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

// sentenceGap is the gap after the previous cue from which a cue starts a sentence.
const sentenceGap = 2 * time.Second

const (
	ellipsis  = 0x2026
	musicNote = 0x266A
	// asciiSpaces holds the white space that ends a sentence after its punctuation.
	asciiSpaces = " \t\n\r\v\f"
)

// sentenceCaser writes the text runs of the cues in sentence case, in cue order. One value holds the state of one
// pass over a subtitle.
type sentenceCaser struct {
	turkic bool

	started          bool
	previousEnd      time.Duration
	endsSentence     bool
	capitalizeNext   bool
	afterPunctuation bool
	// abbreviation marks a period after i.e, e.g, etc or vs. A lower case word after it continues the sentence.
	abbreviation bool
	// abbreviationSpace marks a space after an abbreviation at the end of a run, so the first letter of the next
	// run decides.
	abbreviationSpace bool
}

func (sc *sentenceCaser) startCue(c Cue) {
	visible := markupcore.VisibleText(c.Text())
	sc.capitalizeNext = sc.endsSentence || startsAnnotation(visible) || sc.started && c.Start-sc.previousEnd >= sentenceGap
	sc.afterPunctuation = false
	sc.started = true
	sc.previousEnd = c.End
	sc.endsSentence = endsSentence(visible)
}

// run returns the next text run in sentence case. First marks the first run of a line.
func (sc *sentenceCaser) run(text string, first bool) string {
	if first {
		sc.abbreviation = false
		sc.abbreviationSpace = false
		if sc.afterPunctuation || startsTurn(text) {
			sc.capitalizeNext = true
		}
	}
	return sc.caseRun(text)
}

// caseRun lowers a text run and writes its first letter or digit after the start of a sentence in title case.
func (sc *sentenceCaser) caseRun(text string) string {
	chars := markupcore.Characters(lowerCase(text, sc.turkic))
	original := markupcore.Characters(text)
	// Lowering U+0130 adds a combining dot, so the original case is known only when the counts match.
	if len(chars) != len(original) {
		original = nil
	}
	var word, b strings.Builder
	for i, char := range chars {
		switch {
		case isLetterOrDigit(char):
			char = sc.letter(char, original, i)
			word.WriteString(char)
		case isSentencePunctuation(char):
			sc.punctuation(char, &word)
		default:
			word.Reset()
			sc.other(char, original, i)
		}
		b.WriteString(char)
	}
	return b.String()
}

// letter returns char, in title case at the start of a sentence. Original holds the characters of the run before
// lowering, or nil.
func (sc *sentenceCaser) letter(char string, original []string, i int) string {
	keepsLowerCase := sc.abbreviationSpace && original != nil && isLowerLetter(original[i])
	if sc.capitalizeNext && !keepsLowerCase {
		char = titleCase(char, sc.turkic)
	}
	sc.capitalizeNext = false
	sc.afterPunctuation = false
	sc.abbreviationSpace = false
	return char
}

// punctuation handles a sentence punctuation char after word. A dot inside a word such as e.g stays in the word.
func (sc *sentenceCaser) punctuation(char string, word *strings.Builder) {
	sc.afterPunctuation = true
	sc.abbreviationSpace = false
	sc.abbreviation = char == "." && isAbbreviation(word.String())
	if char == "." && word.Len() > 0 {
		word.WriteString(char)
		return
	}
	word.Reset()
}

// other handles a character that is no letter, digit or sentence punctuation. A space after the punctuation starts
// a sentence, except after an abbreviation that a lower case word follows.
func (sc *sentenceCaser) other(char string, original []string, i int) {
	space := len(char) == 1 && strings.Contains(asciiSpaces, char)
	if !space {
		sc.abbreviationSpace = false
	}
	switch {
	case sc.afterPunctuation && char == ",":
		sc.afterPunctuation = false
	case sc.afterPunctuation && space:
		sc.spaceAfterPunctuation(original, i)
	}
}

func (sc *sentenceCaser) spaceAfterPunctuation(original []string, i int) {
	lower, found := false, true
	if original != nil {
		lower, found = lowerCaseFollows(original[i+1:])
	}
	sc.abbreviationSpace = sc.abbreviation && !found
	sc.afterPunctuation = !sc.abbreviation || !lower
	sc.capitalizeNext = sc.capitalizeNext || sc.afterPunctuation
}

// lowerCaseFollows reports whether the first letter in chars is in lower case. Found is false when only ASCII
// white space follows.
func lowerCaseFollows(chars []string) (lower, found bool) {
	for _, char := range chars {
		if r, ok := singleRune(char); ok && unicode.IsLetter(r) {
			return unicode.IsLower(r), true
		}
		if len(char) != 1 || !strings.Contains(asciiSpaces, char) {
			return false, true
		}
	}
	return false, false
}

func isAbbreviation(word string) bool {
	if len(word) > len("etc") {
		return false
	}
	switch asciiLower(word) {
	case "i.e", "e.g", "etc", "vs":
		return true
	}
	return false
}

func isSentencePunctuation(char string) bool {
	r, ok := singleRune(char)
	return ok && (r == '.' || r == '!' || r == '?' || r == ellipsis)
}

func isLetterOrDigit(char string) bool {
	r, ok := singleRune(char)
	return ok && (unicode.IsLetter(r) || unicode.IsNumber(r))
}

func isLowerLetter(char string) bool {
	r, ok := singleRune(char)
	return ok && unicode.IsLower(r)
}

// singleRune returns the rune of a character of markupcore.Characters, or false for a byte of invalid UTF-8.
func singleRune(char string) (rune, bool) {
	r, size := utf8.DecodeRuneInString(char)
	return r, size == len(char) && r != utf8.RuneError
}

// startsTurn reports whether text starts with the CEA-608 speaker change ">>" or a dialogue dash.
func startsTurn(text string) bool {
	text = strings.TrimLeft(text, phpTrimSet)
	return strings.HasPrefix(text, ">>") || markupcore.StartsDialogueTurn(text)
}

// startsAnnotation reports whether text starts with a sound description or a music note.
func startsAnnotation(text string) bool {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeft(text, phpTrimSet))
	return r == '[' || r == '(' || r == musicNote
}

// endsSentence reports whether text ends a sentence, with optional closing marks after the punctuation, or ends
// with a sound description, a music note or a speaker label. Invalid UTF-8 knows only ASCII closing marks.
func endsSentence(text string) bool {
	if !utf8.ValidString(text) {
		return endsSentenceASCII(text)
	}
	if r, _ := utf8.DecodeLastRuneInString(strings.TrimRightFunc(text, isClosingMark)); isSentencePunctuation(string(r)) {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text)
	return r == ']' || r == ')' || r == musicNote || r == ':'
}

func isClosingMark(r rune) bool {
	return r == '"' || r == '\'' || unicode.In(r, unicode.Pe, unicode.Pi, unicode.Pf)
}

func endsSentenceASCII(text string) bool {
	if trimmed := strings.TrimRight(text, `)]}"'`); trimmed != "" && strings.ContainsRune(".!?", rune(trimmed[len(trimmed)-1])) {
		return true
	}
	if text != "" && strings.ContainsRune("]):", rune(text[len(text)-1])) {
		return true
	}
	return strings.HasSuffix(text, string(rune(musicNote)))
}
