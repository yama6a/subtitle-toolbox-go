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
	t := tokenizer{content: content, line: 1, start: 1}
	for i := 0; i < len(content); {
		c := content[i]
		if c == '"' && t.cell.Len() == 0 {
			next, err := t.quoted(i, lenient)
			if err != nil {
				return nil, nil, err
			}
			i = next
			continue
		}
		switch c {
		case delimiter:
			t.endCell()
		case '\r', '\n':
			i = lineBreakEnd(content, i) - 1
			t.endRecord()
			t.line++
			t.start = t.line
		default:
			t.cell.WriteByte(c)
		}
		i++
	}
	if t.cell.Len() > 0 || len(t.cells) > 0 {
		t.endRecord()
	}
	return t.records, t.unclosed, nil
}

// tokenizer holds the state of records.
type tokenizer struct {
	content  string
	records  []record
	unclosed []unclosedQuote
	cells    []string
	quotedAt []int
	cell     strings.Builder
	line     int // the line of the byte that records reads
	start    int // the line where the record starts
}

func (t *tokenizer) endCell() {
	t.cells = append(t.cells, t.cell.String())
	t.cell.Reset()
}

func (t *tokenizer) endRecord() {
	t.endCell()
	t.records = append(t.records, record{line: t.start, cells: t.cells, quoted: t.quotedAt})
	t.cells, t.quotedAt = nil, nil
}

// quoted reads the quoted cell that starts at the quote at offset i and returns the offset after it.
func (t *tokenizer) quoted(i int, lenient bool) (int, error) {
	if text, next, breaks, ok := quotedCell(t.content, i+1); ok {
		t.cell.WriteString(text)
		t.line += breaks
		t.quotedAt = append(t.quotedAt, len(t.cells))
		return next, nil
	}
	if !lenient {
		return 0, &model.ParseError{Format: model.CSV, Line: t.start, Msg: "A quoted CSV cell has no closing quote."}
	}
	lineStart := strings.LastIndexAny(t.content[:i], "\r\n") + 1
	end := len(t.content)
	if j := strings.IndexAny(t.content[i+1:], "\r\n"); j >= 0 {
		end = i + 1 + j
	}
	t.cell.WriteString(t.content[i+1 : end])
	t.unclosed = append(t.unclosed, unclosedQuote{line: t.line, text: t.content[lineStart:end]})
	return end, nil
}

// quotedCell reads a quoted cell from the offset after its opening quote. A doubled quote is one quote, and CR,
// LF or CR LF is one LF. It returns the cell, the offset after the closing quote and the count of line breaks.
// ok is false without a closing quote.
func quotedCell(content string, from int) (cell string, next, breaks int, ok bool) {
	var b strings.Builder
	for i := from; i < len(content); i++ {
		switch c := content[i]; c {
		case '"':
			if i+1 == len(content) || content[i+1] != '"' {
				return b.String(), i + 1, breaks, true
			}
			b.WriteByte('"')
			i++
		case '\r', '\n':
			i = lineBreakEnd(content, i) - 1
			b.WriteByte('\n')
			breaks++
		default:
			b.WriteByte(c)
		}
	}
	return "", 0, 0, false
}

// lineBreakEnd returns the offset after the line break at offset i: CR, LF or CR LF.
func lineBreakEnd(content string, i int) int {
	if content[i] == '\r' && i+1 < len(content) && content[i+1] == '\n' {
		return i + 2
	}
	return i + 1
}
