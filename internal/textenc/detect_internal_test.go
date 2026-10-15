package textenc

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCandidatesResolve(t *testing.T) {
	t.Parallel()

	for _, c := range candidates {
		_, err := Lookup(c.name)
		require.NoError(t, err, c.name)
		for _, language := range c.languages {
			known := alphabets[language] != "" || ranges[language] != nil || language == "ko" || language == "zh-Hans" || language == "zh-Hant"
			assert.True(t, known, "%s of %s has no alphabet", language, c.name)
		}
	}
}

func TestKSX1001HasTheCommonSyllables(t *testing.T) {
	t.Parallel()

	n := 0
	for r := rune(0xAC00); r <= 0xD7A3; r++ {
		if isKSX1001Syllable(r) {
			n++
		}
	}
	assert.Equal(t, 2350, n)
}

func TestHanLists(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 661, countHan(hanSimplified))
	assert.Equal(t, 688, countHan(hanTraditional))
}

func countHan(s string) int {
	return utf8.RuneCountInString(strings.TrimSpace(s))
}
