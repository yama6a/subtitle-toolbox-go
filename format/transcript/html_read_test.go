package transcript_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/transcript"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func readHTML(t *testing.T, content string, common model.CommonReadOptions) (*model.Subtitle, []model.ParseWarning) {
	t.Helper()

	s, warnings, err := transcript.ReadHTML(strings.NewReader(content), common, transcript.ReadOptions{})
	require.NoError(t, err)
	return s, warnings
}

type htmlCue struct {
	start, end time.Duration
	speaker    string
	lines      []string
}

func describeHTML(s *model.Subtitle) []htmlCue {
	out := make([]htmlCue, 0, s.Len())
	for _, c := range s.Cues() {
		out = append(out, htmlCue{c.Start, c.End, c.Speaker, c.Lines})
	}
	return out
}

func TestReadHTML(t *testing.T) {
	t.Parallel()

	document := "<!DOCTYPE html>\r\n<html>\r\n<head><title>Episode 4</title></head>\r\n<body>\r\n" +
		"  <cite>Anna:</cite>\r\n  <time>0:05</time>\r\n  <p class=\"line\">Hello<br>there.</p>\r\n  <p>Come <b>in</b>.</p>\r\n" +
		"  <time>1:02:03.5</time>\r\n  <P>&lt;3 &amp; bye</P>\r\n</body>\r\n</html>\r\n"
	tests := []struct {
		name, in string
		want     []htmlCue
	}{
		{"full document", document, []htmlCue{
			{5 * time.Second, 3723500 * time.Millisecond, "Anna", []string{"Hello", "there.", "Come in."}},
			{3723500 * time.Millisecond, 3728500 * time.Millisecond, "", []string{"&lt;3 &amp; bye"}},
		}},
		{"a cite names only the next paragraph", "<cite>Dr. O'Neil :</cite><time>0:00</time><p>Hi.</p><time>0:02</time><p>Bye.</p>", []htmlCue{
			{0, 2 * time.Second, "Dr. O'Neil", []string{"Hi."}},
			{2 * time.Second, 7 * time.Second, "", []string{"Bye."}},
		}},
		{"br forms and empty parts", "<time>0:01</time><p>a<BR/>b<br />c<br  >  <br></p>", []htmlCue{
			{time.Second, 6 * time.Second, "", []string{"a", "b", "c"}},
		}},
		{"a paragraph without text gives no cue", "<time>0:01</time><p> </p><time>0:02</time><p>x</p>", []htmlCue{
			{2 * time.Second, 7 * time.Second, "", []string{"x"}},
		}},
		{"equal starts end at the first later start", "<time>0:01</time><p>a</p><time>0:01</time><p>b</p><time>0:04</time><p>c</p>", []htmlCue{
			{time.Second, 4 * time.Second, "", []string{"a"}},
			{time.Second, 4 * time.Second, "", []string{"b"}},
			{4 * time.Second, 9 * time.Second, "", []string{"c"}},
		}},
		{"no elements", "<html><body>Hello</body></html>", []htmlCue{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, warnings := readHTML(t, tt.in, model.CommonReadOptions{})
			assert.Empty(t, warnings)
			assert.Equal(t, tt.want, describeHTML(s))
		})
	}
}

func TestReadHTMLErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, in, msg string
		line          int
		times         []string
	}{
		{"no time", "<time>0:00</time><p>Hi.</p>\n<cite>Ben:</cite>\n<p>Bye.</p>", "The paragraph has no <time>.", 2, nil},
		{"bad time", "<cite>Ben:</cite>\n\n<time>soon</time>\n<p>Bye.</p>", `The time "soon" is not valid.`, 3, nil},
		{"61 seconds", "<time>0:00</time><p>Hi.</p>\n<time>0:61</time><p>Oops.</p>", `The time "0:61" is not valid.`, 2, nil},
		{"two CRs before an LF count as one line break", "<time>0:01</time>\r\r\n<time>x</time><p>a</p>", `The time "x" is not valid.`, 2, nil},
		{"20 hour digits", "<time>99999999999999999999:00:04</time><p>x</p>", `The time "99999999999999999999:00:04" is not below 100000 hours.`, 1, nil},
		{"100000 hours", "<time>100000:00:00</time><p>x</p>", `The time "100000:00:00" is not below 100000 hours.`, 1, nil},
		{"rounds to 100000 hours", "<time>99999:59:59.9996</time><p>x</p>", `The time "99999:59:59.9996" is not below 100000 hours.`, 1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := transcript.ReadHTML(strings.NewReader(tt.in), model.CommonReadOptions{}, transcript.ReadOptions{})
			var perr *model.ParseError
			require.ErrorAs(t, err, &perr)
			assert.Equal(t, tt.msg, perr.Msg)
			assert.Equal(t, tt.line, perr.Line)
			assert.Equal(t, model.HTMLTranscript, perr.Format)
		})
	}
}

func TestReadHTMLTimes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want time.Duration
	}{
		{"0:09", 9 * time.Second},
		{"12:05", 12*time.Minute + 5*time.Second},
		{"1:02:03.5", time.Hour + 2*time.Minute + 3500*time.Millisecond},
		{"00:00:00", 0},
		{"90:00", 90 * time.Minute},
		{"0:05,25", 5250 * time.Millisecond},
		{"0:00.0005", time.Millisecond},
		{"99999:59:59.999", 99999*time.Hour + 59*time.Minute + 59999*time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			s, _ := readHTML(t, "<time>"+tt.in+"</time><p>x</p>", model.CommonReadOptions{})
			c, ok := s.Cue(0)
			require.True(t, ok)
			assert.Equal(t, tt.want, c.Start)
		})
	}
}

func TestReadHTMLLenient(t *testing.T) {
	t.Parallel()

	s, warnings := readHTML(t, "<time>0:00</time><p>Hi.</p>\n<time>0:61</time><p>Oops.</p>\n<time>0:04</time><p>Bye.</p>", model.CommonReadOptions{Lenient: true})
	assert.Equal(t, []htmlCue{
		{0, 4 * time.Second, "", []string{"Hi."}},
		{4 * time.Second, 9 * time.Second, "", []string{"Bye."}},
	}, describeHTML(s))
	require.Len(t, warnings, 1)
	assert.Equal(t, model.ParseWarning{
		Message: `The time "0:61" is not valid.`, Line: 2, Block: 1, Action: model.Skipped, Lines: []string{"0:61"},
	}, warnings[0])
}

func TestReadHTMLLastCueDuration(t *testing.T) {
	t.Parallel()

	s, _ := readHTML(t, "<time>0:00</time><p>Hi.</p>", model.CommonReadOptions{LastCueDuration: new(3 * time.Second)})
	assert.Equal(t, 3*time.Second, s.Cues()[0].End)
}
