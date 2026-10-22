package xmlraw_test

import (
	"encoding/xml"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/internal/xmlraw"
)

func parse(t *testing.T, s string, recovering bool) *xmlraw.Node {
	t.Helper()

	doc, err := xmlraw.Parse(strings.NewReader(s), xmlraw.ParseOptions{Recover: recovering})
	require.NoError(t, err)
	return doc
}

func serialize(t *testing.T, n *xmlraw.Node) string {
	t.Helper()

	var b strings.Builder
	require.NoError(t, xmlraw.Serialize(&b, n))
	return b.String()
}

func root(t *testing.T, doc *xmlraw.Node) *xmlraw.Node {
	t.Helper()

	r, ok := doc.Root()
	require.True(t, ok)
	return r
}

// firstChild returns the first child of the root, which the serialiser tests write.
func firstChild(t *testing.T, s string) string {
	t.Helper()

	r := root(t, parse(t, s, false))
	require.NotEmpty(t, r.Children)
	return serialize(t, r.Children[0])
}

func TestSerialize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"own declarations first, no declaration of an ancestor",
			`<tt xmlns="urn:a" xmlns:tts="urn:s"><region tts:x="1" xmlns:q="urn:q" a="2"/></tt>`,
			`<region xmlns:q="urn:q" tts:x="1" a="2"/>`,
		},
		{"empty element", `<tt><x></x></tt>`, `<x/>`},
		{"xmlns with a prefix that is no NCName stays in its place", `<tt><x a="1" xmlns:0="u"/></tt>`, `<x a="1" xmlns:0="u"/>`},
		{"text escapes", `<tt><x>a &amp; &lt;b&gt; "c" 'd'</x></tt>`, `<x>a &amp; &lt;b&gt; "c" 'd'</x>`},
		{"apostrophe in text", `<tt><x>it&apos;s</x></tt>`, `<x>it's</x>`},
		{"apostrophe in an attribute", `<tt><x a="it&apos;s"/></tt>`, `<x a="it's"/>`},
		{"quote in an attribute", `<tt><x a='say "hi"'/></tt>`, `<x a="say &quot;hi&quot;"/>`},
		{"attribute escapes", `<tt><x a="&amp;&lt;&gt;"/></tt>`, `<x a="&amp;&lt;&gt;"/>`},
		{"referenced CR in text", `<tt><x>a&#13;b</x></tt>`, `<x>a&#13;b</x>`},
		{"referenced CR in an attribute", `<tt><x a="a&#13;b"/></tt>`, `<x a="a&#13;b"/>`},
		{"literal LF and TAB in an attribute become spaces", "<tt><x a=\"a\nb\tc\"/></tt>", `<x a="a b c"/>`},
		{"literal CR LF in an attribute becomes one space", "<tt><x a=\"a\r\nb\"/></tt>", `<x a="a b"/>`},
		{"character reference in text", `<tt><x>caf&#233;</x></tt>`, "<x>caf\u00e9</x>"},
		{"non-ASCII attribute value as UTF-8", "<tt><x tts:fontFamily=\"Caf\u00e9\"/></tt>", "<x tts:fontFamily=\"Caf\u00e9\"/>"},
		{"comment and processing instruction", `<tt><x><!-- c --><?pi data?><?bare?></x></tt>`, `<x><!-- c --><?pi data?><?bare?></x>`},
		{"CDATA as escaped text", `<tt><x><![CDATA[a<b]]></x></tt>`, `<x>a&lt;b</x>`},
		{
			"golden: metadata keeps its declaration",
			`<tt xmlns="http://www.w3.org/ns/ttml"><metadata xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><ttm:title>T</ttm:title></metadata></tt>`,
			`<metadata xmlns:ttm="http://www.w3.org/ns/ttml#metadata"><ttm:title>T</ttm:title></metadata>`,
		},
		{
			"golden: quotes in a font family",
			`<tt><style tts:fontFamily="&quot;Verdana&quot;, sansSerif"/></tt>`,
			`<style tts:fontFamily="&quot;Verdana&quot;, sansSerif"/>`,
		},
		{
			"golden: white space between children",
			"<tt><head>\n    <styling>\n      <style/>\n    </styling>\n  </head></tt>",
			"<head>\n    <styling>\n      <style/>\n    </styling>\n  </head>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, firstChild(t, tt.in))
		})
	}
}

