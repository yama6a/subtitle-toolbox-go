package ttml_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/format/ttml"
)

// Rows of the TTML signature of https://github.com/yama6a/subtitle-toolbox-go/issues/15.
func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"XML declaration and comment", "<?xml version=\"1.0\"?>\n<!-- made by hand -->\n<tt xmlns=\"http://www.w3.org/ns/ttml\"/>", true},
		{"prefixed root", `<tt:tt xmlns:tt="http://www.w3.org/ns/ttml"></tt:tt>`, true},
		{"root alone", "<tt>", true},
		{"root and a tab", "<tt\txml:lang=\"en\">", true},
		{"root and a slash", "<tt/>", true},
		{"DOCTYPE", "<!DOCTYPE tt>\n<tt>", true},
		{"comment with a tag inside", "<!-- <p> -->\n<tt>", true},
		{"processing instruction", "<?xml-stylesheet href=\"a\"?><tt>", true},
		{"other root", "<?xml version=\"1.0\"?>\n<ttx/>", false},
		{"root after an element", "<a/><tt>", false},
		{"root after text", "Credit\n<tt>", false},
		{"bad prefix", "<1x:tt>", false},
		{"SAMI", "<SAMI><BODY>", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, ttml.Detect([]byte(tt.content)))
		})
	}
}
