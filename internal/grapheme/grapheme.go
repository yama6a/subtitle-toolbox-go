// Package grapheme counts and splits text in the extended grapheme clusters of UAX #29.
//
// A cluster is one character as a reader sees it. The segmentation comes from github.com/rivo/uniseg v0.4.7,
// which follows Unicode 15.0.0. That version has no rule GB9c, so the Devanagari conjunct ksha (U+0915 U+094D
// U+0937) counts 2. Unicode 15.1 joins it into 1 cluster.
package grapheme

import (
	"iter"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// Count returns the number of grapheme clusters in s that a reader sees.
// Invisible format characters count 0. Invalid UTF-8 counts its bytes.
func Count(s string) int {
	if !utf8.ValidString(s) {
		return len(s)
	}
	n, state := 0, -1
	for s != "" {
		var c string
		c, s, _, state = uniseg.FirstGraphemeClusterInString(s, state)
		if r, _ := utf8.DecodeRuneInString(c); !IsInvisible(r) {
			n++
		}
	}
	return n
}

// Clusters yields the extended grapheme clusters of s in order.
// For invalid UTF-8 it yields one byte at a time.
func Clusters(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		if !utf8.ValidString(s) {
			for i := range len(s) {
				if !yield(s[i : i+1]) {
					return
				}
			}
			return
		}
		state := -1
		for s != "" {
			var c string
			c, s, _, state = uniseg.FirstGraphemeClusterInString(s, state)
			if !yield(c) {
				return
			}
		}
	}
}

// IsInvisible reports whether r counts 0 characters.
func IsInvisible(r rune) bool {
	return r == 0x00AD || r == 0x200B || r == 0xFEFF || IsBidiControl(r)
}

// IsBidiControl reports whether r is one of the bidi controls U+200E, U+200F, U+202A to U+202E and U+2066 to U+2069.
func IsBidiControl(r rune) bool {
	return 0x200E <= r && r <= 0x200F || 0x202A <= r && r <= 0x202E || 0x2066 <= r && r <= 0x2069
}
