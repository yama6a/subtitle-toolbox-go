package translation

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
)

var (
	tagRegexp              = regexp.MustCompile(`</?[a-zA-Z][^<>]*>`)
	tagNameRegexp          = regexp.MustCompile(`^</?([a-zA-Z][^\t\n\v\f\r />]*)`)
	exactPlaceholderRegexp = regexp.MustCompile(`</?x\d+/?>`)
	loosePlaceholderRegexp = regexp.MustCompile(`(?i)<[\t\n\v\f\r ]*/?[\t\n\v\f\r ]*x[\t\n\v\f\r ]*\d+[\t\n\v\f\r ]*/?[\t\n\v\f\r ]*>`)
)

// tagPair is the tag of one placeholder number. close is "" for a single tag.
type tagPair struct{ open, close string }

type tokenKind uint8

const (
	textToken tokenKind = iota
	openToken
	closeToken
	singleToken
)

// token is a text run with entities decoded, or a placeholder with its number.
type token struct {
	kind   tokenKind
	text   string
	number int
}

type tagRole uint8

const (
	roleSingle tagRole = iota
	roleOpen
	roleClose
)

// encode returns text with each tag replaced by a placeholder. tags[N-1] is the tag of placeholder N.
func encode(text string) (string, []tagPair) {
	spans := tagRegexp.FindAllStringIndex(text, -1)
	roles := make([]tagRole, len(spans))
	partner := make([]int, len(spans))
	open := map[string][]int{}
	for i, sp := range spans {
		tag := text[sp[0]:sp[1]]
		name := markupcore.ASCIILower(tagNameRegexp.FindStringSubmatch(tag)[1])
		switch {
		case strings.HasSuffix(tag, "/>"):
		case strings.HasPrefix(tag, "</"):
			stack := open[name]
			if len(stack) == 0 {
				break
			}
			j := stack[len(stack)-1]
			open[name] = stack[:len(stack)-1]
			roles[j], roles[i] = roleOpen, roleClose
			partner[j], partner[i] = i, j
		default:
			open[name] = append(open[name], i)
		}
	}

	var b strings.Builder
	var tags []tagPair
	numbers := make([]int, len(spans))
	pos := 0
	for i, sp := range spans {
		b.WriteString(markupcore.EscapeText(markupcore.DecodeEntities(text[pos:sp[0]])))
		pos = sp[1]
		tag := text[sp[0]:sp[1]]
		switch roles[i] {
		case roleSingle:
			tags = append(tags, tagPair{open: tag})
			b.WriteString("<x" + strconv.Itoa(len(tags)) + "/>")
		case roleOpen:
			closing := spans[partner[i]]
			tags = append(tags, tagPair{open: tag, close: text[closing[0]:closing[1]]})
			numbers[i] = len(tags)
			b.WriteString("<x" + strconv.Itoa(len(tags)) + ">")
		case roleClose:
			b.WriteString("</x" + strconv.Itoa(numbers[partner[i]]) + ">")
		}
	}
	b.WriteString(markupcore.EscapeText(markupcore.DecodeEntities(text[pos:])))
	return b.String(), tags
}

func expectedPlaceholders(tags []tagPair) []string {
	var want []string
	for i, t := range tags {
		n := strconv.Itoa(i + 1)
		if t.close == "" {
			want = append(want, "<x"+n+"/>")
		} else {
			want = append(want, "<x"+n+">", "</x"+n+">")
		}
	}
	return want
}

// keepsPlaceholders reports whether translation holds exactly the placeholders of tags, with each opening one before its closing one.
func keepsPlaceholders(translation string, tags []tagPair) bool {
	found := exactPlaceholderRegexp.FindAllString(translation, -1)
	if len(loosePlaceholderRegexp.FindAllStringIndex(translation, -1)) != len(found) {
		return false
	}
	want := expectedPlaceholders(tags)
	if len(found) != len(want) {
		return false
	}
	opened := map[int]bool{}
	for _, p := range found {
		kind, n := parsePlaceholder(p)
		switch kind {
		case openToken:
			opened[n] = true
		case closeToken:
			if !opened[n] {
				return false
			}
		case textToken, singleToken:
		}
	}
	slices.Sort(found)
	slices.Sort(want)
	return slices.Equal(found, want)
}

func removePlaceholders(translation string) string {
	return loosePlaceholderRegexp.ReplaceAllString(translation, "")
}

// parsePlaceholder reads an exact placeholder. A number too large for int gives 0, which no tag holds.
func parsePlaceholder(p string) (tokenKind, int) {
	kind := openToken
	switch {
	case strings.HasPrefix(p, "</"):
		kind = closeToken
	case strings.HasSuffix(p, "/>"):
		kind = singleToken
	}
	n, _ := strconv.Atoi(strings.Trim(p, "</>x"))
	return kind, n
}

// tokenize splits a translation that passed keepsPlaceholders or removePlaceholders.
func tokenize(translation string) []token {
	var tokens []token
	pos := 0
	addText := func(s string) {
		if s != "" {
			tokens = append(tokens, token{kind: textToken, text: markupcore.DecodeEntities(s)})
		}
	}
	for _, sp := range exactPlaceholderRegexp.FindAllStringIndex(translation, -1) {
		addText(translation[pos:sp[0]])
		pos = sp[1]
		kind, n := parsePlaceholder(translation[sp[0]:sp[1]])
		tokens = append(tokens, token{kind: kind, number: n})
	}
	addText(translation[pos:])
	return tokens
}

const placeholderSpace = " \t\n\v\f\r"

func appendText(out []token, s string) []token {
	if n := len(out); n > 0 && out[n-1].kind == textToken {
		out[n-1].text += s
		return out
	}
	return append(out, token{kind: textToken, text: s})
}

// emptyPairStart returns the index of the open token of number that the end of out holds with only white space
// after it, or -1.
func emptyPairStart(out []token, number int) int {
	n := len(out)
	if n > 0 && out[n-1].kind == openToken && out[n-1].number == number {
		return n - 1
	}
	if n > 1 && out[n-2].kind == openToken && out[n-2].number == number &&
		out[n-1].kind == textToken && strings.Trim(out[n-1].text, placeholderSpace) == "" {
		return n - 2
	}
	return -1
}

// dropEmptyPairs removes each open token that its close token follows with only white space between them.
// The white space stays.
func dropEmptyPairs(tokens []token) []token {
	out := make([]token, 0, len(tokens))
	for _, t := range tokens {
		switch {
		case t.kind == textToken:
			out = appendText(out, t.text)
		case t.kind != closeToken:
			out = append(out, t)
		default:
			start := emptyPairStart(out, t.number)
			if start < 0 {
				out = append(out, t)
				break
			}
			var space string
			if start+1 < len(out) {
				space = out[start+1].text
			}
			out = out[:start]
			if space != "" {
				out = appendText(out, space)
			}
		}
	}
	return out
}

// restore writes the tokens as text with the tags of tags.
func restore(tokens []token, tags []tagPair) string {
	var b strings.Builder
	for _, t := range dropEmptyPairs(tokens) {
		switch t.kind {
		case textToken:
			b.WriteString(markupcore.EscapeText(t.text))
		case openToken, singleToken:
			b.WriteString(tags[t.number-1].open)
		case closeToken:
			b.WriteString(tags[t.number-1].close)
		}
	}
	return b.String()
}
