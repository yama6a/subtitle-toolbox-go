package chapters_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/chapters"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestReadOGM(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    []span
	}{
		{
			"times of mkvmerge", "  CHAPTER02 = 00:01:02,5\nCHAPTER02NAME= Second\n\nCHAPTER01=0:00:00.123456789\nCHAPTER01NAME=First\nCHAPTER03=01:00:00.000\n",
			[]span{{sec(0.123), sec(62.5), "First"}, {sec(62.5), sec(62.5), "Second"}},
		},
		{
			"white space around each separator and text after the digits", "CHAPTER01\t=\v1 : 02 :\f03 .\t 4abc\nCHAPTER01NAME\t=\tName & <x>\n",
			[]span{{3723*sec(1) + sec(0.4), 3723*sec(1) + sec(0.4), "Name &amp; &lt;x&gt;"}},
		},
		{
			"rounds half up", "CHAPTER01=00:00:00.0005\nCHAPTER01NAME=A\nCHAPTER02=00:00:00.0004999\nCHAPTER02NAME=B\n",
			[]span{{0, sec(0.001), "B"}, {sec(0.001), sec(0.001), "A"}},
		},
		{"CR line ends and no name", "CHAPTER01=00:00:01.000\rCHAPTER01NAME=\r", []span{{sec(1), sec(1), ""}}},
		{"blank content", "\n", nil},
		{"a million spaces in the name", "CHAPTER01=00:00:00.000\nCHAPTER01NAME=" + strings.Repeat(" ", 1<<20) + "x\n", []span{{0, 0, "x"}}},
		{"a million spaces in the time line", "CHAPTER01=" + strings.Repeat(" ", 1<<20) + "0:0:1.0\nCHAPTER01NAME=x\n", []span{{sec(1), sec(1), "x"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := readSubtitle(t, chapters.ReadOGM, tt.content, chapters.ReadOptions{})
			assert.Equal(t, tt.want, spansOf(s))
		})
	}
}

func TestReadOGMErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantMsg  string
		wantLine int
	}{
		{"name first", "CHAPTER01NAME=Intro\n", `The line "CHAPTER01NAME=Intro" is not a CHAPTERxx= line.`, 1},
		{"no fraction", "CHAPTER01=00:00:00\n", `The line "CHAPTER01=00:00:00" is not a CHAPTERxx= line.`, 1},
		{"minute 60", "CHAPTER01=00:60:00.000\n", `The line "CHAPTER01=00:60:00.000" has a minute or second above 59.`, 1},
		{"second 60", "CHAPTER01=00:00:60.000\n", `The line "CHAPTER01=00:00:60.000" has a minute or second above 59.`, 1},
		{"100000 hours", "CHAPTER01=100000:00:00.000\n", `The time "CHAPTER01=100000:00:00.000" is not below 100000 hours.`, 1},
		{"hours beyond int64", "CHAPTER01=99999999999999999999:00:00.000\n", `The time "CHAPTER01=99999999999999999999:00:00.000" is not below 100000 hours.`, 1},
		{"two times", "CHAPTER01=00:00:00.000\n\nCHAPTER02=00:01:00.000\n", `The line "CHAPTER02=00:01:00.000" is not a CHAPTERxxNAME= line.`, 3},
		{"name without digits", "CHAPTER01=00:00:00.000\nCHAPTERNAME=x\n", `The line "CHAPTERNAME=x" is not a CHAPTERxxNAME= line.`, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			perr := readError(t, chapters.ReadOGM, tt.content)
			assert.Equal(t, tt.wantMsg, perr.Msg)
			assert.Equal(t, tt.wantLine, perr.Line)
		})
	}
}

func TestWriteOGMNumbersWithTwoDigitsAtLeast(t *testing.T) {
	t.Parallel()

	rows := make([]model.Cue, 100)
	for i := range rows {
		rows[i] = model.Cue{Start: sec(float64(i * 60)), Lines: []string{"<b>Part</b> &amp; " + strconv.Itoa(i), "two"}}
	}
	lines := strings.Split(writeString(t, chapters.WriteOGM, chaptersOf(rows...), model.CommonWriteOptions{}), "\n")

	require.Len(t, lines, 201)
	assert.Equal(t, []string{"CHAPTER01=00:00:00.000", "CHAPTER01NAME=Part & 0 two"}, lines[:2])
	assert.Equal(t, []string{"CHAPTER100=01:39:00.000", "CHAPTER100NAME=Part & 99 two", ""}, lines[198:])
}

func BenchmarkReadOGM(b *testing.B) {
	for _, tc := range []struct{ name, content string }{
		{"lines=1MB", strings.Repeat("CHAPTER01=00:00:01.500\nCHAPTER01NAME=Chapter\n", (1<<20)/46)},
		{"spaces=1MB", "CHAPTER01=" + strings.Repeat(" ", 1<<20) + "0:0:0.0\nCHAPTER01NAME=x\n"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := chapters.ReadOGM(strings.NewReader(tc.content), model.CommonReadOptions{}, chapters.ReadOptions{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
