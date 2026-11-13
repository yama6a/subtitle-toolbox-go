package stboxjson

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
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

// validateWords returns the message for the first word of c that breaks a rule of ValidateWords.
func validateWords(c *model.Cue, path string) string {
	err := c.ValidateWords()
	var iae *model.InvalidArgumentError
	if !errors.As(err, &iae) {
		return ""
	}
	field := strings.TrimPrefix(iae.Option, "Words")
	i, name, _ := strings.Cut(field, "].")
	field = path + ".words" + i + "]"
	if strings.HasSuffix(iae.Msg, "after the word before it.") {
		return field + " is not in the text of the cue after the word before it."
	}
	return field + "." + strings.ToLower(name[:1]) + name[1:] + " is invalid. " + iae.Msg
}
