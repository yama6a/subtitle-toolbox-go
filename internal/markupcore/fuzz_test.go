package markupcore_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
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
