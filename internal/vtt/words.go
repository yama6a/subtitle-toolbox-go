package vtt

import (
	"slices"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// TaggedLinesKey is the key of the vtt cue format data that holds the lines of a cue with their tags, as the file
// had them. ExtractWords stores it only when TaggedLines cannot rebuild those lines from Lines, Speaker, Words and
// the voice form.
const TaggedLinesKey = "taggedLines"

// ExtractWords moves the word timestamps and the whole-cue voice of the lines of c into Words and Speaker.
// It keeps the class and the </v> of the voice in the vtt cue format data, also when the cue has no settings.
// The reader has already written the timestamp <mm:ss.ttt> as <00:mm:ss.ttt>.
func ExtractWords(c *model.Cue) {
	_, had := c.FormatData(string(model.WebVTT))
	if !had {
		// An empty Data is not stored, so a marker key stands in until ExtractWordTags has set the voice form.
		_ = c.SetFormatData(string(model.WebVTT), model.NewData(model.Field{Key: "voiceEnd", Value: false}))
	}
	c.ExtractWordTags()
	keepTaggedLines(c)
	if had {
		return
	}
	if d, ok := c.FormatData(string(model.WebVTT)); ok {
		if end, _ := d.Bool("voiceEnd"); !end {
			d.Delete("voiceEnd")
			_ = c.SetFormatData(string(model.WebVTT), d)
		}
	}
}

func keepTaggedLines(c *model.Cue) {
	tagged := c.TaggedLines()
	plain := model.Cue{Lines: c.Lines, Speaker: c.Speaker, Words: c.Words}
	if d, ok := c.FormatData(string(model.WebVTT)); ok {
		_ = plain.SetFormatData(string(model.WebVTT), d)
	}
	if slices.Equal(tagged, plain.TaggedLines()) {
		return
	}
	d, _ := c.FormatData(string(model.WebVTT))
	if d == nil {
		d = model.NewData()
	}
	d.Set(TaggedLinesKey, stringsToAny(tagged))
	_ = c.SetFormatData(string(model.WebVTT), d)
}

func stringsToAny(lines []string) []any {
	out := make([]any, len(lines))
	for i, l := range lines {
		out[i] = l
	}
	return out
}
