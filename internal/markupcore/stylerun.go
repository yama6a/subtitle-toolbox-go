package markupcore

import (
	"slices"
	"strings"
)

// Run is a text with one style, such as "bold, red", for StyleRuns.
type Run struct {
	Text string
	// Color is the value of the color attribute of the <font> tag. Empty writes no <font> tag.
	Color                           string
	Bold, Italic, Underline, Strike bool
}

func (r Run) tags() []string {
	var tags []string
	if r.Color != "" {
		tags = append(tags, `<font color="`+r.Color+`">`)
	}
	for _, t := range [...]struct {
		on  bool
		tag string
	}{{r.Bold, "<b>"}, {r.Italic, "<i>"}, {r.Underline, "<u>"}, {r.Strike, "<s>"}} {
		if t.on {
			tags = append(tags, t.tag)
		}
	}
	return tags
}

// StyleRuns writes the runs as core markup and escapes their text. The tags nest in the order font, b, i, u, s.
// Between 2 runs, the tags close from the first style that changes and open again. A run of only white space keeps
// the tags. With spacesOutside, the white space at both ends of a run goes outside its tags.
func StyleRuns(runs []Run, spacesOutside bool) string {
	var b strings.Builder
	var open []string
	pending := ""
	for _, r := range runs {
		lead := len(r.Text) - len(strings.TrimLeft(r.Text, asciiSpace))
		body := strings.TrimRight(r.Text[lead:], asciiSpace)
		if body == "" {
			if spacesOutside {
				pending += r.Text
			} else {
				b.WriteString(r.Text)
			}
			continue
		}

		before, after := "", ""
		if spacesOutside {
			before, after = r.Text[:lead], r.Text[lead+len(body):]
		} else {
			body = r.Text
		}
		wanted := r.tags()
		keep := 0
		for keep < len(open) && keep < len(wanted) && open[keep] == wanted[keep] {
			keep++
		}
		b.WriteString(closeTags(open[keep:]) + pending + before + strings.Join(wanted[keep:], "") + EscapeText(body))
		open, pending = wanted, after
	}
	return b.String() + closeTags(open) + pending
}

func closeTags(tags []string) string {
	var b strings.Builder
	for _, tag := range slices.Backward(tags) {
		name, _, _ := strings.Cut(strings.TrimSuffix(tag[1:], ">"), " ")
		b.WriteString("</" + name + ">")
	}
	return b.String()
}
