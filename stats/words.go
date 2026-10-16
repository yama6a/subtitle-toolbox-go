package stats

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
)

// wordCounter counts normalized words and remembers the order of their first use.
type wordCounter struct {
	index map[string]int
	words []WordCount
}

func (c *wordCounter) add(word string) {
	if word == "" {
		return
	}
	if i, ok := c.index[word]; ok {
		c.words[i].Count++
		return
	}
	if c.index == nil {
		c.index = map[string]int{}
	}
	c.index[word] = len(c.words)
	c.words = append(c.words, WordCount{Word: word, Count: 1})
}

func (c *wordCounter) sorted() []WordCount {
	slices.SortStableFunc(c.words, func(a, b WordCount) int { return cmp.Compare(b.Count, a.Count) })
	return c.words
}

// normalize strips the punctuation and symbols around a word and makes it lower case with the full Unicode
// mapping, as PHP mb_strtolower does. A word that is not valid UTF-8 gets ASCII punctuation and ASCII letters only.
func normalize(word string, lower cases.Caser) string {
	if utf8.ValidString(word) {
		return lower.String(strings.TrimFunc(word, isPunctuationOrSymbol))
	}
	return asciiLower(strings.TrimFunc(word, isASCIIPunctuation))
}

func isPunctuationOrSymbol(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

func isASCIIPunctuation(r rune) bool {
	return '!' <= r && r <= '/' || ':' <= r && r <= '@' || '[' <= r && r <= '`' || '{' <= r && r <= '~'
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
