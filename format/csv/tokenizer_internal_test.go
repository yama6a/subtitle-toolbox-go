package csv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testSplitsRecords of tests/Parsers/CsvTokenizerTest.php of subtitle-toolbox-php.
func TestRecords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []record
	}{
		{"quoted cells", "\"a\",b,\"\"\n", []record{{1, []string{"a", "b", ""}, []int{0, 2}}}},
		{"CR LF inside quotes", "a,\"b\r\nc\"\r\nd,e\r\n", []record{{1, []string{"a", "b\nc"}, []int{1}}, {3, []string{"d", "e"}, nil}}},
		{"doubled quote", "\"say \"\"hi\"\"\",x\n", []record{{1, []string{"say \"hi\"", "x"}, []int{0}}}},
		{"quote inside unquoted cell", "5\" disk,x\n", []record{{1, []string{"5\" disk", "x"}, nil}}},
		{"text after a closing quote", "\"ab\"cd,x\n", []record{{1, []string{"abcd", "x"}, []int{0}}}},
		{"missing last line break", "a,b\nc,d", []record{{1, []string{"a", "b"}, nil}, {2, []string{"c", "d"}, nil}}},
		{"empty last cell", "a,\n", []record{{1, []string{"a", ""}, nil}}},
		{"lone CR ends a record", "a\rb\r", []record{{1, []string{"a"}, nil}, {2, []string{"b"}, nil}}},
		{"empty quoted cell before a quote", "\"\"\"x\",y\n", []record{{1, []string{"\"x", "y"}, []int{0}}}},
		{"empty line", "a\n\nb\n", []record{{1, []string{"a"}, nil}, {2, []string{""}, nil}, {3, []string{"b"}, nil}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, unclosed, err := records(tt.content, ',', false)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			assert.Empty(t, unclosed)
		})
	}
}

// Ports testStrictModeThrowsForAnUnclosedQuote of tests/Parsers/CsvTokenizerTest.php of subtitle-toolbox-php.
func TestRecordsStrictUnclosedQuote(t *testing.T) {
	t.Parallel()

	_, _, err := records("a,b\nc,\"d,e\nf,g\n", ',', false)
	assert.Equal(t, &model.ParseError{Format: model.CSV, Line: 2, Msg: "A quoted CSV cell has no closing quote."}, err)
}

// Ports testLenientModeEndsAnUnclosedQuoteAtTheLineEnd of tests/Parsers/CsvTokenizerTest.php of subtitle-toolbox-php.
func TestRecordsLenientUnclosedQuote(t *testing.T) {
	t.Parallel()

	got, unclosed, err := records("a,b\nc,\"d,e\nf,g\n", ',', true)
	require.NoError(t, err)
	assert.Equal(t, []record{{1, []string{"a", "b"}, nil}, {2, []string{"c", "d,e"}, nil}, {3, []string{"f", "g"}, nil}}, got)
	assert.Equal(t, []unclosedQuote{{2, "c,\"d,e"}}, unclosed)
}

// Ports testDetectsTheDelimiter of tests/Parsers/CsvTokenizerTest.php of subtitle-toolbox-php.
func TestDetectDelimiter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    Delimiter
	}{
		{"comma wins a tie", "a,b;c\n", Comma},
		{"semicolon by count", "a;b;c,d\n", Semicolon},
		{"tab by count", "a\tb\tc\n", Tab},
		{"quoted delimiters ignored", "\"a;b;c\",d\n", Comma},
		{"first line with one", "\nstart;end\n", Semicolon},
		{"semicolon wins a tie with tab", "a;b\tc\n", Semicolon},
		{"quoted line break", "\"a\nb\";c\n", Semicolon},
		{"none", "abc\n", Comma},
		{"no line break", "a\tb", Tab},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, detectDelimiter(tt.content))
		})
	}
}