func TestSerializeDocument(t *testing.T) {
	t.Parallel()

	doc := parse(t, "<?xml version=\"1.0\"?>\n<!DOCTYPE tt>\n<!-- c -->\n<tt a=\"1\">x</tt>\n", false)
	assert.Equal(t, `<!DOCTYPE tt><!-- c --><tt a="1">x</tt>`, serialize(t, doc))
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       string
		wantLine int
		wantMsg  string
	}{
		{"mismatched end tag", "<tt>\n<p>x</div>\n</tt>", 2, "element <p> closed by </div>"},
		{"2 roots", "<tt/>\n<tt/>", 2, "the document has a second root element <tt>"},
		{"text after the root", "<tt/>\nx", 2, "the document has text outside the root element"},
		{"text before the root", "x<tt/>", 1, "the document has text outside the root element"},
		{"end tag outside the root", "<tt/></tt>", 1, "the document has the end tag </tt> outside the root element"},
		{"unclosed element", "<tt>\n<p>", 2, "unexpected EOF: element <p> is not closed"},
		{"no root", "<?xml version=\"1.0\"?>\n<!-- empty -->", 2, "the document has no root element"},
		{"empty", "", 1, "the document has no root element"},
		{"bare ampersand", "<tt>\nA & B</tt>", 2, "invalid character entity & (no semicolon)"},
		{"unknown entity", "<tt>&eacute;</tt>", 1, "invalid character entity &eacute;"},
		{"element name with a digit first", "<tt><0a/></tt>", 1, "invalid XML name: 0a"},
		{"attribute name with a digit first", "<tt\n-a=\"1\"/>", 2, "invalid XML name: -a"},
		{"unsupported version", `<?xml version="1.1"?><tt/>`, 1, `unsupported version "1.1"; only version 1.0 is supported`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := xmlraw.Parse(strings.NewReader(tt.in), xmlraw.ParseOptions{})
			var serr *xmlraw.SyntaxError
			require.ErrorAs(t, err, &serr)
			assert.Equal(t, tt.wantLine, serr.Line)
			assert.Equal(t, tt.wantMsg, serr.Msg)
		})
	}
}

func TestParseRecover(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"mismatched end tag closes the current element", "<tt><p>a<span>b</p></div></tt>", "<tt><p>a<span>b</span></p></tt>"},
		{"end of input closes every element", "<tt><p>a", "<tt><p>a</p></tt>"},
		{"stops at a second root", "<tt>a</tt><tt>b</tt>", "<tt>a</tt>"},
		{"stops at text after the root", "<tt>a</tt>b<x/>", "<tt>a</tt>"},
		{"stops at the first other error", "<tt><p>a & b</p></tt>", "<tt><p/></tt>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, serialize(t, parse(t, tt.in, true)))
		})
	}
}

func TestParseRecoverWithoutRootFails(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"Subtitles & captions", "x<tt/>", "<!-- empty -->"} {
		_, err := xmlraw.Parse(strings.NewReader(in), xmlraw.ParseOptions{Recover: true})
		var serr *xmlraw.SyntaxError
		require.ErrorAs(t, err, &serr, in)
		assert.Equal(t, "the document has no root element", serr.Msg, in)
	}
}

func TestParseLines(t *testing.T) {
	t.Parallel()

	doc := parse(t, "<tt\n  xmlns=\"urn:a\"\n  ><p\n    a=\"1\">x</p>\r\n<br/>\r<q/></tt>", false)
	var lines []int
	for n := range doc.Descendants() {
		lines = append(lines, n.Line)
	}
	assert.Equal(t, []int{3, 4, 5, 6}, lines)
}

