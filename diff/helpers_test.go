package diff_test

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/diff"
	"github.com/yama6a/subtitle-toolbox-go/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// row is a cue in seconds.
type row struct {
	start, end float64
	text       string
}

// want is the kind and the 0-based indexes of one difference. -1 marks the side without a partner.
type want struct {
	kind     diff.Kind
	old, new int
}

func seconds(f float64) time.Duration {
	return time.Duration(math.Round(f*1000)) * time.Millisecond
}

func subtitle(rows ...row) *model.Subtitle {
	var s model.Subtitle
	for _, r := range rows {
		s.AddCue(model.Cue{Start: seconds(r.start), End: seconds(r.end), Lines: []string{r.text}})
	}
	return &s
}

func load(tb testing.TB, name string) *model.Subtitle {
	tb.Helper()

	content := bytes.TrimPrefix(testfiles.Read(tb, "diff/"+name), []byte("\xef\xbb\xbf"))
	s, _, err := srt.Read(bytes.NewReader(content), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(tb, err)
	return s
}

func summarize(differences []diff.Difference) []want {
	out := make([]want, len(differences))
	for i, d := range differences {
		out[i] = want{d.Kind, d.OldIndex, d.NewIndex}
	}
	return out
}

func compare(tb testing.TB, old, updated *model.Subtitle, opts diff.Options) []want {
	tb.Helper()

	differences, err := diff.Compare(old, updated, opts)
	require.NoError(tb, err)
	return summarize(differences)
}

func tolerance(d time.Duration) *time.Duration { return &d }
