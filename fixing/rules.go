package fixing

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/dlclark/regexp2/v2"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

// The patterns use the syntax of regexp2. space is \s of PCRE, which also matches U+180E.
const (
	dashClass      = `[\-\u2010\u2013\u2014]`
	spaceClass     = `[ \t\u00a0]`
	space          = `[\s\u180e]`
	wordClass      = `[\p{L}\p{N}'\u2019]`
	notInWord      = `(?<!` + wordClass + `)`
	wordEnds       = `(?!` + wordClass + `)`
	tags           = `(?:` + markupcore.TagPattern + `)*`
	edge           = `(?:` + markupcore.TagPattern + `|` + space + `)*`
	lStart         = notInWord + `(?<!\p{N} )`
	punctuationEnd = `(?:[\s\u180e"'\u2019\u00bb)\]]|[,.!?;:]|$)`
	musicNote      = "\u266a"
)

// matchTimeout limits each regexp2 match. The patterns are fixed and never backtrack without end.
const matchTimeout = time.Second

//nolint:gochecknoglobals // compiled patterns and lookup tables hold no state
var (
	adSites = compile(`(?<![\p{L}\p{N}])(?:opensubtitles|open-subtitles|osdb|addic7ed|yify|yts|subscene|podnapisi\.net)(?!\p{L})`,
		regexp2.IgnoreCase)
	lInWord  = compile(notInWord+`[\p{L}'\u2019]*l[\p{L}'\u2019]*`+wordEnds, regexp2.None)
	lEnglish = []*regexp2.Regexp{
		compile(lStart+`l(?=['\u2019](?:m|ll|ve|d)`+wordEnds+`)`, regexp2.None),
		compile(lStart+`(?<![^\s\u180e"'(\[\u00bf\u00a1-])l(?=$|[\s\u180e.,!?;:"')\]\u2026-])`, regexp2.None),
	}
	lConsonant = map[string]*regexp2.Regexp{
		"en": compile(lStart+`(?!l(?:bs?|td)`+wordEnds+`)l(?=[bcdfghjkmnpqrstvwxz])`, regexp2.None),
		"de": compile(lStart+`l(?=[bcdfghjkmnpqrstvwxz])`, regexp2.None),
		"es": compile(lStart+`l(?=[bcdfghjkmnpqrstvwxz])`, regexp2.None),
		"fr": compile(lStart+`l(?=[bcdfghjklmnpqrstvwxz])`, regexp2.None),
	}
	unneededPeriod     = compile(`(?<=[?!])\.(?![.\u2026])`, regexp2.None)
	spaceBeforeMarks   = spaceBeforePattern(`(?:(?:[,!?]|\.(?!\.))` + punctuationEnd + `|[;:](?:` + space + `|$))`)
	spaceBeforeFrench  = spaceBeforePattern(`(?:,|\.(?!\.))` + punctuationEnd)
	missingSpaceAfter  = compile(`(?<=\p{L})([,;]|[!?]+)(?=[\p{L}\u00bf\u00a1])`, regexp2.None)
	missingSpacePeriod = compile(`(?<=\p{L}\p{L})\.(?=\p{Lu}\p{Ll})`, regexp2.None)
	missingSpaceDots   = compile(`(?<=\p{L})(\.\.\.|\u2026)(?=\p{L})`, regexp2.None)
	dialogueSplit      = compile(`[.?!\u2026\u3002\uff01\uff1f\uff61]["'\u2019\u201d)\]]*`+tags+`(`+spaceClass+`+)(?=`+tags+
		dashClass+`(?!`+dashClass+`)`+spaceClass+`*[^\s\u180e\p{N}])`, regexp2.None)
	musicStart = compile(`^(`+edge+`(?:`+dashClass+space+`*)?)[#*](?=`+space+`|`+edge+`$)`, regexp2.None)
	musicEnd   = compile(`(?<=`+space+`)[#*](`+edge+`)$`, regexp2.None)

	zeroWord      = regexp.MustCompile(`[\p{L}\p{N}'\x{2019}]*0[\p{L}\p{N}'\x{2019}]*`)
	dots          = regexp.MustCompile(`\.(?: ?\.){2,}`)
	dotsOrUnicode = regexp.MustCompile(`\.(?: ?\.){2,}|\x{2026}`)
	spaces        = regexp.MustCompile(`[ \t\x{00A0}]{2,}`)
	apostrophes   = strings.NewReplacer("''", `"`, "\u2019\u2019", `"`)
	styleTags     = []string{"b", "i", "u", "s", "font"}
)

