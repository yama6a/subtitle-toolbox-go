package subviewer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/subviewer"
)

// Rows of the SubViewer signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"SubViewer 1 script line", "[TITLE]\nBakery\n******** START SCRIPT ********\n[00:00:01]\nHello\n", true},
		{"script line alone", "******** START SCRIPT ********", true},
		{"script line with spaces and tabs after it", "x\n******** START SCRIPT ******** \t\ny\n", true},
		{"script line far down", "a\nb\n\n\n******** START SCRIPT ********\n", true},
		{"script line with a space before it", "x\n ******** START SCRIPT ********\n", false},
		{"script line with other text after it", "x\n******** START SCRIPT ******** x\n", false},
		{"script line with 7 stars", "x\n******* START SCRIPT *******\n", false},
		{"script line in lower case", "x\n******** start script ********\n", false},
		{"information line", "[INFORMATION]\n[TITLE]Bakery\n", true},
		{"information line alone", "[INFORMATION]", true},
		{"information line with spaces after it", "[INFORMATION] \t\n", true},
		{"information line in lower case", "[information]\n", false},
		{"information line that is not the first line", "[TITLE]x\n[INFORMATION]\n", false},
		{"information line with text after it", "[INFORMATION] x\n", false},
		{"timing line first", "00:00:01.50,00:00:04.00\nHello[br]world\n", true},
		{"timing line alone", "00:00:01.50,00:00:04.00", true},
		{"timing line with spaces after it", "00:00:01.50,00:00:04.00 \t\nHello\n", true},
		{"timing line after bracket lines", "[SUBTITLE]\n[COLF]&HFFFFFF,[SIZE]18\n00:00:01.50,00:00:04.00\nHello\n", true},
		{"timing line after an empty bracket line", "[TITLE]\n00:00:01.50,00:00:04.00\n", true},
		{"timing line after a line that does not start with [", "Hello\n00:00:01.50,00:00:04.00\n", false},
		{"timing line after an empty line", "[TITLE]\n\n00:00:01.50,00:00:04.00\n", false},
		{"timing line with 3 fraction digits", "00:00:01.500,00:00:04.000\nHello\n", false},
		{"timing line with 1 fraction digit", "00:00:01.5,00:00:04.0\nHello\n", false},
		{"timing line with 1 hour digit", "0:00:01.50,0:00:04.00\nHello\n", false},
		{"timing line with a comma instead of the dot", "00:00:01,50,00:00:04,00\nHello\n", false},
		{"timing line with other text after it", "00:00:01.50,00:00:04.00 x\n", false},
		{"timing line with 3 hour digits", "100:00:01.50,00:00:04.00\nHello\n", false},
		{"LRC tag", "[00:01.00]Hello\n[00:02.50]world\n", false},
		{"SubViewer 1 time line", "[00:00:01]\nHello\n", false},
		{"MicroDVD", "{24}{72}Hello\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, subviewer.Detect([]byte(tt.content)))
		})
	}
}
