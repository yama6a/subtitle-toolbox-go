package webvtt

import (
	"io"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/position"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
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

type writer struct {
	stripTags bool
	dropped   []DroppedColor
}

// Write writes s as WebVTT.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	_, err := WriteWithReport(w, s, common, opts)
	return err
}

// WriteWithReport writes s as WebVTT and returns what the output leaves out.
func WriteWithReport(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) (WriteReport, error) {
	wr := &writer{stripTags: common.StripTags}
	body := wr.document(s)
	if err := writing.Write(w, model.WebVTT, []byte(body), common, true); err != nil {
		return WriteReport{}, err
	}
	return WriteReport{DroppedColors: wr.dropped}, nil
}

func (w *writer) document(s *model.Subtitle) string {
	file, _ := s.FormatData(string(model.WebVTT))
	blocks := fileBlocks(file)
	comments := make([][]string, s.Len()+1)
	for _, c := range s.Comments() {
		at := min(c.BeforeCue, s.Len())
		comments[at] = append(comments[at], noteBlock(c.Text))
	}
	for i, c := range s.All() {
		blocks = append(blocks, comments[i]...)
		blocks = append(blocks, w.cueBlock(c, i))
	}
	blocks = append(blocks, comments[s.Len()]...)

	out := header(file) + "\n\n"
	if len(blocks) > 0 {
		out += strings.Join(blocks, "\n\n") + "\n"
	}
	return out
}

func header(file *model.Data) string {
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

func fileBlocks(file *model.Data) []string {
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

// noteBlock writes a comment as a NOTE block. The block must not hold "-->", and a line of white space would end it,
// so the text loses each "-->" until none is left and every empty line. See https://www.w3.org/TR/webvtt1/#webvtt-comment-block.
func noteBlock(text string) string {
	for strings.Contains(text, arrow) {
		text = strings.ReplaceAll(text, arrow, "->")
	}
	var lines []string
	for l := range strings.SplitSeq(normalizeLineEnds(text), "\n") {
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

func (w *writer) cueBlock(c model.Cue, i int) string {
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

func (w *writer) text(c model.Cue, i int) string {
	text := c.Text()
	if w.stripTags {
		text = markupcore.StripAllTags(text)
	} else {
		var dropped []string
		text, dropped = markupcore.FontToWebVTTColors(text)
		for _, color := range dropped {
			w.dropped = append(w.dropped, DroppedColor{
				CueIndex: i,
				Color:    color,
				Message: "Cue #" + strconv.Itoa(i) + " at " + timecode.Seconds(c.Start) + ` s: dropped the color "` + color +
					`", because WebVTT has classes for 8 colors only.`,
			})
		}
		text = keepSpanTags(text)
	}
	return strings.ReplaceAll(text, arrow, "--&gt;")
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