func spaceBeforePattern(marks string) *regexp2.Regexp {
	return compile(`(?<=[\p{L}\p{N}"'\u2019\u00bb)\]])`+spaceClass+`+(?=`+marks+`)`, regexp2.None)
}

func compile(pattern string, opts regexp2.RegexOptions) *regexp2.Regexp {
	re := regexp2.MustCompile(pattern, opts)
	re.MatchTimeout = matchTimeout
	return re
}

// replace returns text unchanged when it is not valid UTF-8, as PHP does for a pattern with the u flag.
func replace(re *regexp2.Regexp, text, replacement string) string {
	if !utf8.ValidString(text) {
		return text
	}
	out, err := re.Replace(text, replacement, -1, -1)
	if err != nil {
		return text
	}
	return out
}

func replaceFunc(re *regexp2.Regexp, text string, fn func(string) string) string {
	if !utf8.ValidString(text) {
		return text
	}
	out, err := re.ReplaceFunc(text, func(m regexp2.Match) string { return fn(m.String()) }, -1, -1)
	if err != nil {
		return text
	}
	return out
}

// fixes holds the fix of each rule. A fix returns lines, or a new slice when it changes a line.
//
//nolint:gochecknoglobals // a lookup table of the rules
var fixes = map[Rule]func(f *fixer, lines []string) []string{
	RuleAdCues:                       (*fixer).adCues,
	RuleUnbalancedTags:               (*fixer).unbalancedTags,
	RuleEmptyTags:                    (*fixer).emptyTags,
	RuleDoubleApostrophes:            (*fixer).doubleApostrophes,
	RuleOCRPipe:                      (*fixer).ocrPipe,
	RuleOCRZeroInWords:               (*fixer).ocrZero,
	RuleOCRLowercaseL:                (*fixer).ocrLowercaseL,
	RuleLoneLowercaseI:               (*fixer).loneLowercaseI,
	RuleEllipsis:                     (*fixer).ellipsis,
	RuleUnneededPeriods:              (*fixer).unneededPeriods,
	RuleDoubleSpaces:                 (*fixer).doubleSpaces,
	RuleSpaceBeforePunctuation:       (*fixer).spaceBeforePunctuation,
	RuleMissingSpaceAfterPunctuation: (*fixer).missingSpaceAfterPunctuation,
	RuleDialogueOnOneLine:            (*fixer).dialogueOnOneLine,
	RuleDialogueDashes:               (*fixer).dialogueDashes,
	RuleMusicNotes:                   (*fixer).musicNotes,
	RuleSentenceStartCase:            (*fixer).sentenceStartCase,
}

func mapRuns(lines []string, fn func(text string, first bool) string) []string {
	return markupcore.MapTextRuns(lines, func(text string, first, _ bool) string { return fn(text, first) })
}

func mapLines(lines []string, fn func(line string) string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = fn(line)
	}
	return out
}

func (*fixer) adCues(lines []string) []string {
	visible := markupcore.VisibleText(strings.Join(lines, "\n"))
	if !utf8.ValidString(visible) {
		return lines
	}
	if ok, err := adSites.MatchString(visible); ok && err == nil {
		return nil
	}
	return lines
}

func (*fixer) unbalancedTags(lines []string) []string {
	stray, open := markupcore.UnbalancedTags(lines, styleTags)
	if len(stray) == 0 && len(open) == 0 {
		return lines
	}
	out := slices.Clone(lines)
	for _, t := range slices.Backward(stray) {
		line := out[t.Line]
		out[t.Line] = line[:t.Offset] + line[t.Offset+t.Length:]
	}
	for _, name := range slices.Backward(open) {
		out[len(out)-1] += "</" + name + ">"
	}
	return out
}

func (*fixer) emptyTags(lines []string) []string {
	return mapLines(lines, func(line string) string { return markupcore.RemoveEmptyTagPairs(line, true, true) })
}

