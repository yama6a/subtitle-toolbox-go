package mpl2_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/format/mpl2"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testWritesTenthsOfASecondAndLineBreaks, testWritesAnItalicLineWithASlash and
// testStripsOtherTagsAndDecodesEntities of tests/Formatters/Mpl2FormatterTest.php of subtitle-toolbox-php,
// then adds rows of its own.
func TestWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cues []model.Cue
		want string
	}{
		{
			"tenths round half away from zero",
			[]model.Cue{{Start: 1240 * ms, End: 4460 * ms, Lines: []string{"Where are you?", "Home."}}},
			"[12][45]Where are you?|Home.\n",
		},
		{
			"a half tenth rounds up",
			[]model.Cue{{Start: 1250 * ms, End: 1349 * ms, Lines: []string{"x"}}},
			"[13][13]x\n",
		},
		{"a start below 0 is 0", []model.Cue{{Start: -2 * time.Second, End: -time.Second, Lines: []string{"x"}}}, "[0][0]x\n"},
		{
			"the largest time",
			[]model.Cue{{Start: 0, End: timecodeMaxValid, Lines: []string{"x"}}},
			"[0][3600000000]x\n",
		},
		{"a cue without text", []model.Cue{{Start: time.Second, End: 2 * time.Second}}, "[10][20]\n"},
		{"a cue with lines that hold no text", []model.Cue{{Start: 0, End: time.Second, Lines: []string{"<b></b>", "<v Anna>"}}}, "[0][10]\n"},
		{"italic first line", []model.Cue{{Start: time.Second, End: 2 * time.Second, Lines: []string{"<i>One</i>", "Two"}}}, "[10][20]/One|Two\n"},
		{
			"italic carries over to the next line",
			[]model.Cue{{Start: 3 * time.Second, End: 4 * time.Second, Lines: []string{"<i>Three", "Four</i>"}}},
			"[30][40]/Three|/Four\n",
		},
		{
			"nested tags and a partly italic line",
			[]model.Cue{{Start: 5 * time.Second, End: 6 * time.Second, Lines: []string{"<b><i>Five</i></b>", "<i>Six</i> and seven"}}},
			"[50][60]/Five|Six and seven\n",
		},
		{
			"other tags and entities",
			[]model.Cue{{Start: 0, End: time.Second, Lines: []string{`<font color="#ff0000">Red</font> &amp; <v Anna>blue`, "<b> </b>"}}},
			"[0][10]Red & blue\n",
		},
		{"italic tags in any case", []model.Cue{{Start: 0, End: time.Second, Lines: []string{"<I>One</I>", "<i>Two</I>"}}}, "[0][10]/One|/Two\n"},
		{
			"the italic state runs over a line without text",
			[]model.Cue{{Start: 0, End: time.Second, Lines: []string{"<i>One", "<b></b>", "Two"}}},
			"[0][10]/One|/Two\n",
		},
		{
			"an italic tag with attributes is no italic tag",
			[]model.Cue{{Start: 0, End: time.Second, Lines: []string{"<i class=a>One</i>"}}},
			"[0][10]One\n",
		},
		{"a ruby annotation is text", []model.Cue{{Start: 0, End: time.Second, Lines: []string{"<ruby>Tokyo<rt>to</rt></ruby>"}}}, "[0][10]Tokyo (to)\n"},
		{
			"two cues",
			[]model.Cue{{Start: 0, End: time.Second, Lines: []string{"a"}}, {Start: time.Second, End: 2 * time.Second, Lines: []string{"b"}}},
			"[0][10]a\n[10][20]b\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, write(t, subtitle(tt.cues...), model.CommonWriteOptions{}))
		})
	}
}

const timecodeMaxValid = 100000*time.Hour - ms

func TestWriteEmptySubtitle(t *testing.T) {
	t.Parallel()

	assert.Empty(t, write(t, &model.Subtitle{}, model.CommonWriteOptions{}))
}

// Ports testAppliesTheOutputOptions of tests/Formatters/Mpl2FormatterTest.php of subtitle-toolbox-php.
func TestWriteOutputOptions(t *testing.T) {
	t.Parallel()

	s := subtitle(model.Cue{End: time.Second, Lines: []string{"Hello"}})
	assert.Equal(t, "\xEF\xBB\xBF[0][10]Hello\r\n", write(t, s, model.CommonWriteOptions{LineEnding: model.CRLF, BOM: model.BOMAdd}))
	assert.Equal(t, "[0][10]Hello\n", write(t, s, model.CommonWriteOptions{}))
	assert.Equal(t, "[0][10]Hello\n", write(t, s, model.CommonWriteOptions{BOM: model.BOMRemove}))
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestWriteReturnsTheWriterError(t *testing.T) {
	t.Parallel()

	err := mpl2.Write(failingWriter{}, subtitle(model.Cue{Lines: []string{"x"}}), model.CommonWriteOptions{}, mpl2.WriteOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}
