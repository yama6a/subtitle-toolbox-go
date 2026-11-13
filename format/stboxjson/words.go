package stboxjson

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/vtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func cueSpeaker(c *model.Cue, d *model.Data, path string) string {
	switch speaker, _ := d.Get("speaker"); speaker := speaker.(type) {
	case nil:
	case string:
		c.Speaker = speaker
	default:
		return path + ".speaker must be a string."
	}
	return ""
}

func cueWords(c *model.Cue, d *model.Data, path string) string {
	v, _ := d.Get("words")
	if v == nil {
		return ""
	}
	path += ".words"
	list, ok := v.([]any)
	if !ok {
		return path + " must be a list."
	}
	c.Words = make([]model.Word, len(list))
	for i, item := range list {
		itemPath := path + "[" + strconv.Itoa(i) + "]"
		obj, ok := item.(*model.Data)
		if !ok {
			return itemPath + " must be an object."
		}
		if msg := word(&c.Words[i], obj, itemPath); msg != "" {
			return msg
		}
	}
	return ""
}

func word(w *model.Word, d *model.Data, path string) string {
	text, _ := d.String("text")
	if text == "" {
		return path + ".text must be a string that is not empty."
	}
	w.Text = text
	if msg := wordTimes(w, d, path); msg != "" {
		return msg
	}
	return wordConfidence(w, d, path)
}

func wordTimes(w *model.Word, d *model.Data, path string) string {
	for _, t := range []struct {
		key string
		to  *time.Duration
	}{{"start", &w.Start}, {"end", &w.End}} {
		n, ok := finite(d.Get(t.key))
		if !ok || n.f < 0 {
			return path + "." + t.key + " must be a time of 0 or more."
		}
		d, ok := milliseconds(n)
		if !ok {
			return path + "." + t.key + " is not below " + strconv.Itoa(timecode.MaxHours) + " hours."
		}
		*t.to = d
	}
	return ""
}

func wordConfidence(w *model.Word, d *model.Data, path string) string {
	v, _ := d.Get("confidence")
	if v == nil {
		return ""
	}
	n, ok := finite(v, true)
	if !ok || !(n.f >= 0 && n.f <= 1) {
		return path + ".confidence must be a number from 0 to 1."
	}
	w.Confidence = &n.f
	return ""
}

// validateWords returns the message for the first word of c that breaks a rule of ValidateWords. A cue whose text is
// just its word texts shows which rules the words break on their own. When they break none, the order rule fails.
func validateWords(c *model.Cue, path string) string {
	texts := make([]string, len(c.Words))
	for i, w := range c.Words {
		texts[i] = w.Text
	}
	own := model.Cue{Lines: []string{strings.Join(texts, " ")}, Words: c.Words}
	if err := own.ValidateWords(); err != nil {
		return wordMessage(err, path)
	}
	err := c.ValidateWords()
	var iae *model.InvalidArgumentError
	if !errors.As(err, &iae) {
		return ""
	}
	i, _, _ := strings.Cut(strings.TrimPrefix(iae.Option, "Words"), "].")
	return path + ".words" + i + "] is not in the text of the cue after the word before it."
}

func wordMessage(err error, path string) string {
	var iae *model.InvalidArgumentError
	if !errors.As(err, &iae) {
		return ""
	}
	i, name, _ := strings.Cut(strings.TrimPrefix(iae.Option, "Words"), "].")
	return path + ".words" + i + "]." + strings.ToLower(name[:1]) + name[1:] + " is invalid. " + iae.Msg
}

// restoreTaggedLines gives c the tag layout that formatData.vtt.taggedLines holds. It works on a copy of c that reads
// the lines as the WebVTT reader does. The layout stays when the copy has the Lines, Speaker, voice form and word
// texts and starts of c. Else the key goes and the read adds a warning. Word ends and confidences are not in tags,
// so they stay as the document gives them.
func (r *reader) restoreTaggedLines(c *model.Cue, i int, entry any) {
	stored, ok := c.FormatData(vttKey)
	if _, has := stored.Get(vtt.TaggedLinesKey); !ok || !has {
		return
	}
	probe := c.Clone()
	probe.Lines, probe.Speaker, probe.Words = taggedLines(stored), "", nil
	rest := stored.Clone()
	rest.Delete(vtt.TaggedLinesKey)
	_ = probe.SetFormatData(vttKey, rest)
	vtt.ExtractWords(&probe)
	if sameLayout(*c, probe) {
		probe.Words = c.Words
		_ = probe.SetFormatData(vttKey, stored)
		*c = probe
		return
	}
	stored.Delete(vtt.TaggedLinesKey)
	_ = c.SetFormatData(vttKey, stored)
	r.st.Warn("The field "+cuePath(i)+".formatData.vtt.taggedLines does not match the lines, the speaker, the voice form and "+
		"the words of the cue. The parser dropped it.", model.NoLine, i, []string{encodeEntry(entry)}, model.Repaired)
}

func sameLayout(c, probe model.Cue) bool {
	if !slices.Equal(c.Lines, probe.Lines) || c.Speaker != probe.Speaker || len(c.Words) != len(probe.Words) {
		return false
	}
	for i, w := range c.Words {
		if w.Text != probe.Words[i].Text || w.Start != probe.Words[i].Start {
			return false
		}
	}
	return voiceForm(c) == voiceForm(probe)
}

type voiceFormat struct {
	class string
	end   bool
}

func voiceForm(c model.Cue) voiceFormat {
	d, _ := c.FormatData(vttKey)
	class, _ := d.String("voiceClass")
	end, _ := d.Bool("voiceEnd")
	return voiceFormat{class, end}
}
