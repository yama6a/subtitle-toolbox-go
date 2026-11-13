package model

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
)

// Word is a timed part of the text of a cue, usually one word.
type Word struct {
	// Text is the word as it occurs in the plain text of the cue. It is not empty and has no white space at either end.
	Text string
	// Start is the time the word starts.
	Start time.Duration
	// End is the time the word ends. It is at or after Start.
	End time.Duration
	// Confidence is the confidence of the source, from 0 to 1. Nil means the source gives none.
	Confidence *float64
}

// taggedSource holds what ExtractWordTags read and set. Nothing changes it after ExtractWordTags built it.
type taggedSource struct {
	lines   []string // the cleaned lines with the tags, as ExtractWordTags read them
	plain   []string // Lines after ExtractWordTags
	speaker string
	voice   markupcore.Voice // the voice of the cue right after ExtractWordTags
	words   []markupcore.TimedText
}

// PlainText returns the lines joined with one space, without tags, with entities decoded, each run of white space
// as one space, and no white space at either end. The Text of each word occurs in it.
func (c Cue) PlainText() string {
	return markupcore.CuePlainText(c.Lines)
}

// ValidateWords returns an *InvalidArgumentError for the first word that breaks a rule. Each Text is not empty, has
// no white space at either end, and occurs in PlainText after the end of the word before it. Start is at most End.
// The starts do not decrease. Both times are 0 or more and below 100000 hours. Confidence is nil or from 0 to 1.
func (c Cue) ValidateWords() error {
	for i, w := range c.Words {
		var before *Word
		if i > 0 {
			before = &c.Words[i-1]
		}
		if err := checkWord(i, w, before); err != nil {
			return err
		}
	}
	found := markupcore.FindWords(c.PlainText(), wordTexts(c.Words))
	if len(found) < len(c.Words) {
		return wordError(len(found), "Text", "The word "+strconv.Quote(c.Words[len(found)].Text)+" is not in the text of the cue after the word before it.")
	}
	return nil
}

func checkWord(i int, w Word, before *Word) error {
	switch {
	case w.Text == "":
		return wordError(i, "Text", "The word text must not be empty.")
	case strings.TrimFunc(w.Text, markupcore.IsSpace) != w.Text:
		return wordError(i, "Text", "The word text must not start or end with white space, got "+strconv.Quote(w.Text)+".")
	}
	return checkWordTimes(i, w, before)
}

func checkWordTimes(i int, w Word, before *Word) error {
	switch {
	case !validWordTime(w.Start):
		return wordError(i, "Start", "The word start must be 0 s or more and below 100000 hours, got "+timecode.Seconds(w.Start)+" s.")
	case !validWordTime(w.End):
		return wordError(i, "End", "The word end must be 0 s or more and below 100000 hours, got "+timecode.Seconds(w.End)+" s.")
	case w.End < w.Start:
		return wordError(i, "End", "The word end "+timecode.Seconds(w.End)+" s must not be before the word start "+timecode.Seconds(w.Start)+" s.")
	case before != nil && w.Start < before.Start:
		return wordError(i, "Start", "The word start "+timecode.Seconds(w.Start)+" s must not be before the start "+timecode.Seconds(before.Start)+" s of the word before it.")
	case w.Confidence != nil && !(*w.Confidence >= 0 && *w.Confidence <= 1):
		return wordError(i, "Confidence", "The confidence must be from 0 to 1, got "+timecode.Number(*w.Confidence)+".")
	}
	return nil
}

func validWordTime(d time.Duration) bool {
	return d >= 0 && d < timecode.MaxTime
}

func wordError(i int, field, msg string) error {
	return &InvalidArgumentError{Option: "Words[" + strconv.Itoa(i) + "]." + field, Msg: msg}
}

func wordTexts(words []Word) []string {
	texts := make([]string, len(words))
	for i, w := range words {
		texts[i] = w.Text
	}
	return texts
}

func timedTexts(words []Word) []markupcore.TimedText {
	out := make([]markupcore.TimedText, len(words))
	for i, w := range words {
		out[i] = markupcore.TimedText{Text: w.Text, Start: w.Start}
	}
	return out
}

