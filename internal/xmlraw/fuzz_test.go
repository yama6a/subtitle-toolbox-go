package xmlraw_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
)

func FuzzParse(f *testing.F) {
	for _, dir := range []string{"ttml/real", "forced", "lenient"} {
		entries, err := os.ReadDir(testfiles.Path(f, dir))
		if err != nil {
			f.Fatal(err)
		}
		for _, e := range entries {
			if !e.IsDir() && e.Name() != "SOURCES.md" {
				f.Add(testfiles.Read(f, dir+"/"+e.Name()))
			}
		}
	}
	for _, seed := range []string{
		"<tt><x a=\"a&#13;b\tc\"/>a&#13;b<![CDATA[]]>]]&gt;<?pi?><!--a--->x</tt>",
		"<tt>\n<p>x</div>\n</tt>",
		"<tt><p>a<span>b</p></div></tt>",
		"<!DOCTYPE tt [<!ENTITY a \"b\">]><a:b:c/>",
		"<!DOCTYPE tt [<!-- it's -->]><tt a=\"1\n2\"/>",
		"<!DOCTYPE tt [<!-- [ -->]><tt a=\"1\n2\"/>",
		"<tt><!--> <x a=\"1\n2\"/> --><!---><y a='3\t4&#10;'/>--></tt>",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, content []byte) {
		if _, err := xmlraw.Parse(bytes.NewReader(content), xmlraw.ParseOptions{Recover: true}); err != nil {
			var serr *xmlraw.SyntaxError
			if !errors.As(err, &serr) {
				t.Fatalf("recover mode returned %v, which is no *SyntaxError", err)
			}
		}
		doc, err := xmlraw.Parse(bytes.NewReader(content), xmlraw.ParseOptions{})
		if err != nil {
			var serr *xmlraw.SyntaxError
			if !errors.As(err, &serr) {
				t.Fatalf("Parse returned %v, which is no *SyntaxError", err)
			}
			return
		}
		var out bytes.Buffer
		if err := xmlraw.Serialize(&out, doc); err != nil {
			t.Fatal(err)
		}
		again, err := xmlraw.Parse(bytes.NewReader(out.Bytes()), xmlraw.ParseOptions{})
		if err != nil {
			t.Fatalf("the output %q does not parse: %v", out.String(), err)
		}
		if !sameTree(doc, again) {
			t.Fatalf("the output %q parses to another tree", out.String())
		}
	})
}

// sameTree reports whether b is the tree of the output of a. The output writes the xmlns declarations first.
func sameTree(a, b *xmlraw.Node) bool {
	if a.Kind != b.Kind || a.Name != b.Name || a.Data != b.Data || !slices.Equal(declsFirst(a.Attrs), b.Attrs) || len(a.Children) != len(b.Children) {
		return false
	}
	for i := range a.Children {
		if !sameTree(a.Children[i], b.Children[i]) {
			return false
		}
	}
	return true
}

func declsFirst(attrs []xml.Attr) []xml.Attr {
	decls := slices.DeleteFunc(slices.Clone(attrs), func(a xml.Attr) bool { return !xmlraw.IsNamespaceDecl(a) })
	return slices.Concat(decls, slices.DeleteFunc(slices.Clone(attrs), xmlraw.IsNamespaceDecl))
}
