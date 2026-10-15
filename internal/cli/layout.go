package cli

import (
	"regexp"
	"strings"
)

// helpWidth is the line width of the help in bytes, as PHP strlen counts. All help text is ASCII.
const helpWidth = 80

// usageWord is a word of a usage line that the wrap keeps whole: a [...] group, an --option VALUE pair, or a
// plain word.
var usageWord = regexp.MustCompile(`\[[^\]]*\]|--\S+ [^\s\[<-]\S*|\S+`)

// table lays out rows in columns, each line indented by indent spaces. Every column but the last is padded to
// its widest cell plus gap spaces. The last column wraps at helpWidth with a hanging indent.
func table(rows [][]string, indent, gap int) string {
	var widths []int
	for _, row := range rows {
		for i, cell := range row[:len(row)-1] {
			if i == len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], len(cell))
		}
	}
	var b strings.Builder
	for _, row := range rows {
		prefix := strings.Repeat(" ", indent)
		for i, cell := range row[:len(row)-1] {
			prefix += cell + strings.Repeat(" ", widths[i]+gap-len(cell))
		}
		b.WriteString(wrapWords(strings.Split(row[len(row)-1], " "), prefix, strings.Repeat(" ", len(prefix))))
	}
	return b.String()
}

// wrap wraps each line of text at helpWidth. A line that starts with a space is a command or an example and
// stays as it is.
func wrap(text string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(text, "\n") {
		if line == "" || line[0] == ' ' {
			b.WriteString(rtrim(line) + "\n")
			continue
		}
		b.WriteString(wrapWords(strings.Split(line, " "), "", ""))
	}
	return b.String()
}

// usageLine wraps a usage line after prefix, such as "Usage: stbox convert". The next lines start below the
// first word after prefix.
func usageLine(prefix, line string) string {
	words := append([]string{prefix}, usageWord.FindAllString(line, -1)...)
	return wrapWords(words, "", strings.Repeat(" ", len(prefix)+1))
}

// wrapWords joins words into lines of at most helpWidth bytes. The first line starts with prefix, the others
// with nextPrefix. A word wider than a line gets a line of its own.
func wrapWords(words []string, prefix, nextPrefix string) string {
	var lines []string
	line, started := prefix, false
	for _, word := range words {
		switch {
		case word == "":
			continue
		case !started:
			line, started = prefix+word, true
		case len(line)+1+len(word) > helpWidth:
			lines = append(lines, rtrim(line))
			line = nextPrefix + word
		default:
			line += " " + word
		}
	}
	lines = append(lines, rtrim(line))
	return strings.Join(lines, "\n") + "\n"
}

func rtrim(s string) string {
	return strings.TrimRight(s, " \t\n\r\x00\v")
}