func (*fixer) doubleApostrophes(lines []string) []string {
	return mapRuns(lines, func(text string, _ bool) string {
		if !utf8.ValidString(text) {
			return text
		}
		return apostrophes.Replace(text)
	})
}

// endsWith reports whether the last rune of s matches is. It is false for invalid UTF-8.
func endsWith(s string, is func(rune) bool) bool {
	r, _ := utf8.DecodeLastRuneInString(s)
	return s != "" && utf8.ValidString(s) && is(r)
}

func startsWith(s string, is func(rune) bool) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return s != "" && utf8.ValidString(s) && is(r)
}

func (f *fixer) ocrPipe(lines []string) []string {
	return mapRuns(lines, func(text string, _ bool) string {
		b := []byte(text)
		for i, c := range b {
			if c == '|' {
				b[i] = pipeLetter(string(b[:i]), string(b[i+1:]), f.language)
			}
		}
		return string(b)
	})
}

// pipeLetter returns the letter that a pipe between before and after stands for, or the pipe.
func pipeLetter(before, after, lang string) byte {
	switch {
	case endsWith(before, unicode.IsLower):
		return 'l'
	case endsWith(before, unicode.IsLetter) || startsWith(after, unicode.IsLetter):
		return 'I'
	case apostropheBeforeLetter(after):
		switch lang {
		case "en":
			return 'I'
		case "fr":
			return 'l'
		}
	case lang == "en" && pipeStandsAlone(before, after):
		return 'I'
	}
	return '|'
}

func apostropheBeforeLetter(s string) bool {
	r, n := utf8.DecodeRuneInString(s)
	return utf8.ValidString(s) && isApostrophe(r) && startsWith(s[n:], unicode.IsLetter)
}

func pipeStandsAlone(before, after string) bool {
	if before != "" && !endsWith(before, isPipeLeft) || !utf8.ValidString(after) {
		return false
	}
	return after == "" || startsWith(after, isPipeRight)
}

func isApostrophe(r rune) bool { return r == '\'' || r == '\u2019' }

func isPipeLeft(r rune) bool {
	return markupcore.IsSpace(r) || r == '"' || r == '\'' || r == '(' || markupcore.IsDialogueDash(r)
}

func isPipeRight(r rune) bool {
	return markupcore.IsSpace(r) || strings.ContainsRune(`.,!?;:"`, r)
}

func isOpening(r rune) bool {
	return markupcore.IsSpace(r) || r == '"' || r == '\'' || r == '\u00bf' || r == '\u00a1' || markupcore.IsDialogueDash(r)
}

func (*fixer) ocrZero(lines []string) []string {
	return mapRuns(lines, func(text string, startsLine bool) string {
		if !utf8.ValidString(text) || !strings.Contains(text, "0") {
			return text
		}
		var b strings.Builder
		copied := 0
		for _, loc := range zeroWord.FindAllStringIndex(text, -1) {
			fixed, ok := fixZeroWord(text[loc[0]:loc[1]], startsLine && startsSentenceAt(text[:loc[0]], true) ||
				startsSentenceAt(text[:loc[0]], false))
			if ok {
				b.WriteString(text[copied:loc[0]])
				b.WriteString(fixed)
				copied = loc[1]
			}
		}
		if copied == 0 {
			return text
		}
		b.WriteString(text[copied:])
		return b.String()
	})
}

// fixZeroWord replaces each 0 of a word of 2 or more letters and no other digit with o, or with O when the word
// has no lower case letter or the 0 starts a sentence.
func fixZeroWord(word string, startsSentence bool) (string, bool) {
	digits, letters, lower := 0, 0, false
	for _, r := range word {
		switch {
		case unicode.IsNumber(r):
			digits++
		case unicode.IsLetter(r):
			letters++
			lower = lower || unicode.IsLower(r)
		}
	}
	if digits != strings.Count(word, "0") || letters < 2 {
		return word, false
	}
	zero := "O"
	if lower {
		zero = "o"
	}
	fixed := strings.ReplaceAll(word, "0", zero)
	if word[0] == '0' && startsSentence {
		fixed = "O" + fixed[1:]
	}
	return fixed, true
}

