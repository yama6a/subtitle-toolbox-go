// Package speakers converts between speakers and the labels, dashes and colors that formats without speakers show.
// docs/text.md has the rules.
package speakers

import (
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Style is the form in which Apply writes the speakers into the text.
type Style uint8

// The styles. StyleNone is the zero value.
const (
	// StyleNone keeps the speakers.
	StyleNone Style = iota
	// StylePrefix writes "ANNA: Is the ferry on time today?".
	StylePrefix
	// StyleDialogueDashes writes a dash before each speaker, in cues with 2 or more speakers.
	StyleDialogueDashes
	// StyleColors writes a <font color> per speaker.
	StyleColors
)

// String returns "none", "prefix", "dialogue-dashes" or "colors", and "Style(n)" for another value.
func (s Style) String() string {
	names := [...]string{"none", "prefix", "dialogue-dashes", "colors"}
	if int(s) < len(names) {
		return names[s]
	}
	return "Style(" + strconv.Itoa(int(s)) + ")"
}

// Report counts the cues whose lines or Speaker changed.
type Report struct {
	ChangedCues int
}

// SpeakerCount is a speaker and the number of cues in which it speaks.
type SpeakerCount struct {
	Name string
	Cues int
}

// work is a cue with its expanded lines: the lines with Speaker written as a voice tag.
type work struct {
	cue     model.Cue
	lines   []string
	changed bool
	styled  bool // a style wrote the lines, so they hold no voice tag and no word timestamp tag
}

// Apply runs the steps that opts selects: labels to speakers, rename, then the style. It returns the error of
// Validate and changes nothing when opts is not valid. A cue that no step changes stays as it is.
func Apply(s *model.Subtitle, opts Options) (Report, error) {
	if err := opts.Validate(); err != nil {
		return Report{}, err
	}
	works := make([]work, 0, s.Len())
	for _, c := range s.All() {
		works = append(works, work{cue: c, lines: expandedLines(c)})
	}
	for i := range works {
		works[i].applyTextSteps(opts)
	}
	if opts.Style != StyleNone {
		colors := colorsBySpeaker(works, opts)
		for i := range works {
			works[i].setStyle(opts, colors)
		}
	}
	var r Report
	for i := range works {
		changed, err := works[i].store(s, i)
		if err != nil {
			return r, err
		}
		if changed {
			r.ChangedCues++
		}
	}
	return r, nil
}

// applyTextSteps runs the label step and the rename step on the expanded lines.
func (w *work) applyTextSteps(opts Options) {
	if opts.FromLabels {
		w.set(fromLabels(w.lines, opts.AnyCaseLabels))
	}
	if len(opts.Rename) > 0 {
		w.set(rename(w.lines, opts.Rename))
	}
}

// store writes the cue back to s when its lines or Speaker differ from the ones it had.
func (w *work) store(s *model.Subtitle, i int) (changed bool, err error) {
	if !w.changed {
		return false, nil
	}
	c, err := w.rebuilt()
	if err != nil || (slices.Equal(c.Lines, w.cue.Lines) && c.Speaker == w.cue.Speaker) {
		return false, err
	}
	return true, s.SetCue(i, c)
}

func (w *work) set(lines []string) {
	if !slices.Equal(lines, w.lines) {
		w.lines = lines
		w.changed = true
	}
}

// List returns the cue count of each speaker, in order of first appearance. A cue counts once for each speaker in it.
func List(s *model.Subtitle) []SpeakerCount {
	var t tally
	for i, c := range s.All() {
		t.add(i, expandedLines(c))
	}
	return t.counts
}

// tally counts the cues of each speaker.
type tally struct {
	counts  []SpeakerCount
	index   map[string]int
	lastCue []int
}

func (t *tally) add(cue int, lines []string) {
	for _, p := range speakerParts(lines) {
		if p.speaker == "" {
			continue
		}
		i, ok := t.index[p.speaker]
		if !ok {
			if t.index == nil {
				t.index = map[string]int{}
			}
			i = len(t.counts)
			t.index[p.speaker] = i
			t.counts = append(t.counts, SpeakerCount{Name: p.speaker})
			t.lastCue = append(t.lastCue, -1)
		}
		if t.lastCue[i] != cue {
			t.lastCue[i] = cue
			t.counts[i].Cues++
		}
	}
}

// expandedLines returns the lines of c with Speaker written as the voice tag that the WebVTT writer writes.
func expandedLines(c model.Cue) []string {
	if c.Speaker == "" {
		return c.Lines
	}
	v := markupcore.Voice{Name: c.Speaker}
	if d, ok := c.FormatData("vtt"); ok {
		v.Class, _ = d.String("voiceClass")
		v.Close, _ = d.Bool("voiceEnd")
	}
	return markupcore.TagWords(c.Lines, nil, v)
}

// rebuilt turns the expanded lines back into lines and Speaker. A single voice that covers the cue becomes Speaker.
func (w *work) rebuilt() (model.Cue, error) {
	c := w.cue
	c.Speaker = ""
	c.Lines = markupcore.CleanLines(w.lines)
	if !w.styled {
		c.ExtractWordTags()
	}
	c.SyncWords()
	if c.Speaker == "" {
		if err := dropVoiceForm(&c); err != nil {
			return c, err
		}
	}
	return c, nil
}

// dropVoiceForm removes the voice class and the closing voice tag from the vtt cue format data of a cue without Speaker.
func dropVoiceForm(c *model.Cue) error {
	d, ok := c.FormatData("vtt")
	if !ok {
		return nil
	}
	_, hasClass := d.Get("voiceClass")
	_, hasEnd := d.Get("voiceEnd")
	if !hasClass && !hasEnd {
		return nil
	}
	d.Delete("voiceClass")
	d.Delete("voiceEnd")
	return c.SetFormatData("vtt", d)
}