func TestNamespaces(t *testing.T) {
	t.Parallel()

	doc := parse(t, `<tt xmlns="urn:tt" xmlns:tts="urn:s"><p xmlns:x="urn:x" tts:color="red" foo:a="1" b="2" xml:id="i"><foo:q/></p></tt>`, false)
	r := root(t, doc)
	p := r.Children[0]
	assert.Equal(t, "urn:tt", r.NamespaceURI())
	assert.Equal(t, "urn:tt", p.NamespaceURI())
	assert.Empty(t, p.Children[0].NamespaceURI(), "an undeclared prefix has no namespace")
	assert.Equal(t, xml.Name{Space: "foo", Local: "q"}, p.Children[0].Name)

	uris := make([]string, 0, len(p.Attrs))
	for _, a := range p.Attrs {
		uris = append(uris, p.AttrNamespaceURI(a))
	}
	assert.Equal(t, []string{"", "urn:s", "", "", xmlraw.XMLNamespace}, uris)

	for _, tt := range []struct {
		prefix, uri string
		ok          bool
	}{{"", "urn:tt", true}, {"tts", "urn:s", true}, {"x", "urn:x", true}, {"xml", xmlraw.XMLNamespace, true}, {"foo", "", false}} {
		uri, ok := p.LookupPrefix(tt.prefix)
		assert.Equal(t, tt.uri, uri, tt.prefix)
		assert.Equal(t, tt.ok, ok, tt.prefix)
	}
	bad := root(t, parse(t, `<tt xmlns:0="urn:x" a="1"><p/></tt>`, false))
	assert.False(t, xmlraw.IsNamespaceDecl(bad.Attrs[0]), "libxml reads xmlns:0 as a plain attribute")
	_, ok := bad.Children[0].LookupPrefix("0")
	assert.False(t, ok, "xmlns:0 binds no prefix")

	_, ok = r.LookupPrefix("x")
	assert.False(t, ok, "a declaration on a child does not reach its parent")
	assert.True(t, xmlraw.IsNamespaceDecl(p.Attrs[0]))
	assert.False(t, xmlraw.IsNamespaceDecl(p.Attrs[1]))
}

func TestText(t *testing.T) {
	t.Parallel()

	r := root(t, parse(t, `<tt>a<b>b<!-- c --><?pi x?>c</b><![CDATA[d]]></tt>`, false))
	assert.Equal(t, "abcd", r.Text())
}

func TestDescendantsStopsEarly(t *testing.T) {
	t.Parallel()

	r := root(t, parse(t, `<tt><a><b/></a><c/></tt>`, false))
	var names []string
	for n := range r.Descendants() {
		names = append(names, n.Name.Local)
		if n.Name.Local == "b" {
			break
		}
	}
	assert.Equal(t, []string{"a", "b"}, names)
	assert.Len(t, slices.Collect(r.Descendants()), 3)
}

func TestRemoveChild(t *testing.T) {
	t.Parallel()

	r := root(t, parse(t, `<tt><a/><b/><c/></tt>`, false))
	b := r.Children[1]
	r.RemoveChild(b)
	assert.Equal(t, `<tt><a/><c/></tt>`, serialize(t, r))
	assert.Nil(t, b.Parent)
	r.RemoveChild(b)
	assert.Len(t, r.Children, 2)
}

func TestExternalEntitiesAreNotLoaded(t *testing.T) {
	t.Parallel()

	in := "<?xml version=\"1.0\"?>\n<!DOCTYPE tt [<!ENTITY xxe SYSTEM \"file:///etc/passwd\">]>\n<tt>a&xxe;b</tt>"
	_, err := xmlraw.Parse(strings.NewReader(in), xmlraw.ParseOptions{})
	var serr *xmlraw.SyntaxError
	require.ErrorAs(t, err, &serr)
	assert.Equal(t, "invalid character entity &xxe;", serr.Msg)

	doc := parse(t, "<!DOCTYPE tt SYSTEM \"http://127.0.0.1:1/missing.dtd\">\n<tt>x</tt>", false)
	assert.Equal(t, xmlraw.DirectiveNode, doc.Children[0].Kind)
	assert.Equal(t, "x", root(t, doc).Text())
}

func TestDeclaredEncodingIsIgnored(t *testing.T) {
	t.Parallel()

	r := root(t, parse(t, "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><tt>caf\u00e9</tt>", false))
	assert.Equal(t, "caf\u00e9", r.Text())
}

func TestSyntaxErrorMessage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "xml: line 3: boom", (&xmlraw.SyntaxError{Line: 3, Msg: "boom"}).Error())
}
