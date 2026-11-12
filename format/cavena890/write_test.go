package cavena890_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v2/format/cavena890"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// blockNumbers returns the subtitle numbers of the blocks of a written file.
func blockNumbers(out []byte) []int {
	var numbers []int
	for at := 388; at < len(out); at += 128 {
		numbers = append(numbers, int(binary.BigEndian.Uint16(out[at:])))
	}
	return numbers
}

func texts(s *model.Subtitle) []string {
	var out []string
	for _, c := range s.All() {
		out = append(out, c.Text())
	}
	return out
}

func TestWriteNewFile(t *testing.T) {
	t.Parallel()

	c := cue(second, 4*second, "<i>Grüße</i> [x]")
	c.Position.Alignment = model.BottomLeft
	out := write(t, subtitle(c), model.CommonWriteOptions{})

	require.Len(t, out, 388+128-4)
	for _, at := range []int{179, 208, 210} {
		assert.Equal(t, uint16(1), binary.LittleEndian.Uint16(out[at:]))
	}
	assert.Equal(t, "00100000190000645000000000000000", hex.EncodeToString(out[388:388+16]))
	assert.Equal(t, "8847728675df659820e578e6"+strings.Repeat("7f", 39), hex.EncodeToString(out[388+73:388+73+51]))
}

func TestWriteNewBlocks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		variant     string // the file format data names the variant and holds the header with its language code
		language    string
		strip       bool
		cues        [][]string
		wantNumbers []int
		wantText    []string
	}{
		{
			name: "more than 2 lines take blocks with the same number", cues: [][]string{{"One", "Two", "Three"}, {"Four"}},
			wantNumbers: []int{16, 16, 32}, wantText: []string{"One\nTwo\nThree", "Four"},
		},
		{name: "a cue without lines writes one empty block", cues: [][]string{{}, {"Two"}}, wantNumbers: []int{16, 32}, wantText: []string{"", "Two"}},
		{name: "StripTags drops the italics", strip: true, cues: [][]string{{"<i>Plain</i> <b>text</b>"}}, wantNumbers: []int{16}, wantText: []string{"Plain text"}},
		{
			name: "entities and characters outside the table", cues: [][]string{{"a&lt;b € \u0e01 <b>x</b>"}},
			wantNumbers: []int{16}, wantText: []string{"a&lt;b € ? x"},
		},
		{
			name: "Chinese with a surrogate pair and italics", variant: "chinese", language: "\x90", cues: [][]string{{"<i>a\U0001F600</i>b"}},
			wantNumbers: []int{16}, wantText: []string{"<i>a\U0001F600</i>b"},
		},
		{
			name: "Greek writes 2 characters as one code", variant: "greek", language: "\x70", cues: [][]string{{"δια"}},
			wantNumbers: []int{16}, wantText: []string{"δια"},
		},
		{
			name: "Russian has no italics", variant: "russian", language: "\x56", cues: [][]string{{"<i>АБ</i>"}},
			wantNumbers: []int{16}, wantText: []string{"АБ"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := withFileData(t)
			if tt.variant != "" {
				s = withFileData(t, model.Field{Key: "variant", Value: tt.variant},
					model.Field{Key: "header", Value: hex.EncodeToString(header(t, map[int]string{146: tt.language}))})
			}
			for i, lines := range tt.cues {
				s.AddCue(cue(at(i), at(i+1), lines...))
			}
			out := write(t, s, model.CommonWriteOptions{StripTags: tt.strip})

			assert.Equal(t, tt.wantNumbers, blockNumbers(out))
			back, _, err := read(t, out, false)
			require.NoError(t, err)
			assert.Equal(t, tt.wantText, texts(back))
		})
	}
}

func at(i int) time.Duration { return time.Duration(i+1) * second }

func TestWriteErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cues    int
		lines   string
		wantCue int
		want    string
	}{
		{
			name: "a line of 52 bytes", cues: 2, lines: strings.Repeat("a", 52), wantCue: 1,
			want: `Cavena 890 holds 51 bytes per line, got 52 for "` + strings.Repeat("a", 52) + `".`,
		},
		{
			name: "a line of 52 bytes after the italics", cues: 1, lines: "<i>" + strings.Repeat("a", 50) + "</i>", wantCue: 0,
			want: `Cavena 890 holds 51 bytes per line, got 52 for "<i>` + strings.Repeat("a", 50) + `</i>".`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			for i := range tt.cues {
				line := "ok"
				if i == tt.wantCue {
					line = tt.lines
				}
				s.AddCue(cue(at(i), at(i+1), line))
			}
			_, err := cavena890.Write(&bytes.Buffer{}, s, model.CommonWriteOptions{}, cavena890.WriteOptions{})

			var uerr *model.UnwritableError
			require.ErrorAs(t, err, &uerr)
			assert.Equal(t, tt.wantCue, uerr.CueIndex)
			assert.Equal(t, tt.want, uerr.Reason)
		})
	}
}

func TestWriteNumbersPastTheStep(t *testing.T) {
	t.Parallel()

	newCues := func(n int) *model.Subtitle {
		s := &model.Subtitle{}
		for i := range n {
			s.AddCue(cue(time.Duration(i)*second, time.Duration(i+1)*second, "Cue "+strconv.Itoa(i)))
		}
		return s
	}

	var out bytes.Buffer
	_, err := cavena890.Write(&out, newCues(4110), model.CommonWriteOptions{}, cavena890.WriteOptions{})
	require.NoError(t, err)
	numbers := blockNumbers(out.Bytes())
	require.Len(t, numbers, 4110)
	assert.Equal(t, []int{16, 32, 48}, numbers[:3])
	assert.Equal(t, []int{65504, 65520, 65521, 65522}, numbers[4093:4097])
	assert.Equal(t, 65535, numbers[4109])
	back, _, err := read(t, out.Bytes(), false)
	require.NoError(t, err)
	assert.Equal(t, 4110, back.Len())

	_, err = cavena890.Write(&bytes.Buffer{}, newCues(4111), model.CommonWriteOptions{}, cavena890.WriteOptions{})
	var uerr *model.UnwritableError
	require.ErrorAs(t, err, &uerr)
	assert.Equal(t, 4110, uerr.CueIndex)
	assert.Equal(t, "Cavena 890 allows subtitle numbers up to 65535.", uerr.Reason)
}

func TestWriteTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		start, end time.Duration
		want       string
	}{
		{"frames at 25 fps", 1 * second, 4 * second, "000019000064"},
		{"a negative start is 0", -second, second, "000000000019"},
		{"a frame rounds to the nearest", 1019 * time.Millisecond, 1021 * time.Millisecond, "000019" + "00001a"},
		{"the end is capped at FFFFFFh", 0, 1000 * time.Hour, "000000ffffff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := write(t, subtitle(cue(tt.start, tt.end, "x")), model.CommonWriteOptions{})
			assert.Equal(t, tt.want, hex.EncodeToString(out[388+2:388+8]))
		})
	}
}

func TestWriteIgnoresBadStoredValues(t *testing.T) {
	t.Parallel()

	newFile := write(t, subtitle(cue(second, 2*second, "One")), model.CommonWriteOptions{})
	tests := []struct {
		name  string
		file  []model.Field
		cue   []model.Field
		match bool
	}{
		{name: "unknown variant", file: []model.Field{{Key: "variant", Value: "klingon"}}},
		{name: "header that is not hex", file: []model.Field{{Key: "header", Value: "zz"}}},
		{name: "header of 10 bytes", file: []model.Field{{Key: "header", Value: strings.Repeat("00", 10)}}},
		{name: "blocks that are not hex", cue: []model.Field{{Key: "text", Value: "One"}, {Key: "blocks", Value: []any{"zz"}}}},
		{name: "blocks of 10 bytes", cue: []model.Field{{Key: "text", Value: "One"}, {Key: "blocks", Value: []any{strings.Repeat("00", 10)}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := withFileData(t, tt.file...)
			c := cue(second, 2*second, "One")
			if tt.cue != nil {
				require.NoError(t, c.SetFormatData("cavena890", model.NewData(tt.cue...)))
			}
			s.AddCue(c)

			assert.Equal(t, newFile, write(t, s, model.CommonWriteOptions{}))
		})
	}
}

func TestWriteKeepsWhatReadStored(t *testing.T) {
	t.Parallel()

	danish := header(t, map[int]string{146: "\x07"})
	file := slices.Concat(danish, block(100, 0x51, "", "Hej"))
	file = append(file, block(7, 0x50, "", "Ja")...)
	copy(file[388+128+2:], []byte{0, 0, 250, 0, 0, 255}) // the second cue starts at 10 s, so editing the first keeps the order
	stored, _, err := read(t, file, false)
	require.NoError(t, err)

	tests := []struct {
		name        string
		edit        func(c *model.Cue)
		wantHead    string
		keepsRest   bool
		wantNumbers []int
		wantText    []string
	}{
		{name: "unchanged", edit: func(*model.Cue) {}, wantHead: "006400001900006451", keepsRest: true, wantNumbers: []int{100, 116}, wantText: []string{"Hej", "Ja"}},
		{
			name: "new times and a new alignment", wantHead: "006400003200004b52", keepsRest: true, wantNumbers: []int{100, 116}, wantText: []string{"Hej", "Ja"},
			edit: func(c *model.Cue) { c.Start, c.End, c.Position.Alignment = 2*second, 3*second, model.BottomRight },
		},
		{
			name: "a stored alignment byte stays when it means the same alignment", wantHead: "006400003200006451", keepsRest: true,
			wantNumbers: []int{100, 116}, wantText: []string{"Hej", "Ja"}, edit: func(c *model.Cue) { c.Start = 2 * second },
		},
		{
			name: "new text writes new blocks with the table of the file", wantHead: "001000001900006454", wantNumbers: []int{16, 32},
			wantText: []string{"Hej \u00e5\u00e6", "Ja"}, edit: func(c *model.Cue) { c.SetText("Hej \u00e5\u00e6") },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := stored.Clone()
			c, _ := s.Cue(0)
			tt.edit(&c)
			require.NoError(t, s.SetCue(0, c))
			out := write(t, s, model.CommonWriteOptions{})

			assert.Equal(t, byte(0x07), out[146])
			assert.Equal(t, tt.wantHead, hex.EncodeToString(out[388:388+9]))
			assert.Equal(t, tt.wantNumbers, blockNumbers(out))
			if tt.keepsRest {
				assert.True(t, bytes.Equal(file[388+9:388+128], out[388+9:388+128]), "the bytes after the alignment change")
			}
			back, _, err := read(t, out, false)
			require.NoError(t, err)
			assert.Equal(t, tt.wantText, texts(back))
		})
	}
}

func TestWriteKeepsTheVariantOfAFile(t *testing.T) {
	t.Parallel()

	s, _, err := read(t, testfiles.Read(t, "cavena890/real/train_russian.890"), false)
	require.NoError(t, err)
	c, _ := s.Cue(0)
	c.SetText("\u0414\u0430, Ivan.")
	require.NoError(t, s.SetCue(0, c))

	back, _, err := read(t, write(t, s, model.CommonWriteOptions{}), false)
	require.NoError(t, err)
	first, _ := back.Cue(0)
	assert.Equal(t, "\u0414\u0430, ????.", first.Text())
}
