package markupcore

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

type letterRow struct {
	Name string `json:"name"`
	Line string `json:"line"`
	Want bool   `json:"want"`
}

func readRows[T any](t *testing.T, name string) []T {
	t.Helper()

	var rows []T
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "rtl/"+name), &rows))
	return rows
}

func TestHasRightToLeftLetter(t *testing.T) {
	t.Parallel()

	for _, tt := range readRows[letterRow](t, "letters.json") {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.Want, hasRightToLeftLetter(tt.Line))
		})
	}
}

func TestHasRightToLeftLetterInvalidUTF8(t *testing.T) {
	t.Parallel()

	rows := readRows[letterRow](t, "letters.json")
	assert.False(t, hasRightToLeftLetter("\xff"+rows[0].Line))
}
