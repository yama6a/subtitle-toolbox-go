package cavena890_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const second = time.Second

// header returns the header of a new file. Each entry of edits replaces the bytes at its offset.
func header(t *testing.T, edits map[int]string) []byte {
	t.Helper()

	var out bytes.Buffer
	_, err := cavena890.Write(&out, &model.Subtitle{}, model.CommonWriteOptions{}, cavena890.WriteOptions{})
	require.NoError(t, err)
	h := out.Bytes()
	for at, value := range edits {
		copy(h[at:], value)
	}
	return h
}

// block returns a block of 1 s to 4 s with the number, the alignment code and the bytes of both lines. Short lines
// are filled with 7Fh.
func block(number uint16, align byte, upper, lower string) []byte {
	line := func(s string) string { return s + strings.Repeat("\x7F", 51-len(s)) }
	b := []byte{byte(number >> 8), byte(number), 0, 0, 25, 0, 0, 100, align}
	b = append(b, make([]byte, 7)...)
	b = append(b, line(upper)...)
	b = append(b, make([]byte, 6)...)
	b = append(b, line(lower)...)
	return append(b, 0, 0, 0, 0)
}

func read(t *testing.T, content []byte, lenient bool) (*model.Subtitle, []model.ParseWarning, error) {
	t.Helper()

	return cavena890.Read(bytes.NewReader(content), model.CommonReadOptions{Lenient: lenient}, cavena890.ReadOptions{})
}

func write(t *testing.T, s *model.Subtitle, common model.CommonWriteOptions) []byte {
	t.Helper()

	var out bytes.Buffer
	_, err := cavena890.Write(&out, s, common, cavena890.WriteOptions{})
	require.NoError(t, err)
	return out.Bytes()
}

func cue(start, end time.Duration, lines ...string) model.Cue {
	return model.Cue{Start: start, End: end, Lines: lines}
}

func subtitle(cues ...model.Cue) *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(cues...)
	return s
}

// withFileData returns a subtitle with the cavena890 file format data.
func withFileData(t *testing.T, fields ...model.Field) *model.Subtitle {
	t.Helper()

	s := &model.Subtitle{}
	require.NoError(t, s.SetFormatData("cavena890", model.NewData(fields...)))
	return s
}
