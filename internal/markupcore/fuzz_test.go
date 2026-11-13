package markupcore_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
)

func FuzzMarkup(f *testing.F) {
	f.Add(`<v O'Neil><i>Tom &amp; Jerry</i> <00:00:01.000>a < b > c &nbsp;`)
	f.Add("<c.yellow>&#233;&#x10FFFF;&notit;&amp</c><>< b>")
	f.Add("caf\xe9 &lt;<ruby>K<rt>kan</ruby>")

	f.Fuzz(func(t *testing.T, s string) {
		if got := strings.Join(markupcore.SplitTags(s), ""); got != s {
			t.Fatalf("SplitTags(%q) joins to %q", s, got)
		}
		if n := len(markupcore.SplitTags(s)); n%2 != 1 {
			t.Fatalf("SplitTags(%q) has %d parts", s, n)
		}
		markupcore.StripAllTags(s)
		markupcore.DecodeEntities(s)
		identity := func(text string, _, _ bool) string { return text }
		if got := markupcore.MapTextRuns([]string{s}, identity); got[0] != s {
			t.Fatalf("MapTextRuns(%q) with the identity gives %q", s, got[0])
		}
	})
}

func FuzzEmbedRightToLeft(f *testing.F) {
	data := testfiles.Read(f, "rtl/embed.json")
	var rows []struct {
		In string `json:"in"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		f.Fatal(err)
	}
	for _, row := range rows {
		f.Add(row.In)
	}

	f.Fuzz(func(t *testing.T, s string) {
		once := markupcore.EmbedRightToLeft(s)
		if twice := markupcore.EmbedRightToLeft(once); twice != once {
			t.Fatalf("EmbedRightToLeft(%q) = %q, then %q", s, once, twice)
		}
		markupcore.HasRightToLeftLetter(s)
		markupcore.HoldsRightToLeftEmbedding([]string{s, once})
	})
}

func FuzzWordTags(f *testing.F) {
	for _, s := range []string{
		"<00:00:01.000>Hi there",
		"Hi <00:00:02.000>there",
		"<00:00:01.000><c> beach</c>",
		"<00:00:01.000>Hel<00:00:01.400>lo",
		"Well, <00:00:02.000>yes",
		"<v Anna>Good <00:00:01.000>morning.",
		"<v Anna>Hi.</v>\n<v Ben>Hello.</v>",
		"<00:00:01.000>Hi\n<00:00:04.000>",
		"<00:00:03.000>a <00:00:02.000>b <00:00:04.000>c",
		"<v.loud Anna>Hi</v>",
		"<v Guide>Please keep your hands\n<v Guide>inside the boat.",
		"<v Anna>Hi</v> there",
		"the<00:05:04.199><c> train</c><c.colorE5E5E5><00:05:04.379><c> leaves</c></c>",
	} {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		const cueEnd = 5 * time.Second
		got := markupcore.ExtractWordTags(strings.Split(s, "\n"), cueEnd, true)
		texts := make([]string, len(got.Words))
		for i, w := range got.Words {
			texts[i] = w.Text
			if w.Text == "" || strings.TrimFunc(w.Text, markupcore.IsSpace) != w.Text || got.Ends[i] < w.Start {
				t.Fatalf("word %d of %q is %+v with end %v", i, s, w, got.Ends[i])
			}
		}
		plain := markupcore.CuePlainText(got.Lines)
		if n := len(markupcore.FindWords(plain, texts)); n != len(texts) {
			t.Fatalf("%q: %d of %d words occur in %q", s, n, len(texts), plain)
		}
		again := markupcore.ExtractWordTags(markupcore.TagWords(got.Lines, got.Words, got.Voice), cueEnd, true)
		if !slices.Equal(again.Lines, got.Lines) || again.Voice != got.Voice {
			t.Fatalf("%q: lines %q voice %+v, then lines %q voice %+v", s, got.Lines, got.Voice, again.Lines, again.Voice)
		}
	})
}
