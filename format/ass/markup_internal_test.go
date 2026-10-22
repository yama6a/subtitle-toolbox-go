package ass

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOverrideTags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{"one tag", `\an8`, []string{`\an8`}},
		{"two tags", `\b1\i1`, []string{`\b1`, `\i1`}},
		{"backslash inside parentheses", `\t(\1c&HFF&)`, []string{`\t(\1c&HFF&)`}},
		{"nested parentheses", `\t(0,500,\clip(1,2,3,4)\b1)\i1`, []string{`\t(0,500,\clip(1,2,3,4)\b1)`, `\i1`}},
		{"text after the parentheses", `\t(\b1)x \i1`, []string{`\t(\b1)x`, `\i1`}},
		{"unbalanced parentheses split at the backslash", `\t(\b1`, []string{`\t(`, `\b1`}},
		{"a second group after a balanced one splits at the backslash", `\t(\b1)(\i1)`, []string{`\t(\b1)(`, `\i1)`}},
		{"text before the first backslash", `note\b1`, []string{`\b1`}},
		{"spaces are trimmed", ` \b1 \i1 `, []string{`\b1`, `\i1`}},
		{"a note without backslash", "note", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, overrideTags(tt.content))
		})
	}
}

func TestKaraokeDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		whole, fraction string
		want            time.Duration
	}{
		{"50", "", 500 * time.Millisecond},
		{"12", "5", 125 * time.Millisecond},
		{"0", "05", 500 * time.Microsecond},
		{"1", "123456789", 10*time.Millisecond + 1234567*time.Nanosecond},
		{"00000000000000050", "", 500 * time.Millisecond},
		{"99999999999", "", 999999999990 * time.Millisecond},
		{"999999999999", "", 100_000 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.whole+"."+tt.fraction, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, karaokeDuration(tt.whole, tt.fraction))
		})
	}
}

func TestMergeBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		parts []part
		want  string
	}{
		{"adjacent tags merge", []part{{true, `\an8`}, {true, `\b1`}, {false, "a"}}, `{\an8\b1}a`},
		{"a later tag of a kind replaces the earlier and goes last", []part{{true, `\b1`}, {true, `\i1`}, {true, `\b0`}, {false, "a"}}, `{\i1\b0}a`},
		{"clip is no colour tag", []part{{true, `\c&H0000FF&`}, {true, `\clip(1,2,3,4)`}, {true, `\c`}, {false, "a"}}, `{\clip(1,2,3,4)\c}a`},
		{"tags at the end", []part{{false, "a"}, {true, `\b0`}}, `a{\b0}`},
		{"text only", []part{{false, "a"}, {false, "b"}}, "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, mergeBlocks(tt.parts))
		})
	}
}
