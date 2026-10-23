package mpsub_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/mpsub"
)

// Rows of the MPSub signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"FORMAT first", "FORMAT=25\n\n0 50\nHello\n", true},
		{"FORMAT alone", "FORMAT=", true},
		{"FORMAT after other headers", "TITLE=Bakery\nAUTHOR=Jane\nFORMAT=TIME   # seconds\n\n1 2\nHello\n", true},
		{"FORMAT far down", "TITLE=Bakery\n\n\n1 2\nHello\n\nFORMAT=25\n", true},
		{"FORMAT with any value", "NOTE=x\nFORMAT=PAL\n", true},
		{"headers without FORMAT", "TITLE=Bakery\nAUTHOR=Jane Doe\n", false},
		{"FORMAT in lower case", "TITLE=x\nformat=25\n", false},
		{"FORMAT after a space", "TITLE=x\n FORMAT=25\n", false},
		{"FORMAT after other text on its line", "TITLE=x\nx FORMAT=25\n", false},
		{"FORMAT inside a value", "TITLE=x FORMAT=25\n", false},
		{"FORMAT without equals sign", "TITLE=x\nFORMAT 25\n", false},
		{"first line lower case", "title=x\nFORMAT=25\n", false},
		{"first line without equals sign", "TITLE x\nFORMAT=25\n", false},
		{"first line with a digit in the key", "TITLE2=x\nFORMAT=25\n", false},
		{"first line starts with a digit", "1TITLE=x\nFORMAT=25\n", false},
		{"first line starts with equals sign", "=x\nFORMAT=25\n", false},
		{"first line starts with a space", " TITLE=x\nFORMAT=25\n", false},
		{"capitals only", "TITLE", false},
		{"capitals that are not ASCII", "ТITLE=x\nFORMAT=25\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, mpsub.Detect([]byte(tt.content)))
		})
	}
}
