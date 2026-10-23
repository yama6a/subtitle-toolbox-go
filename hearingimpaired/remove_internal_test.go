package hearingimpaired

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Adapted from HearingImpairedRemoverTest::testRemoveFromTextReturnsTheRemainingText.
func TestRemoveFromLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		text, want string
	}{
		{"JOHN: Hi.", "Hi."},
		{"[DOOR SLAMS]", ""},
		{"Note: this stays.", "Note: this stays."},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()

			var before, after model.Cue
			before.SetText(tt.text)
			for _, l := range removeFromLines(before.Lines, Options{}) {
				after.AddLine(l)
			}
			assert.Equal(t, tt.want, after.Text())
		})
	}
}
