package vtt

import "github.com/yama6a/subtitle-toolbox-go/v4/model"

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
