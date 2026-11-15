package grapheme_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/grapheme"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

func TestCount(t *testing.T) {
	t.Parallel()

	data := testfiles.Read(t, "character-count/count.json")
	type row struct {
		Name string `json:"name"`
		Text string `json:"text"`
		Want int    `json:"want"`
	}
	var rows []row
	require.NoError(t, json.Unmarshal(data, &rows))
	tests := slices.Concat(rows, []row{
		{"invalid UTF-8 counts bytes", "Caf\xe9", 4},
		{"invalid UTF-8 counts the invisible characters too", "\xff\u200b", 4},
	})
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.Want, grapheme.Count(tt.Text))
		})
	}
}

func TestClusters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want []string
	}{
		{"empty", "", nil},
		{"letters", "abc", []string{"a", "b", "c"}},
		{"combining mark", "Cafe\u0301!", []string{"C", "a", "f", "e\u0301", "!"}},
		{"skin tone and ZWJ sequence", "\U0001f44d\U0001f3fd \U0001f468\u200d\U0001f469", []string{"\U0001f44d\U0001f3fd", " ", "\U0001f468\u200d\U0001f469"}},
		{"invisible characters are clusters", "a\u200bb", []string{"a", "\u200b", "b"}},
		{"CRLF", "a\r\nb", []string{"a", "\r\n", "b"}},
		{"invalid UTF-8 gives bytes", "a\xe9\xff", []string{"a", "\xe9", "\xff"}},
		{"truncated sequence gives bytes", "\xe2\x82", []string{"\xe2", "\x82"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, slices.Collect(grapheme.Clusters(tt.text)))
		})
	}
}

func TestClustersStopsEarly(t *testing.T) {
	t.Parallel()

	var got []string
	for c := range grapheme.Clusters("abc") {
		got = append(got, c)
		break
	}
	assert.Equal(t, []string{"a"}, got)
}

func TestIsBidiControl(t *testing.T) {
	t.Parallel()

	for _, r := range []rune{0x200E, 0x200F, 0x202A, 0x202E, 0x2066, 0x2069} {
		assert.True(t, grapheme.IsBidiControl(r), "%U", r)
	}
	for _, r := range []rune{'a', 0x00AD, 0x200B, 0x200D, 0x2029, 0x202F, 0x2065, 0x206A, 0xFEFF} {
		assert.False(t, grapheme.IsBidiControl(r), "%U", r)
	}
}
