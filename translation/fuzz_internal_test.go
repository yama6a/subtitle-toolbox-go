package translation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
)

func FuzzPlaceholders(f *testing.F) {
	for _, text := range []string{
		"<i>Run, <b>now</b>!</i>",
		"<v Fred>Hi <br/>there.",
		"<b>Tom &amp; Jerry</b> sagen a &lt; b",
		"<b><i>x</b></i>",
		"<v Fred>Hi",
		"&lt;x1&gt;",
		"<x1>",
		"<x<x1/>1>Hallo",
		"<i> </i>a<b><i></i></b>",
		"a b c d e f g",
		"\xff<i>\xfe</i>",
	} {
		f.Add(text, uint64(0x0102030405))
	}

	f.Fuzz(checkPlaceholders)
}

func checkPlaceholders(t *testing.T, text string, seed uint64) {
	weights := make([]int, 1+seed%5)
	for i := range weights {
		weights[i] = 1 + int((seed>>(8*i))%50)
	}

	encoded, tags := encode(text)
	assert.True(t, keepsPlaceholders(encoded, tags), "encoded %q", encoded)
	tokens := tokenize(encoded)
	if !hasEmptyPair(tokens) {
		assert.Equal(t, decodedRuns(text), restore(tokens, tags))
	}

	pieces := split(tokens, weights)
	assert.Len(t, pieces, len(weights))
	for _, piece := range pieces {
		opens, closes := map[int]int{}, map[int]int{}
		for _, tok := range piece {
			switch tok.kind {
			case openToken:
				opens[tok.number]++
			case closeToken:
				closes[tok.number]++
			case textToken, singleToken:
			}
		}
		assert.Equal(t, opens, closes)
	}

	if keepsPlaceholders(text, tags) {
		restore(tokenize(text), tags)
	}
	restore(plainTokens(removePlaceholders(text)), nil)
}

// hasEmptyPair reports whether an open token is followed by its close token, with only white space between them.
func hasEmptyPair(tokens []token) bool {
	for i, t := range tokens {
		if t.kind != openToken {
			continue
		}
		next := i + 1
		if next < len(tokens) && tokens[next].kind == textToken && strings.Trim(tokens[next].text, placeholderSpace) == "" {
			next++
		}
		if next < len(tokens) && tokens[next].kind == closeToken && tokens[next].number == t.number {
			return true
		}
	}
	return false
}

// decodedRuns returns text with each run between tags decoded and escaped again.
func decodedRuns(text string) string {
	var b strings.Builder
	pos := 0
	for _, sp := range tagRegexp.FindAllStringIndex(text, -1) {
		b.WriteString(markupcore.EscapeText(markupcore.DecodeEntities(text[pos:sp[0]])))
		b.WriteString(text[sp[0]:sp[1]])
		pos = sp[1]
	}
	b.WriteString(markupcore.EscapeText(markupcore.DecodeEntities(text[pos:])))
	return b.String()
}
