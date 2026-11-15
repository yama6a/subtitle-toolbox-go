package stbox_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
)

// parseBytes reads content with stbox.Parse.
func parseBytes(content []byte, opts stbox.ReadOptions) (*stbox.Subtitle, stbox.ReadInfo, error) {
	return stbox.Parse(bytes.NewReader(content), int64(len(content)), opts)
}

type cueRow struct {
	Start, End time.Duration
	Text       string
}

func cueRows(s *stbox.Subtitle) []cueRow {
	out := make([]cueRow, 0, s.Len())
	for _, c := range s.All() {
		out = append(out, cueRow{Start: c.Start, End: c.End, Text: strings.Join(c.TaggedLines(), "\n")})
	}
	return out
}

func writeAs(t *testing.T, s *stbox.Subtitle, f stbox.Format) []byte {
	t.Helper()

	var out bytes.Buffer
	_, err := stbox.Write(&out, s, f, stbox.WriteOptions{})
	require.NoError(t, err)
	return out.Bytes()
}
