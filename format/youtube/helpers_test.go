package youtube_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/youtube"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

var withWords = youtube.ReadOptions{WordTimestamps: true}

func read(t *testing.T, content string, opts youtube.ReadOptions) *model.Subtitle {
	t.Helper()

	s, _, err := youtube.Read(strings.NewReader(content), model.CommonReadOptions{}, opts)
	require.NoError(t, err)
	return s
}

func readFile(t *testing.T, name string, opts youtube.ReadOptions) (*model.Subtitle, string) {
	t.Helper()

	src := string(testfiles.Read(t, "youtube/real/"+name))
	return read(t, src, opts), src
}

func write(t *testing.T, s *model.Subtitle) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, youtube.Write(&out, s, model.CommonWriteOptions{}, youtube.WriteOptions{}))
	return out.String()
}

// editCue applies edit to cue i of s.
func editCue(t *testing.T, s *model.Subtitle, i int, edit func(c *model.Cue)) {
	t.Helper()

	c, ok := s.Cue(i)
	require.True(t, ok)
	edit(&c)
	require.NoError(t, s.SetCue(i, c))
}

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// times returns the start and the end of each cue, in milliseconds.
func times(s *model.Subtitle) [][2]int {
	out := make([][2]int, 0, s.Len())
	for _, c := range s.Cues() {
		out = append(out, [2]int{int(c.Start.Milliseconds()), int(c.End.Milliseconds())})
	}
	return out
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()

	return testfiles.Read(t, name)
}