// startsSentenceAt reports whether a word after before starts a sentence. With startsLine, only opening characters
// come before it. Without, before ends with a sentence end, white space and opening characters.
func startsSentenceAt(before string, startsLine bool) bool {
	head := strings.TrimRightFunc(before, isOpening)
	if startsLine {
		return head == ""
	}
	next, _ := utf8.DecodeRuneInString(before[len(head):])
	return head != "" && strings.ContainsAny(head[len(head)-1:], ".!?") && markupcore.IsSpace(next)
}

func (f *fixer) ocrLowercaseL(lines []string) []string {
	consonant, hasRules := lConsonant[f.language]
	return mapRuns(lines, func(text string, _ bool) string {
		text = replaceFunc(lInWord, text, upperCaseLWord)
		if !hasRules {
			return text
		}
		if f.language == "en" {
			for _, re := range lEnglish {
				text = replace(re, text, "I")
			}
		}
		return replace(consonant, text, "I")
	})
}

// upperCaseLWord writes each l of a word as I when l is its only lower case letter and the word has 2 upper case
// letters or an l before an upper case letter.
func upperCaseLWord(word string) string {
	upper := 0
	previousL, lBeforeUpper := false, false
	for _, r := range word {
		if r != 'l' && unicode.IsLower(r) {
			return word
		}
		if unicode.IsUpper(r) {
			upper++
			lBeforeUpper = lBeforeUpper || previousL
		}
		previousL = r == 'l'
	}
	if upper < 2 && !lBeforeUpper {
		return word
	}
	return strings.ReplaceAll(word, "l", "I")
}

func (*fixer) loneLowercaseI(lines []string) []string {
	return mapLines(lines, func(line string) string {
		parts := markupcore.SplitTags(line)
		var runs []string
		for j := 0; j < len(parts); j += 2 {
			runs = append(runs, parts[j])
		}
		for k, run := range markupcore.UpperCaseEnglishI(runs, false) {
			parts[2*k] = run
		}
		return strings.Join(parts, "")
	})
}

func (f *fixer) ellipsis(lines []string) []string {
	re, ellipsis := dots, "..."
	if f.opts.Ellipsis == EllipsisUnicode {
		re, ellipsis = dotsOrUnicode, "\u2026"
	}
	return mapRuns(lines, func(text string, _ bool) string {
		if !utf8.ValidString(text) {
			return text
		}
		return re.ReplaceAllLiteralString(text, ellipsis)
	})
}

func (*fixer) unneededPeriods(lines []string) []string {
	return mapRuns(lines, func(text string, _ bool) string { return replace(unneededPeriod, text, "") })
}

func isSpaceChar(r rune) bool { return r == ' ' || r == '\t' || r == '\u00a0' }

func (*fixer) doubleSpaces(lines []string) []string {
	previousSpace := false
	return mapRuns(lines, func(text string, first bool) string {
		if !utf8.ValidString(text) {
			previousSpace = false
			return text
		}
		text = spaces.ReplaceAllLiteralString(text, " ")
		if !first && previousSpace {
			text = strings.TrimLeftFunc(text, isSpaceChar)
		}
		previousSpace = endsWith(text, isSpaceChar) || previousSpace && text == ""
		return text
	})
}

func (f *fixer) spaceBeforePunctuation(lines []string) []string {
	re := spaceBeforeMarks
	if f.language == "fr" {
		re = spaceBeforeFrench
	}
	return mapRuns(lines, func(text string, _ bool) string { return replace(re, text, "") })
}

func (*fixer) missingSpaceAfterPunctuation(lines []string) []string {
	return mapRuns(lines, func(text string, _ bool) string {
		text = replace(missingSpaceAfter, text, "$1 ")
		text = replace(missingSpacePeriod, text, ". ")
		return replace(missingSpaceDots, text, "$1 ")
	})
}

func (f *fixer) dialogueOnOneLine(lines []string) []string {
	if len(lines) > 2 {
		return lines
	}
	joined := strings.Join(lines, " ")
	offset, length, ok := dialogueSplitAt(joined)
	if !ok || len(lines) == 2 && offset == len(lines[0]) {
		return lines
	}

	first := joined[:offset]
	open := markupcore.OpenCoreTags(first, nil)
	var reopen strings.Builder
	for _, t := range open {
		reopen.WriteString(t.Tag)
	}
	split := []string{first + markupcore.CloseCoreTags(open), reopen.String() + joined[offset+length:]}
	style := f.opts.DialogueDashStyle.Text()
	return mapRuns(split, func(text string, isFirst bool) string {
		if !isFirst || startsWith(strings.TrimLeftFunc(text, markupcore.IsSpace), markupcore.IsDialogueDash) {
			return text
		}
		return style + strings.TrimLeft(text, markupcore.PHPTrimSet)
	})
}

