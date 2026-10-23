package sami

import (
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// dump writes a tree in a short form: name{attributes}(children), "text" and <!--comment-->.
func dump(nodes []*node) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		switch n.kind {
		case textNode:
			parts[i] = strconv.Quote(n.text)
		case commentNode:
			parts[i] = "<!--" + n.text + "-->"
		case elementNode:
			s := n.name
			if len(n.attrs) > 0 {
				attrs := make([]string, len(n.attrs))
				for j, a := range n.attrs {
					attrs[j] = a.name
					if a.hasValue {
						attrs[j] += "=" + a.value
					}
				}
				s += "{" + strings.Join(attrs, " ") + "}"
			}
			if len(n.children) > 0 {
				s += "(" + dump(n.children) + ")"
			}
			parts[i] = s
		}
	}
	return strings.Join(parts, " ")
}

func serializeAll(nodes []*node) string {
	var b strings.Builder
	for _, n := range nodes {
		serialize(&b, n)
	}
	return b.String()
}

// The rows of the scanner table of https://github.com/yama6a/subtitle-toolbox-go/issues/25.
func TestScan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		tree string
		html string
	}{
		{
			"1 names in lower case", `<P Class=ENCC>x<BR><FONT COLOR=red>y`,
			`p{class=ENCC}("x" br font{color=red}("y"))`, `<p class="ENCC">x<br><font color="red">y</font></p>`,
		},
		{
			"2 quoted, unquoted and empty values", `<font a="x y" b='q"r' c=d e f="1" a="2">`,
			`font{a=x y b=q"r c=d e f=1}`, `<font a="x y" b='q"r' c="d" e f="1"></font>`,
		},
		{"2 values decode references", `<font color="&amp;&lt;&#39;">`, `font{color=&<'}`, `<font color="&amp;&lt;'"></font>`},
		{"2 a value with both quotes", `<b t='a"b&#39;c'>`, `b{t=a"b'c}`, `<b t="a&quot;b'c"></b>`},
		{"2 a quote that does not end", `<b t="a>b`, `b{t=a>b}`, `<b t="a&gt;b"></b>`},
		{"2 an empty value", `<b t= >x`, `b{t=}("x")`, `<b t="">x</b>`},
		{"2 junk in a tag", `<b "x" =y z>`, `b{z}`, `<b z></b>`},
		{
			"3 references", `&amp; &#039; &#x27; &eacute; &apos; &lang;`,
			strconv.Quote("& ' ' \u00E9 ' \u2329"), "&amp; ' ' \u00E9 ' \u2329",
		},
		{"3 a reference to a code point that XML does not allow", `a&#0;b&#xD800;c&#1114112;d`, `"abcd"`, `abcd`},
		{"4 unknown names and missing semicolons", `&foo; &nbsp x &AMP; &#; &#x; &`, `"&foo; &nbsp x &AMP; &#; &#x; &"`, `&amp;foo; &amp;nbsp x &amp;AMP; &amp;#; &amp;#x; &amp;`},
		{"3 numeric references without semicolons", `a&#39s &#x27g &#233 &#0x`, "\"a's 'g \u00e9 x\"", "a's 'g \u00e9 x"},
		{"4 an HTML5 name", `&check;`, `"&check;"`, `&amp;check;`},
		{"5 a < without a letter", `a < b <3 </3 <`, `"a < b <3 </3 <"`, `a &lt; b &lt;3 &lt;/3 &lt;`},
		{"6 comments, DOCTYPE and processing instructions", `a<!-- note -->b<!DOCTYPE html><?php x ?>c`, `"a" <!-- note --> "bc"`, `a<!-- note -->bc`},
		{"6 a comment that does not end", `a<!--b-`, `"a" <!--b--->`, `a<!--b--->`},
		{"7 br forms", `<br>a<br/>b<br />c<BR>`, `br "a" br "b" br "c" br`, `<br>a<br>b<br>c<br>`},
		{"7 void elements get no children", `<img src=a>x<hr>`, `img{src=a} "x" hr`, `<img src="a">x<hr>`},
		{"8 /> ends any element", `<b/>x`, `b "x"`, `<b></b>x`},
		{"9 p closes p", `<P Class=A>one<P Class=B>two`, `p{class=A}("one") p{class=B}("two")`, `<p class="A">one</p><p class="B">two</p>`},
		{"9 p closes inline elements", `<b>x<i>y<p>z`, `b("x" i("y")) p("z")`, `<b>x<i>y</i></b><p>z</p>`},
		{
			"10 p does not close font", `<P>one <font color=red>red<P>two`,
			`p("one " font{color=red}("red" p("two")))`, `<p>one <font color="red">red<p>two</p></font></p>`,
		},
		{"10 p does not close span", `<span><b>x<p>y`, `span(b("x") p("y"))`, `<span><b>x</b><p>y</p></span>`},
		{"11 div closes p", `<p>a<div>b`, `p("a") div("b")`, `<p>a</p><div>b</div>`},
		{"11 div closes only an innermost p", `<p><b>a<div>b`, `p(b("a" div("b")))`, `<p><b>a<div>b</div></b></p>`},
		{"12 end tags close what opened after", `<b>open <i>tags</p></div></font>`, `b("open " i("tags"))`, `<b>open <i>tags</i></b>`},
		{"12 an end tag closes the innermost of its name", `<b>a<b>b</b>c`, `b("a" b("b") "c")`, `<b>a<b>b</b>c</b>`},
		{"13 an end tag does not cross a div", `<b><div>x</b>y`, `b(div("xy"))`, `<b><div>xy</div></b>`},
		{"13 an end tag of a div crosses inline elements", `<div><b>x</div>y`, `div(b("x")) "y"`, `<div><b>x</b></div>y`},
		{"13 an end tag does not cross a table", `<table><td><b>x</td>y`, `table(td(b("x")) "y")`, `<table><td><b>x</b></td>y</table>`},
		{"14 ignored end tags", `a</br>b</p>c`, `"abc"`, `abc`},
		{"15 html, head and body", `<html><head><body>x</body></head></html>`, `"x"`, `x`},
		{"16 open elements close at the end", `<b><i>x`, `b(i("x"))`, `<b><i>x</i></b>`},
		{"17 white space between tags", "<b>a</b> \n<i>b</i>", `b("a") " \n" i("b")`, "<b>a</b> \n<i>b</i>"},
		{"unknown elements", `<span class=x>a</span><foo>b`, `span{class=x}("a") foo("b")`, `<span class="x">a</span><foo>b</foo>`},
		{"empty", ``, ``, ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			nodes := scan(tt.in)
			assert.Equal(t, tt.tree, dump(nodes))
			assert.Equal(t, tt.html, serializeAll(nodes))
		})
	}
}

