package markup_test

import (
	"fmt"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/markup"
)

func ExampleStripAllTags() {
	fmt.Println(markup.StripAllTags("<b>Hi</b> &amp; bye"))
	fmt.Println(markup.StripAllTags("a < b > c"))
	// Output:
	// Hi &amp; bye
	// a < b > c
}

func ExamplePlainText() {
	fmt.Println(markup.PlainText("<b>Hi</b> &amp; bye"))
	// Output: Hi & bye
}

func ExampleKeepTags() {
	fmt.Println(markup.KeepTags("<b>Hi</b> <c.red>you</c>", "b"))
	// Output: <b>Hi</b> you
}

func ExampleDecodeEntities() {
	fmt.Println(markup.DecodeEntities("Hi &amp; bye &#13; &amp"))
	// Output: Hi & bye &#13; &amp
}

func ExampleEscapeText() {
	fmt.Println(markup.EscapeText("Fish & Chips"))
	// Output: Fish &amp; Chips
}

func ExampleVisibleLength() {
	fmt.Println(markup.VisibleLength("<i>Caf\u00e9</i> &amp; tea "))
	// Output: 10
}

func ExampleVoiceTag() {
	fmt.Println(markup.VoiceTag("O'Neil", ""))
	fmt.Println(markup.VoiceTag("Ann & Bo", ".loud"))
	// Output:
	// <v O'Neil>
	// <v.loud Ann &amp; Bo>
}

func ExampleMapTextRuns() {
	fmt.Printf("%q\n", markup.MapTextRuns([]string{"<i>Hi</i> you"}, func(text string, _, _ bool) string {
		return strings.ToUpper(text)
	}))
	// Output: ["<i>HI</i> YOU"]
}

func ExampleHasVisibleText() {
	fmt.Println(markup.HasVisibleText([]string{"<i></i>", " "}))
	fmt.Println(markup.HasVisibleText([]string{"< b>"}))
	// Output:
	// false
	// true
}

func Example() {
	fmt.Println(markup.StripAllTags("<b>Hi</b> &amp; bye"))
	fmt.Println(markup.PlainText("<b>Hi</b> &amp; bye"))
	fmt.Println(markup.KeepTags("<b>Hi</b> <c.red>you</c>", "b"))
	fmt.Println(markup.EscapeText("Fish & Chips"))
	fmt.Println(markup.VoiceTag("O'Neil", ""))
	fmt.Println(markup.HasVisibleText([]string{"<i></i>", " "}))
	// Output:
	// Hi &amp; bye
	// Hi & bye
	// <b>Hi</b> you
	// Fish &amp; Chips
	// <v O'Neil>
	// false
}
