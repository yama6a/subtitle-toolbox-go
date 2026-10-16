package cli

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTable(t *testing.T) {
	t.Parallel()

	path := "/media/" + strings.Repeat("x", 83)
	words := func(n int) string { return strings.TrimSpace(strings.Repeat("word ", n)) }
	tests := []struct {
		name        string
		rows        [][]string
		indent, gap int
		want        string
	}{
		{
			"a word wider than a line gets a line of its own",
			[][]string{{"--log", "Read " + path + " from here."}},
			2, 2,
			"  --log  Read\n         " + path + "\n         from here.\n",
		},
		{
			"the last column wraps with a hanging indent",
			[][]string{{"-o", "Output."}, {"--from", words(20)}},
			2, 2,
			"  -o      Output.\n  --from  " + words(14) + "\n          " + words(6) + "\n",
		},
		{
			"an empty last cell leaves no trailing spaces",
			[][]string{{"--lenient", ""}, {"--json", "Print JSON."}},
			2, 2,
			"  --lenient\n  --json     Print JSON.\n",
		},
		{
			"gap 1 and 3 columns",
			[][]string{{"a", "bb", "x"}, {"ccc", "d", "y"}},
			2, 1,
			"  a   bb x\n  ccc d  y\n",
		},
		{
			"no indent",
			[][]string{{"Name", "Read"}, {"srt", "yes"}},
			0, 2,
			"Name  Read\nsrt   yes\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, table(tt.rows, tt.indent, tt.gap))
		})
	}
}

func TestUsageLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want string
	}{
		{
			"an option group at the wrap point moves to the next line whole",
			"<input>... --to FORMAT --from FMT --errors-fix [--output-dir DIR] [options]",
			"Usage: stbox convert <input>... --to FORMAT --from FMT --errors-fix\n" +
				"                     [--output-dir DIR] [options]\n",
		},
		{
			"an option and its value at the wrap point move to the next line together",
			"<input>... --to FORMAT [-o FILE.srt] --errors-fix --encoding NAME [options]",
			"Usage: stbox convert <input>... --to FORMAT [-o FILE.srt] --errors-fix\n" +
				"                     --encoding NAME [options]\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, usageLine("Usage: stbox convert", tt.line))
		})
	}
}

func TestWrap(t *testing.T) {
	t.Parallel()

	command := "  stbox convert " + strings.Repeat("film.srt ", 10) + "--to vtt"
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a line that starts with a space stays", command, command + "\n"},
		{"empty lines stay", "a\n\nb", "a\n\nb\n"},
		{"trailing spaces go", "a  \n   ", "a\n\n"},
		{
			"a long line wraps at 80 columns",
			strings.Repeat("abcd ", 20),
			strings.TrimSpace(strings.Repeat("abcd ", 16)) + "\n" + strings.TrimSpace(strings.Repeat("abcd ", 4)) + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, wrap(tt.in))
		})
	}
}

func TestFormatNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   float64
		want string
	}{
		{1.0005, "1.001"},
		{27.53333, "27.533"},
		{4.9, "4.9"},
		{4.90, "4.9"},
		{-0.0004, "0"},
		{-0.0005, "-0.001"},
		{15, "15"},
		{0.9995, "1"},
		{99.9999, "100"},
		{-2.5, "-2.5"},
		{1e21, "1000000000000000000000"},
		{math.Inf(1), "INF"},
		{math.Inf(-1), "INF"},
		{math.NaN(), "-"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, formatNumber(tt.in))
		})
	}
}
