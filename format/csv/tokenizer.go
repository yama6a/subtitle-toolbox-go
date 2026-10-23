package csv

import (
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// record is one row of a table: the 1-based line of its start, its cells and the indexes of its quoted cells.
type record struct {
	line   int
	cells  []string
	quoted []int
}

// unclosedQuote is a quoted cell without a closing quote that lenient mode ended at the end of its line.
type unclosedQuote struct {
	line int
	text string // the whole line of the quote
}

// detectDelimiter returns the delimiter that occurs most often outside quotes in the first line that has one.
// A tie goes to the first of comma, semicolon and tab.
func detectDelimiter(content string) Delimiter {
	var counts [3]int
	quoted := false
	for i := range len(content) {
		switch c := content[i]; {
		case c == '"':
			quoted = !quoted
		case quoted:
		case (c == '\n' || c == '\r') && counts[0]+counts[1]+counts[2] > 0:
			return mostFrequent(counts)
		case c == ',':
			counts[0]++
		case c == ';':
			counts[1]++
		case c == '\t':
			counts[2]++
		}
	}
	return mostFrequent(counts)
}

func mostFrequent(counts [3]int) Delimiter {
	best := 0
	for i := 1; i < len(counts); i++ {
		if counts[i] > counts[best] {
			best = i
		}
	}
	return [...]Delimiter{Comma, Semicolon, Tab}[best]
}

// records splits content into records. Line breaks inside quotes become LF. A quote inside an unquoted cell is
// text, and text after a closing quote appends to the cell. Strict mode fails for a quote without a closing quote.
// Lenient mode ends that cell at the end of its line and lists it.
func records(content string, delimiter byte, lenient bool) ([]record, []unclosedQuote, error) {
	var (
		out      []record
		unclosed []unclosedQuote
		cells    []string
		quoted   []int
		cell     strings.Builder
		line     = 1
		start    = 1
	)
	endRecord := func() {
		cells = append(cells, cell.String())
		out = append(out, record{line: start, cells: cells, quoted: quoted})
		cells, quoted = nil, nil
		cell.Reset()
	}
	for i := 0; i < len(content); {
		c := content[i]
		if c == '"' && cell.Len() == 0 {
			text, next, breaks, ok := quotedCell(content, i+1)
			if ok {
				cell.WriteString(text)
				i = next
				line += breaks
				quoted = append(quoted, len(cells))
				continue
			}
			if !lenient {
				return nil, nil, &model.ParseError{Format: model.CSV, Line: start, Msg: "A quoted CSV cell has no closing quote."}
			}
			lineStart := strings.LastIndexAny(content[:i], "\r\n") + 1
			end := len(content)
			if j := strings.IndexAny(content[i+1:], "\r\n"); j >= 0 {
				end = i + 1 + j
			}
			cell.WriteString(content[i+1 : end])
			unclosed = append(unclosed, unclosedQuote{line: line, text: content[lineStart:end]})
			i = end
			continue
		}
		switch c {
		case delimiter:
			cells = append(cells, cell.String())
			cell.Reset()
		case '\r', '\n':
			if c == '\r' && i+1 < len(content) && content[i+1] == '\n' {
				i++
			}
			endRecord()
			line++
			start = line
		default:
			cell.WriteByte(c)
		}
		i++
	}
	if cell.Len() > 0 || len(cells) > 0 {
		endRecord()
	}
	return out, unclosed, nil
}

// quotedCell reads a quoted cell from the offset after its opening quote. A doubled quote is one quote, and CR,
// LF or CR LF is one LF. It returns the cell, the offset after the closing quote and the count of line breaks.
// ok is false without a closing quote.
func quotedCell(content string, from int) (cell string, next, breaks int, ok bool) {
	var b strings.Builder
	for i := from; i < len(content); i++ {
		switch c := content[i]; {
		case c == '"' && i+1 < len(content) && content[i+1] == '"':
			b.WriteByte('"')
			i++
		case c == '"':
			return b.String(), i + 1, breaks, true
		case c == '\r' || c == '\n':
			if c == '\r' && i+1 < len(content) && content[i+1] == '\n' {
				i++
			}
			b.WriteByte('\n')
			breaks++
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, 0, false
}
