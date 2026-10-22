package sami

import (
	"regexp"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
)

// paragraph is a <P> of a SYNC block, or the loose content of the block outside every <P>.
type paragraph struct {
	class    string
	hasClass bool
	// attrs holds the attributes except class.
	attrs []attr
	html  string
	lines []string
}

var (
	textSpaceRegexp = regexp.MustCompile(`[ \t\n\r\f]+`)
	hexColorRegexp  = regexp.MustCompile(`^#?([0-9a-f]{6})$`)
)

// readParagraphs reads the <P> elements of the content of one SYNC block. The content outside every <P> becomes a
// paragraph without class, first, when the block has no <P> or when that content holds text.
func readParagraphs(content string) []paragraph {
	var paragraphs []paragraph
	var loose []*node
	var looseText strings.Builder
	for _, n := range scan(fixBareNbsp(content)) {
		if n.kind == elementNode && n.name == "p" {
			paragraphs = append(paragraphs, newParagraph(n))
			continue
		}
		loose = append(loose, n)
		looseText.WriteString(textContent(n))
	}
	if paragraphs == nil || strings.Trim(strings.ReplaceAll(looseText.String(), nbsp, " "), markupcore.PHPTrimSet) != "" {
		wrapper := &node{kind: elementNode, name: "p", children: loose}
		paragraphs = append([]paragraph{newParagraph(wrapper)}, paragraphs...)
	}
	return paragraphs
}

// fixBareNbsp adds the ; to each &nbsp that has none. Browsers and other SAMI readers read it as a no-break space.
func fixBareNbsp(s string) string {
	const bare = "&nbsp"
	if !strings.Contains(s, bare) {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(s, bare)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:i+len(bare)])
		s = s[i+len(bare):]
		if !strings.HasPrefix(s, ";") {
			b.WriteByte(';')
		}
	}
}

func newParagraph(p *node) paragraph {
	var para paragraph
	for _, a := range p.attrs {
		if a.name == "class" {
			para.class, para.hasClass = a.value, true
			continue
		}
		para.attrs = append(para.attrs, a)
	}
	var html strings.Builder
	for _, c := range p.children {
		serialize(&html, c)
	}
	para.html = strings.Trim(html.String(), markupcore.PHPTrimSet)
	var text strings.Builder
	markup(&text, p.children)
	for line := range strings.SplitSeq(text.String(), "\n") {
		line = strings.Trim(line, markupcore.PHPTrimSet)
		if strings.Trim(line, " "+nbsp) != "" {
			para.lines = append(para.lines, line)
		}
	}
	return para
}

// textContent returns the text of a node as the DOM reads it: the data of a comment, and the text nodes of an element
// without its comments.
func textContent(n *node) string {
	if n.kind != elementNode {
		return n.text
	}
	var b strings.Builder
	var walk func(n *node)
	walk = func(n *node) {
		for _, c := range n.children {
			switch c.kind {
			case textNode:
				b.WriteString(c.text)
			case elementNode:
				walk(c)
			case commentNode:
			}
		}
	}
	walk(n)
	return b.String()
}

// markup writes the core markup of nodes, with LF for each <br>.
func markup(b *strings.Builder, nodes []*node) {
	for _, n := range nodes {
		switch n.kind {
		case textNode:
			b.WriteString(markupcore.EscapeText(textSpaceRegexp.ReplaceAllLiteralString(n.text, " ")))
		case elementNode:
			elementMarkup(b, n)
		case commentNode:
		}
	}
}

func elementMarkup(b *strings.Builder, n *node) {
	var open, end string
	switch n.name {
	case "br":
		b.WriteByte('\n')
	case "b", "i", "u", "s":
		open, end = "<"+n.name+">", "</"+n.name+">"
	case "strike":
		open, end = "<s>", "</s>"
	case "font":
		if color, ok := fontColor(n); ok {
			open, end = `<font color="`+color+`">`, "</font>"
		}
	}
	b.WriteString(open)
	markup(b, n.children)
	b.WriteString(end)
}

// fontColor returns the color of a <font> as "#rrggbb": #rrggbb, rrggbb or one of the 16 color names of HTML 4.01.
func fontColor(font *node) (string, bool) {
	var value string
	for _, a := range font.attrs {
		if a.name == "color" {
			value = a.value
		}
	}
	value = markupcore.ASCIILower(strings.Trim(value, markupcore.PHPTrimSet))
	if m := hexColorRegexp.FindStringSubmatch(value); m != nil {
		return "#" + m[1], true
	}
	return markupcore.NamedColor(value, true)
}