// The serialiser examples of https://github.com/yama6a/subtitle-toolbox-go/issues/25, from the read goldens.
func TestSerialize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in, want string
	}{
		{`we&#039;re now<br>arriving`, `we're now<br>arriving`},
		{`<FONT COLOR="#FFFF00"><B>x`, `<font color="#FFFF00"><b>x</b></font>`},
		{`a<br />b`, `a<br>b`},
		{`&amp;`, `&amp;`},
		{"a<br>\n         b", "a<br>\n         b"},
		{`a&nbsp;b`, "a" + nbsp + "b"},
		{`<p id=Source>x`, `<p id="Source">x</p>`},
		{`<font face="it's">x`, `<font face="it's">x</font>`},
		{`<font face='say "hi"'>x`, `<font face='say "hi"'>x</font>`},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, serializeAll(scan(tt.in)))
		})
	}
}

func FuzzScan(f *testing.F) {
	for _, seed := range []string{
		`<P Class=ENCC>x<BR><FONT COLOR=red>y`, `<font a="x y" b='q"r' c=d e f="1" a="2">`, `<b t='a"b&#39;c'>`,
		`&amp; &#039; &#x27; &eacute; &foo; &nbsp`, `a < b <3 </3`, `a<!-- note -->b<!DOCTYPE html><?php x ?>c`,
		`<br>a<br/>b<br />c`, `<b/>x`, `<P>one <font color=red>red<P>two`, `<p>a<div>b`, `<b>open <i>tags</p></div></font>`,
		`<b><div>x</b>y`, `<html><head><body>x</body></html>`, `<b t="a>b`, `a<!--b-`, `<table><td><b>x</td>y`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, in string) {
		nodes := scan(in)
		html := serializeAll(nodes)
		again := scan(html)
		if dump(nodes) != dump(again) {
			t.Fatalf("scan(%q) gives %s, but its serialisation %q gives %s", in, dump(nodes), html, dump(again))
		}
	})
}

// TestScanGrowth scans end tags that close nothing at n and 10n tags, best of 3 each, and fails above a ratio of 30.
func TestScanGrowth(t *testing.T) {
	t.Parallel()

	if os.Getenv("STBOX_GROWTH") != "1" {
		t.Skip("growth test, set STBOX_GROWTH=1")
	}
	inputs := map[string]func(n int) string{
		"unmatched end tags": func(n int) string { return strings.Repeat("<b>", n) + strings.Repeat("</i>", n) },
		"end tags that a div blocks": func(n int) string {
			return "<b>" + strings.Repeat("<div>", n) + strings.Repeat("</b>", n)
		},
		"end tags that a table blocks": func(n int) string {
			return "<td><table>" + strings.Repeat("<div>", n) + strings.Repeat("</td>", n)
		},
	}
	for name, input := range inputs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			best := func(s string) time.Duration {
				fastest := time.Duration(math.MaxInt64)
				for range 3 {
					start := time.Now()
					scan(s)
					fastest = min(fastest, time.Since(start))
				}
				return fastest
			}
			small, large := best(input(3000)), best(input(30000))
			assert.Less(t, float64(large)/float64(max(small, time.Microsecond)), 30.0, "3000: %v, 30000: %v", small, large)
		})
	}
}
