package markupcore_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v5/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v5/internal/testfiles"
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

func readRows[T any](t *testing.T, name string) []T {
	t.Helper()

	var rows []T
	require.NoError(t, json.Unmarshal(testfiles.Read(t, "rtl/"+name), &rows))
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

func TestEmbedRightToLeftMovesPunctuationBeforeTheMongolianVowelSeparator(t *testing.T) {
	t.Parallel()

	got := markupcore.EmbedRightToLeft(". \u05e9\u05dc\u05d5\u05dd\u180e")
	assert.Equal(t, "\u202b\u05e9\u05dc\u05d5\u05dd.\u180e\u202c", got)
}

func TestEmbedRightToLeftKeepsInvalidUTF8(t *testing.T) {
	t.Parallel()

	rows := readRows[embedRow](t, "embed.json")
	for _, in := range []string{
		"\xff" + rows[0].In,
		rows[0].In + "\xe9",
		"<\xff>" + rows[0].In,
		"\xe2\x80\u202c\xab\u05dc",
		"\xe2\u202b\x80\xab\u05d5",
	} {
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

func TestHoldsRightToLeftEmbeddingInvalidUTF8(t *testing.T) {
	t.Parallel()

	for _, line := range []string{"\xe2\u202b\x80\xab\u05d5", "\xff\u202b\u05d0\u202c"} {
		assert.False(t, markupcore.HoldsRightToLeftEmbedding([]string{line}), "%q", line)
	}
}