// dialogueSplitAt returns the spaces before the one dialogue dash after a sentence end in text.
func dialogueSplitAt(text string) (offset, length int, ok bool) {
	if !utf8.ValidString(text) {
		return 0, 0, false
	}
	m, err := dialogueSplit.FindStringMatch(text)
	if err != nil || m == nil {
		return 0, 0, false
	}
	offset, length = m.GroupByNumber(1).ByteRange()
	if next, err := dialogueSplit.FindNextMatch(m); err != nil || next != nil {
		return 0, 0, false
	}
	return offset, length, true
}

func (f *fixer) dialogueDashes(lines []string) []string {
	style := f.opts.DialogueDashStyle.Text()
	return mapRuns(lines, func(text string, first bool) string {
		controls, rest := markupcore.SplitLeadingBidiControls(text)
		if !first || !markupcore.StartsDialogueTurn(rest) {
			return text
		}
		_, n := utf8.DecodeRuneInString(rest)
		return controls + style + strings.TrimLeftFunc(rest[n:], isSpaceChar)
	})
}

func (*fixer) musicNotes(lines []string) []string {
	return mapLines(lines, func(line string) string {
		return replace(musicEnd, replace(musicStart, line, "$1"+musicNote), musicNote+"$1")
	})
}

// endsSentence reports whether text ends with a period that does not follow another period, a question mark or an
// exclamation mark, then optional closing punctuation and quotes. So "..." ends no sentence.
func endsSentence(text string) bool {
	if !utf8.ValidString(text) {
		return false
	}
	t := strings.TrimRightFunc(text, func(r rune) bool {
		return unicode.In(r, unicode.Pe, unicode.Pf) || r == '"' || r == '\''
	})
	switch {
	case strings.HasSuffix(t, "?"), strings.HasSuffix(t, "!"):
		return true
	case strings.HasSuffix(t, "."):
		return !strings.HasSuffix(t, "..")
	}
	return false
}

func (f *fixer) sentenceStartCase(lines []string) []string {
	out := slices.Clone(lines)
	for i, line := range out {
		starts := f.startsSentence
		if i > 0 {
			starts = endsSentence(markupcore.VisibleText(out[i-1]))
		}
		if starts {
			out[i] = f.capitalizeLine(line)
		}
	}
	return out
}

func isSentenceOpening(r rune) bool {
	switch r {
	case '"', '\'', '\u2018', '\u201c', '\u00ab', '\u00bf', '\u00a1', '\u266a':
		return true
	}
	return markupcore.IsSpace(r) || markupcore.IsDialogueDash(r)
}

// capitalizeLine capitalizes the first run that holds more than opening characters.
func (f *fixer) capitalizeLine(line string) string {
	done := false
	return mapRuns([]string{line}, func(text string, _ bool) string {
		valid := utf8.ValidString(text)
		if done || valid && strings.TrimLeftFunc(text, isSentenceOpening) == "" {
			return text
		}
		done = true
		if !valid {
			return text
		}
		return f.capitalizeRun(text)
	})[0]
}

// capitalizeRun writes the first letter after the opening characters in upper case. A word with an upper case
// letter after the first, such as iPhone, stays.
func (f *fixer) capitalizeRun(text string) string {
	rest := strings.TrimLeftFunc(text, isSentenceOpening)
	letter, n := utf8.DecodeRuneInString(rest)
	if !unicode.IsLower(letter) {
		return text
	}
	for _, r := range rest[n:] {
		if !unicode.IsLetter(r) && !unicode.IsMark(r) && !unicode.IsNumber(r) && !isApostrophe(r) {
			break
		}
		if unicode.IsUpper(r) {
			return text
		}
	}
	upper := cases.Upper(language.Und).String(string(letter))
	if f.turkic && letter == 'i' {
		upper = "\u0130"
	}
	return text[:len(text)-len(rest)] + upper + rest[n:]
}
