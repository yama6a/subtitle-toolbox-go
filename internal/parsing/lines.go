package parsing

import (
	"iter"
	"strings"
)

// Lines yields the lines of content and their 0-based numbers. LF, a run of CR before LF, and a lone CR end a line.
func Lines(content string) iter.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		n := 0
		for {
			i := strings.IndexAny(content, "\r\n")
			if i < 0 {
				yield(n, content)
				return
			}
			if !yield(n, content[:i]) {
				return
			}
			next := i + 1
			if content[i] == '\r' {
				j := i
				for j < len(content) && content[j] == '\r' {
					j++
				}
				if j < len(content) && content[j] == '\n' {
					next = j + 1
				}
			}
			content = content[next:]
			n++
		}
	}
}
