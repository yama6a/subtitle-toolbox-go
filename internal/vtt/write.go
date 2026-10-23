package vtt

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/blocks"
	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

//nolint:gochecknoglobals // lookup tables of the writer
var (
	// spanTags are the tags of https://www.w3.org/TR/webvtt1/#webvtt-cue-span-start-tag and <strong>.
	spanTags = []string{"strong", "b", "u", "i", "v", "lang", "c", "ruby", "rt"}
	// keptSettings are the settings that stay when the cue alignment changed. The new alignment replaces the others.
	keptSettings = []string{"vertical", "size", "region"}
	rowSettings  = [...]string{"", "line:50%,center", "line:0"}
	// columnSettings holds the setting of each alignment column. Column 0 does not exist.
	columnSettings = [...]string{"", "align:left", "", "align:right"}
)

var (
	inlineTimestampRegexp = regexp.MustCompile(`<(?:\d{2,}:)?[0-5]\d:[0-5]\d\.\d{3}>`)
	spanClassRegexp       = regexp.MustCompile(`(?i)</?((?:strong|b|u|i|v|lang|c|ruby|rt)\.[^\t\n\v\f\r >]*)`)
)

// Writer writes the blocks of a WebVTT file.
type Writer struct {
	// StripTags writes the cue text without markup.
	StripTags bool
	// OnDroppedColor is called for each font color that no WebVTT color class has. It may be nil.
	OnDroppedColor func(cueIndex int, color, message string)
}

// Header returns the signature line and the header text lines of file, without a line end at the end.
func Header(file *model.Data) string {
	out := signature
	if text, _ := file.String("header"); text != "" {
		out += " " + text
	}
	lines, _ := file.Strings("headerLines")
	for _, l := range lines {
		out += "\n" + l
	}
	return out
}

// FileBlocks returns the REGION and STYLE blocks of file.
func FileBlocks(file *model.Data) []string {
	regions, _ := file.Get("regions")
	styles, _ := file.Strings("styles")
	out := make([]string, 0, len(values(regions))+len(styles))
	for _, region := range values(regions) {
		lines := []string{"REGION"}
		switch region := region.(type) {
		case *model.Data:
			for _, k := range region.Keys() {
				v, _ := region.String(k)
				lines = append(lines, k+":"+v)
			}
		case []any:
			for i, v := range region {
				s, _ := v.(string)
				lines = append(lines, strconv.Itoa(i)+":"+s)
			}
		}
		out = append(out, strings.Join(lines, "\n"))
	}
	for _, style := range styles {
		out = append(out, "STYLE\n"+style)
	}
	return out
}

func values(v any) []any {
	switch v := v.(type) {
	case []any:
		return v
	case *model.Data:
		out := make([]any, 0, v.Len())
		for _, k := range v.Keys() {
			item, _ := v.Get(k)
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

// NoteBlock writes a comment as a NOTE block. The block must not hold "-->", and a line of white space ends it.
// So the text loses each "-->" until none is left, and it loses every empty line.
// See https://www.w3.org/TR/webvtt1/#webvtt-comment-block.
func NoteBlock(text string) string {
	for strings.Contains(text, arrow) {
		text = strings.ReplaceAll(text, arrow, "->")
	}
	var lines []string
	for _, l := range parsing.Lines(text) {
		if l = phpTrim(spacesRegexp.ReplaceAllString(l, " ")); l != "" {
			lines = append(lines, l)
		}
	}
	switch len(lines) {
	case 0:
		return "NOTE"
	case 1:
		return "NOTE " + lines[0]
	default:
		return "NOTE\n" + strings.Join(lines, "\n")
	}
}

// CueBlock writes cue c, the cue number i counting from 0, as a block without a line end at the end.
func (w *Writer) CueBlock(c model.Cue, i int) string {
	id := c.ID
	if phpTrim(id) == "" || strings.Contains(id, arrow) || strings.ContainsAny(id, "\r\n") {
		id = strconv.Itoa(i + 1)
	}
	out := id + "\n" + markupcore.CoreTimestamp(c.Start) + " " + arrow + " " + markupcore.CoreTimestamp(c.End)
	if settings := cueSettingsText(c); settings != "" {
		out += " " + settings
	}
	if text := w.text(c, i); text != "" {
		out += "\n" + text
	}
	return out
}

// cueSettingsText writes the stored settings while the alignment is unchanged, and the settings of the alignment otherwise.
func cueSettingsText(c model.Cue) string {
	d, _ := c.FormatData(string(model.WebVTT))
	unchanged := position.AlignmentUnchanged(d, c.Alignment)
	names := cueSettings
	if !unchanged {
		names = keptSettings
	}
	var settings []string
	for _, k := range d.Keys() {
		if v, ok := d.String(k); ok && slices.Contains(names, k) {
			settings = append(settings, k+":"+v)
		}
	}
	if unchanged && len(settings) > 0 || c.Alignment.Numpad() == model.BottomCenter.Numpad() {
		return strings.Join(settings, " ")
	}
	for _, s := range []string{rowSettings[c.Alignment.Row()], columnSettings[c.Alignment.Column()]} {
		if s != "" {
			settings = append(settings, s)
		}
	}
	return strings.Join(settings, " ")
}

func (w *Writer) text(c model.Cue, i int) string {
	text := c.Text()
	if w.StripTags {
		text = markupcore.StripAllTags(text)
	} else {
		var dropped []string
		text, dropped = markupcore.FontToWebVTTColors(text)
		for _, color := range dropped {
			if w.OnDroppedColor != nil {
				w.OnDroppedColor(i, color, "Cue #"+strconv.Itoa(i)+" at "+timecode.Seconds(c.Start)+` s: dropped the color "`+color+
					`", because WebVTT has classes for 8 colors only.`)
			}
		}
		text = keepSpanTags(text)
	}
	// A line that holds only a dropped tag or a BOM reads as an empty line, and an empty line ends the cue.
	var kept []string
	for line := range strings.SplitSeq(text, "\n") {
		if blocks.CleanLine(line) != "" {
			kept = append(kept, line)
		}
	}
	return strings.ReplaceAll(strings.Join(kept, "\n"), arrow, "--&gt;")
}

// keepSpanTags keeps the inline timestamps and the span tags of WebVTT with their classes, such as <c.yellow>.
// It drops every other tag.
func keepSpanTags(text string) string {
	if !strings.Contains(text, "<") {
		return text
	}
	var b strings.Builder
	at := 0
	for _, loc := range inlineTimestampRegexp.FindAllStringIndex(text, -1) {
		b.WriteString(keepSpanTagsIn(text[at:loc[0]]))
		b.WriteString(text[loc[0]:loc[1]])
		at = loc[1]
	}
	b.WriteString(keepSpanTagsIn(text[at:]))
	return b.String()
}

// keepSpanTagsIn allows the names with classes next to the bare names, because KeepTags compares the whole name.
func keepSpanTagsIn(part string) string {
	names := slices.Clone(spanTags)
	for _, m := range spanClassRegexp.FindAllStringSubmatch(part, -1) {
		names = append(names, strings.TrimRight(m[1], "/"))
	}
	return markupcore.KeepTags(part, names...)
}