// SyncWords searches each word again in PlainText after a text edit, in order, each search from the end of the
// match before. Runes compare with simple case folding. A found word takes the text it matched. A word that
// SyncWords does not find leaves Words and loses its timing. SyncWords returns the number of words it dropped.
// It never changes a time or the order of the words. A word that it does not find costs one pass over the rest
// of the text, and words with the same text share that pass.
func (c *Cue) SyncWords() (dropped int) {
	if len(c.Words) == 0 {
		return 0
	}
	plain := c.PlainText()
	matches := markupcore.SearchWords(plain, wordTexts(c.Words), true)
	kept := make([]Word, 0, len(c.Words))
	for i, m := range matches {
		if m.Start < 0 {
			dropped++
			continue
		}
		w := c.Words[i]
		w.Text = plain[m.Start:m.End]
		kept = append(kept, w)
	}
	c.Words = kept
	if len(kept) == 0 {
		c.Words = nil
	}
	return dropped
}

// ExtractWordTags moves the word timestamp tags, such as <00:00:01.000>, and a whole-cue voice tag, such as
// <v Anna>, out of the lines into Words and Speaker. It reads lines in the form of library JSON version 1 and
// of 3.x. Lines without a word timestamp tag keep Words as they are. docs/markup.md has the rules.
func (c *Cue) ExtractWordTags() {
	vtt, hasVTT := c.formatData["vtt"]
	got := markupcore.ExtractWordTags(c.Lines, c.End, hasVTT)
	source := markupcore.CleanLines(c.Lines)
	if got.Found {
		c.Words = nil
		for i, w := range got.Words {
			c.Words = append(c.Words, Word{Text: w.Text, Start: w.Start, End: got.Ends[i].Round(time.Millisecond)})
		}
	}
	if got.Voice.Name != "" {
		c.Speaker = got.Voice.Name
		if hasVTT {
			c.setVoiceForm(vtt, got.Voice)
		}
	}
	c.Lines = got.Lines
	if got.Found || got.Voice.Name != "" {
		c.tagged = &taggedSource{
			lines:   source,
			plain:   slices.Clone(got.Lines),
			speaker: c.Speaker,
			voice:   c.voice(),
			words:   timedTexts(c.Words),
		}
	}
}

func (c *Cue) setVoiceForm(vtt *Data, v markupcore.Voice) {
	d := vtt.Clone()
	d.Delete("voiceClass")
	d.Delete("voiceEnd")
	if v.Class != "" {
		d.Set("voiceClass", v.Class)
	}
	if v.Close {
		d.Set("voiceEnd", true)
	}
	// The keys are in the schema, so SetFormatData cannot fail.
	_ = c.SetFormatData("vtt", d)
}

// voice returns the whole-cue voice of c. Its class and end come from the vtt cue format data.
func (c Cue) voice() markupcore.Voice {
	if c.Speaker == "" {
		return markupcore.Voice{}
	}
	v := markupcore.Voice{Name: c.Speaker}
	if d, ok := c.formatData["vtt"]; ok {
		v.Class, _ = d.String("voiceClass")
		v.Close, _ = d.Bool("voiceEnd")
	}
	return v
}

// TaggedLines returns the lines with a word timestamp tag before each word and the voice tag of Speaker, in the
// form of library JSON version 1. For a cue whose lines, speaker and word texts and starts are the ones that
// ExtractWordTags set, it returns the lines that ExtractWordTags read, byte for byte.
func (c Cue) TaggedLines() []string {
	if t := c.tagged; t != nil && slices.Equal(c.Lines, t.plain) && c.Speaker == t.speaker &&
		c.voice() == t.voice && slices.Equal(timedTexts(c.Words), t.words) {
		return slices.Clone(t.lines)
	}
	return markupcore.TagWords(c.Lines, timedTexts(c.Words), c.voice())
}

func cloneWords(words []Word) []Word {
	if words == nil {
		return nil
	}
	out := slices.Clone(words)
	for i, w := range out {
		if w.Confidence != nil {
			v := *w.Confidence
			out[i].Confidence = &v
		}
	}
	return out
}

func roundWords(words []Word) []Word {
	for i := range words {
		words[i].Start = words[i].Start.Round(time.Millisecond)
		words[i].End = words[i].End.Round(time.Millisecond)
	}
	return words
}
