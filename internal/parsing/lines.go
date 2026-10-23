package parsing

import (
	"iter"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
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

// PipeLines splits text at each |, the line break of MPL2 and TMPlayer. It trims each part and drops the empty ones.
func PipeLines(text string) []string {
	var lines []string
	for part := range strings.SplitSeq(text, "|") {
		if part = strings.Trim(part, markupcore.PHPTrimSet); part != "" {
			lines = append(lines, part)
		}
	}
	return lines
}
