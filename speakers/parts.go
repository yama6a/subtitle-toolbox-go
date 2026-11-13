package speakers

import (
	"regexp"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

var (
	voiceTagExact = regexp.MustCompile(`^` + markupcore.VoiceTagStartPattern + `(?:[ \t\n\r\f\v]+([^>]*))?>$`)
	voiceEndExact = regexp.MustCompile(`^</v[ \t\n\r\f\v]*>$`)
	openStyleTag  = regexp.MustCompile(`^<([a-zA-Z][a-zA-Z0-9]*)(?:[\s.][^>]*)?>$`)
	closeStyleTag = regexp.MustCompile(`^</([a-zA-Z][a-zA-Z0-9]*)\s*>$`)
	voiceTagAny   = regexp.MustCompile(markupcore.VoiceTagPattern)
	voiceInLines  = regexp.MustCompile(`</?v[ \t\n\r\f\v.>]`)
)

// part is the text of one speaker in one line, with the style tags that were open there.
type part struct {
	speaker string // "" for no speaker
	text    string
	starts  bool // the part has visible text and its speaker differs from the one before
}

type openTag struct{ name, tag string }

// splitter holds the speaker and the open style tags while it splits the lines of one cue.
type splitter struct {
	speaker string
	open    []openTag
	parts   []part
}

// speakerParts splits the expanded lines of a cue into parts with one speaker each. A speaker that changes in the
// middle of a line starts a new part, and the style tags that are open there close and open again. The parts hold
// no voice tag.
func speakerParts(lines []string) []part {
	var st splitter
	for _, line := range lines {
		st.splitLine(line)
	}
	prev, seen := "", false
	for i, p := range st.parts {
		hasText := markupcore.VisibleText(p.text) != ""
		st.parts[i].starts = hasText && (!seen || p.speaker != prev)
		if hasText {
			prev, seen = p.speaker, true
		}
	}
	return st.parts
}

func (st *splitter) splitLine(line string) {
	var cur strings.Builder
	hasText, split := false, false
	for i, tok := range markupcore.SplitTags(line) {
		if i%2 == 0 {
			if split && !hasText {
				tok = strings.TrimLeft(tok, markupcore.PHPTrimSet)
			}
			cur.WriteString(tok)
			hasText = hasText || markupcore.VisibleText(tok) != ""
			continue
		}
		name, isVoice := voiceName(tok)
		if !isVoice {
			st.track(tok)
			cur.WriteString(tok)
			continue
		}
		if name == st.speaker {
			continue
		}
		if hasText {
			st.parts = append(st.parts, part{speaker: st.speaker, text: strings.TrimRight(cur.String(), markupcore.PHPTrimSet) + st.closing()})
			cur.Reset()
			for _, o := range st.open {
				cur.WriteString(o.tag)
			}
			hasText, split = false, true
		}
		st.speaker = name
	}
	st.parts = append(st.parts, part{speaker: st.speaker, text: cur.String()})
}

// voiceName reports whether tok is a voice tag or a closing voice tag. The name is "" for no speaker.
func voiceName(tok string) (name string, ok bool) {
	if m := voiceTagExact.FindStringSubmatch(tok); m != nil {
		return markupcore.DecodeEntities(strings.Trim(m[2], markupcore.PHPTrimSet)), true
	}
	return "", voiceEndExact.MatchString(tok)
}

func (st *splitter) track(tok string) {
	if m := openStyleTag.FindStringSubmatch(tok); m != nil {
		st.open = append(st.open, openTag{m[1], tok})
	} else if m := closeStyleTag.FindStringSubmatch(tok); m != nil {
		for i := len(st.open) - 1; i >= 0; i-- {
			if st.open[i].name == m[1] {
				st.open = append(st.open[:i], st.open[i+1:]...)
				break
			}
		}
	}
}

func (st *splitter) closing() string {
	var b strings.Builder
	for i := len(st.open) - 1; i >= 0; i-- {
		b.WriteString("</" + st.open[i].name + ">")
	}
	return b.String()
}
