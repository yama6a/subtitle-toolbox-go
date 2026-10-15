package markupcore_test

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

type embedRow struct {
	Name string `json:"name"`
	In   string `json:"in"`
	Want string `json:"want"`
}

type holdsRow struct {
	Name  string   `json:"name"`
	Lines []string `json:"lines"`
	Want  bool     `json:"want"`
}

type letterRow struct {
	Name string `json:"name"`
	Line string `json:"line"`
	Want bool   `json:"want"`
}

func readRows[T any](t *testing.T, name string) []T {
	t.Helper()

	data, err := os.ReadFile("testdata/rtl/" + name)
	require.NoError(t, err)
	var rows []T
	require.NoError(t, json.Unmarshal(data, &rows))
	return rows
}

func TestEmbedRightToLeft(t *testing.T) {
	t.Parallel()

	rows := slices.Concat(readRows[embedRow](t, "embed.json"), readRows[embedRow](t, "embed_lf.json"))
	for _, tt := range rows {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			got := markupcore.EmbedRightToLeft(tt.In)
			assert.Equal(t, tt.Want, got)
			assert.Equal(t, got, markupcore.EmbedRightToLeft(got), "idempotent")
		})
	}
}

func TestEmbedRightToLeftKeepsInvalidUTF8(t *testing.T) {
	t.Parallel()

	rows := readRows[embedRow](t, "embed.json")
	for _, in := range []string{"\xff" + rows[0].In, rows[0].In + "\xe9", "<\xff>" + rows[0].In} {
		assert.Equal(t, in, markupcore.EmbedRightToLeft(in))
	}
}

func TestHoldsRightToLeftEmbedding(t *testing.T) {
	t.Parallel()

	for _, tt := range readRows[holdsRow](t, "holds.json") {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.Want, markupcore.HoldsRightToLeftEmbedding(tt.Lines))
		})
	}
}

func TestHasRightToLeftLetter(t *testing.T) {
	t.Parallel()

	for _, tt := range readRows[letterRow](t, "letters.json") {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.Want, markupcore.HasRightToLeftLetter(tt.Line))
		})
	}
}

func TestHasRightToLeftLetterInvalidUTF8(t *testing.T) {
	t.Parallel()

	rows := readRows[letterRow](t, "letters.json")
	assert.False(t, markupcore.HasRightToLeftLetter("\xff"+rows[0].Line))
}

func TestIsBidiControl(t *testing.T) {
	t.Parallel()

	for _, r := range []rune{0x200E, 0x200F, 0x202A, 0x202B, 0x202C, 0x202D, 0x202E, 0x2066, 0x2067, 0x2068, 0x2069} {
		assert.True(t, markupcore.IsBidiControl(r), "%U", r)
	}
	for _, r := range []rune{'a', 0x00AD, 0x200B, 0x200C, 0x200D, 0x2029, 0x202F, 0x2065, 0x206A, 0xFEFF} {
		assert.False(t, markupcore.IsBidiControl(r), "%U", r)
	}
}
