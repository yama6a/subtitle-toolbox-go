package transcript

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestScanElements(t *testing.T) {
	t.Parallel()

	type want struct {
		name  int
		inner string
	}
	tests := []struct {
		name, in string
		want     []want
	}{
		{"three elements", "<cite>A:</cite><time>0:01</time><p>Hi</p>", []want{{nameCite, "A:"}, {nameTime, "0:01"}, {nameP, "Hi"}}},
		{"start tag takes the first end tag", "<p>one<p>two</p></p>", []want{{nameP, "one<p>two"}}},
		{"nested p in cite", "<cite><p>x</p></cite>", []want{{nameCite, "<p>x</p>"}}},
		{"pre and timeline do not match", "<pre>a</pre><timeline>b</timeline><p>c</p>", []want{{nameP, "c"}}},
		{"attributes with a slash", `<p class="a/b" data-x='1/2'>t</p>`, []want{{nameP, "t"}}},
		{"end tag with a space", "<p>t</p ><P>u</P\n>", []want{{nameP, "t"}, {nameP, "u"}}},
		{"end tag with an attribute does not close", "<p>a</p x></p>", []want{{nameP, "a</p x>"}}},
		{"upper case names", "<CITE>A</Cite>", []want{{nameCite, "A"}}},
		{"start tag without a greater-than sign", "<p class=a", nil},
		{"element after a failed one", "<time>x<p>y</p>", []want{{nameP, "y"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var got []want
			for _, el := range scanElements(tt.in) {
				got = append(got, want{el.name, el.inner})
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestScanElementsOfUnclosedTagsFinishes(t *testing.T) {
	t.Parallel()

	for _, in := range []string{
		strings.Repeat("<p>", 2000),
		strings.Repeat("<p ", 2000),
		strings.Repeat("<p>a</pp>", 2000),
		strings.Repeat("<p></p", 2000) + strings.Repeat(" ", 2000),
	} {
		assert.Empty(t, scanElements(in))
	}
}

func TestShortClock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   time.Duration
		want string
	}{
		{-5 * time.Second, "0:00"},
		{999 * time.Millisecond, "0:00"},
		{59*time.Minute + 59999*time.Millisecond, "59:59"},
		{time.Hour, "1:00:00"},
		{3725 * time.Second, "1:02:05"},
		{99999*time.Hour + 59*time.Minute + 59*time.Second, "99999:59:59"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, shortClock(tt.in), tt.in.String())
	}
}
