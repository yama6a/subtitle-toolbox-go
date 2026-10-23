package writing_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

const bom = "\ufeff"

// Ports the cases of tests/Formatters/OutputOptionsTest.php of subtitle-toolbox-php that the output rules decide.
func TestFinish(t *testing.T) {
	t.Parallel()

	const body = "1\n00:00:01,500 --> 00:00:04,000\n<i>Bonjour</i>\nca va ?\n"
	const crlf = "1\r\n00:00:01,500 --> 00:00:04,000\r\n<i>Bonjour</i>\r\nca va ?\r\n"
	tests := []struct {
		name      string
		body      string
		common    model.CommonWriteOptions
		formatBOM bool
		want      string
	}{
		{name: "defaults without a format BOM", body: body, want: body},
		{name: "defaults with a format BOM", body: body, formatBOM: true, want: bom + body},
		{name: "LF keeps the output", body: body, common: model.CommonWriteOptions{LineEnding: model.LF}, want: body},
		{name: "CRLF replaces every LF", body: body, common: model.CommonWriteOptions{LineEnding: model.CRLF}, want: crlf},
		{name: "CRLF keeps a CR LF of the writer", body: "a\r\nb\n", common: model.CommonWriteOptions{LineEnding: model.CRLF}, want: "a\r\nb\r\n"},
		{name: "CRLF keeps a lone CR", body: "a\rb\n", common: model.CommonWriteOptions{LineEnding: model.CRLF}, want: "a\rb\r\n"},
		{name: "CRLF of empty output", common: model.CommonWriteOptions{LineEnding: model.CRLF}, want: ""},
		{name: "BOMAdd", body: body, common: model.CommonWriteOptions{BOM: model.BOMAdd}, want: bom + body},
		{name: "BOMAdd with a format BOM", body: body, common: model.CommonWriteOptions{BOM: model.BOMAdd}, formatBOM: true, want: bom + body},
		{name: "BOMAdd keeps one BOM", body: bom + body, common: model.CommonWriteOptions{BOM: model.BOMAdd}, want: bom + body},
		{name: "BOMAdd to empty output", common: model.CommonWriteOptions{BOM: model.BOMAdd}, want: bom},
		{name: "default format BOM keeps one BOM", body: bom + body, formatBOM: true, want: bom + body},
		{name: "BOMRemove with a format BOM", body: body, common: model.CommonWriteOptions{BOM: model.BOMRemove}, formatBOM: true, want: body},
		{name: "BOMRemove removes every leading BOM", body: bom + bom + body, common: model.CommonWriteOptions{BOM: model.BOMRemove}, want: body},
		{name: "BOMRemove keeps a later BOM", body: "a\n" + bom, common: model.CommonWriteOptions{BOM: model.BOMRemove}, want: "a\n" + bom},
		{name: "default without a format BOM keeps a BOM of the writer", body: bom + body, want: bom + body},
		{name: "CRLF and BOMAdd", body: body, common: model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd}, want: bom + crlf},
		{name: "CRLF and BOMRemove", body: bom + body, common: model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMRemove}, formatBOM: true, want: crlf},
		{name: "StripTags leaves the output alone", body: body, common: model.CommonWriteOptions{StripTags: true}, want: body},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := []byte(tt.body)
			assert.Equal(t, tt.want, string(writing.Finish(in, tt.common, tt.formatBOM)))
			assert.Equal(t, tt.body, string(in), "Finish changed its input")
		})
	}
}

type countingWriter struct {
	out   []byte
	calls int
	err   error
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.err != nil {
		return 0, w.err
	}
	w.out = append(w.out, p...)
	return len(p), nil
}

func TestWriteWritesOnce(t *testing.T) {
	t.Parallel()

	var w countingWriter
	err := writing.Write(&w, model.SubRip, []byte("a\nb\n"), model.CommonWriteOptions{LineEnding: model.CRLF}, true)
	require.NoError(t, err)
	assert.Equal(t, 1, w.calls)
	assert.Equal(t, bom+"a\r\nb\r\n", string(w.out))
}

func TestWriteWrapsTheWriterError(t *testing.T) {
	t.Parallel()

	errFull := errors.New("disk full")
	w := countingWriter{err: errFull}
	err := writing.Write(&w, model.SubRip, []byte("a\n"), model.CommonWriteOptions{}, false)
	require.ErrorIs(t, err, errFull)
	assert.EqualError(t, err, "write srt: disk full")
}

func TestPipeText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"no lines", nil, ""},
		{"lines without pipes", []string{"a", "b"}, "a|b"},
		{"pipes in lines", []string{"Rock | Pop", "|"}, "Rock \u00a6 Pop|\u00a6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, writing.PipeText(tt.lines))
		})
	}
}
