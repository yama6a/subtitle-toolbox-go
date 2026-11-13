package hilabels_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/hilabels"
)

func TestRemoveLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, line string
		anyCase    bool
		want       string
	}{
		{"upper case label", "JOHN: Hi.", false, "Hi."},
		{"label after a dash keeps the dash", "- JOHN: Hi.", false, "- Hi."},
		{"label inside a tag", "<i>JOHN: Hi.</i>", false, "<i>Hi.</i>"},
		{"label alone", "JOHN:", false, ""},
		{"mixed case label is no label", "Baker: Hi.", false, "Baker: Hi."},
		{"mixed case label with any case", "Baker: Hi.", true, "Hi."},
		{"no space after the colon", "JOHN:Hi.", false, "JOHN:Hi."},
		{"time", "At 10:30 we go.", false, "At 10:30 we go."},
		{"invalid UTF-8", "\xffJOHN: Hi.", false, "\xffJOHN: Hi."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, hilabels.RemoveLabel(tt.line, tt.anyCase))
		})
	}
}
