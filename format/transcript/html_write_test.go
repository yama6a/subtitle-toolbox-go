package transcript_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func writeHTML(t *testing.T, s *model.Subtitle, opts transcript.HTMLWriteOptions) string {
	t.Helper()

	var out bytes.Buffer
	require.NoError(t, transcript.WriteHTML(&out, s, model.CommonWriteOptions{}, transcript.WriteOptions{HTML: opts}))
	return out.String()
}

func htmlDialogue() *model.Subtitle {
	s := &model.Subtitle{}
	s.AddCues(
		model.Cue{Start: time.Second, End: 2 * time.Second, Lines: []string{"<v Anna><b>Hello</b>", "there."}},
		model.Cue{Start: 2500 * time.Millisecond, End: 4 * time.Second, Lines: []string{"<v Anna>Where are you going?"}},
		model.Cue{Start: 4 * time.Second, End: 5 * time.Second, Lines: []string{"<v Ben>Home &amp; <00:00:04.500>bed.", "<v Anna>Why?"}},
		model.Cue{Start: 9 * time.Second, End: 10 * time.Second, Lines: []string{"<v Anna>Fish &lt;3"}},
		model.Cue{Start: 3725 * time.Second, End: 3727 * time.Second, Lines: []string{"No speaker."}},
	)
	return s
}

func TestWriteHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts transcript.HTMLWriteOptions
		want string
	}{
		{"default gap", transcript.HTMLWriteOptions{}, "<cite>Anna:</cite>\n<time>0:01</time>\n<p>Hello there. Where are you going?</p>\n" +
			"<cite>Ben:</cite>\n<time>0:04</time>\n<p>Home &amp; bed.</p>\n" +
			"<cite>Anna:</cite>\n<time>0:04</time>\n<p>Why?</p>\n" +
			"<cite>Anna:</cite>\n<time>0:09</time>\n<p>Fish &lt;3</p>\n" +
			"<time>1:02:05</time>\n<p>No speaker.</p>\n"},
		{
			"gap of 0.5 s",
			transcript.HTMLWriteOptions{ParagraphGap: new(500 * time.Millisecond)},
			"<cite>Anna:</cite>\n<time>0:01</time>\n<p>Hello there.</p>\n<cite>Anna:</cite>\n<time>0:02</time>\n<p>Where are you going?</p>\n",
		},
		{
			"no paragraph breaks",
			transcript.HTMLWriteOptions{ParagraphGap: new(transcript.NoParagraphBreaks)},
			"<cite>Anna:</cite>\n<time>0:01</time>\n<p>Hello there. Where are you going?</p>\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.True(t, strings.HasPrefix(writeHTML(t, htmlDialogue(), tt.opts), tt.want), tt.name)
		})
	}
}

func TestWriteHTMLEscapesAndClamps(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	s.AddCue(model.Cue{Start: 0, End: time.Second, Lines: []string{"<v Tom &amp; Jerry>Hi"}})
	assert.Equal(t, "<cite>Tom &amp; Jerry:</cite>\n<time>0:00</time>\n<p>Hi</p>\n", writeHTML(t, s, transcript.HTMLWriteOptions{}))
}

func TestHTMLWriteOptionsValidate(t *testing.T) {
	t.Parallel()

	require.NoError(t, transcript.HTMLWriteOptions{ParagraphGap: new(time.Duration(0))}.Validate())
	err := transcript.HTMLWriteOptions{ParagraphGap: new(-time.Second)}.Validate()
	assert.Equal(t, &model.InvalidArgumentError{Option: "WriteOptions.Transcript.HTML.ParagraphGap", Msg: "The paragraph gap must be 0 or more seconds, got -1."}, err)

	var out bytes.Buffer
	werr := transcript.WriteHTML(&out, htmlDialogue(), model.CommonWriteOptions{}, transcript.WriteOptions{HTML: transcript.HTMLWriteOptions{ParagraphGap: new(-time.Second)}})
	assert.ErrorIs(t, werr, model.ErrInvalidArgument)
}

// Ports testFormatterOutputRoundTripsByteForByte of tests/Parsers/HtmlTranscriptRealFilesTest.php.
func TestWriteHTMLRoundTripsRealFiles(t *testing.T) {
	t.Parallel()

	for _, file := range []string{"spec_example_shape.html", "hhmmss_empty_paragraphs.html"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()

			s, _ := readHTML(t, string(testfiles.Read(t, "html/real/"+file)), model.CommonReadOptions{})
			first := writeHTML(t, s, transcript.HTMLWriteOptions{})
			again, _ := readHTML(t, first, model.CommonReadOptions{})
			assert.Equal(t, first, writeHTML(t, again, transcript.HTMLWriteOptions{}))
		})
	}
}

func TestDetectHTML(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in string
		want     bool
	}{
		{"cite and time", "<cite>Anna:</cite>\n<time>0:00</time>\n<p>Hello</p>\n", true},
		{"document in upper case", "<!DOCTYPE html>\n<html><body>\n<CITE>Anna:</CITE>\n<TIME>0:00</TIME></body></html>", true},
		{"tags with attributes", "<?xml version=\"1.0\"?><div><cite class=\"a\">A</cite><time\n>0:00</time>", true},
		{"white space first", " \n\t<time>0:00</time><cite>A", true},
		{"output without speakers", "<time>0:01</time>\n<p>Hello</p>\n", false},
		{"no time", "<cite>Anna:</cite>\n<p>Hello</p>\n", false},
		{"timeline is no time", "<cite>A</cite><timeline>", false},
		{"text before the first tag", "Hello <cite> and <time>\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, transcript.DetectHTML([]byte(tt.in)))
		})
	}
}
