package wordgroup_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func word(text string, start, end int) wordgroup.Word {
	return wordgroup.Word{Text: text, Start: ms(start), Timed: true, End: ms(end), HasEnd: true}
}

func spoken(w wordgroup.Word, speaker string) wordgroup.Word {
	w.Speaker, w.HasSpeaker = speaker, true
	return w
}

func parseJSON(t *testing.T, s string) orderedjson.Value {
	t.Helper()
	v, err := orderedjson.Decode([]byte(s))
	require.NoError(t, err)
	return v
}

func repeat(s string, n int) string { return strings.Repeat(s, n) }
