package wordgroup

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/grapheme"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// MaxGap and MaxCharacters are the limits of Group, as in PHP.
const (
	MaxGap        = time.Second
	MaxCharacters = 84
)

// Group splits words into the words of one cue each. A new group starts before a word when its speaker differs from
// the one before, when it starts 1 s or more after the word before ends, when the group would pass 84 characters,
// or when the word before ends a sentence and the word does not start in lower case.
func Group(words []Word) [][]Word {
	var groups [][]Word
	from, length := 0, 0
	for i, w := range words {
		n := grapheme.Count(w.Text)
		if i > from && splits(words[i-1], w, length+1+n) {
			groups = append(groups, words[from:i])
			from = i
		}
		if i == from {
			length = n
		} else {
			length += 1 + n
		}
	}
	if from < len(words) {
		groups = append(groups, words[from:])
	}
	return groups
}

func splits(prev, w Word, length int) bool {
	return prev.HasSpeaker != w.HasSpeaker || prev.Speaker != w.Speaker ||
		w.Start-prev.end() >= MaxGap ||
		length > MaxCharacters ||
		(endsSentence(prev.Text) && !startsLower(w.Text))
}

func endsSentence(s string) bool {
	for {
		r, n := utf8.DecodeLastRuneInString(s)
		if r == utf8.RuneError || !isCloser(r) {
			break
		}
		s = s[:len(s)-n]
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	switch r {
	case '.', '?', '!', '\u3002', '\uff0e', '\uff1f', '\uff01':
		return true
	}
	return false
}

func isCloser(r rune) bool {
	switch r {
	case '"', '\'', '\u2019', '\u201d', ')', ']', '\u300d', '\u300f', '\uff09':
		return true
	}
	return false
}

func startsLower(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && unicode.IsLower(r)
}

// CueWords returns the entries of Cue.Words for the plain text of a cue. Each word with a text and a start is
// searched in the text, from the end of the match before. A word that the text does not hold, and a word that starts
// before the entry before it, gets no entry. The end is the source end, else the next entry start, else cueEnd.
func CueWords(plainText string, words []Word, cueEnd time.Duration) []model.Word {
	kept := foundWords(plainText, words)
	out := make([]model.Word, len(kept))
	for k, w := range kept {
		end := cueEnd
		switch {
		case w.HasEnd:
			end = w.End
		case k+1 < len(kept):
			end = kept[k+1].Start
		}
		out[k] = model.Word{Text: w.Text, Start: w.Start, End: max(end, w.Start)}
		if w.Confidence != nil {
			c := *w.Confidence
			out[k].Confidence = &c
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func foundWords(plainText string, words []Word) []Word {
	var timed []Word
	var texts []string
	for _, w := range words {
		if w.Text != "" && w.Timed {
			timed = append(timed, w)
			texts = append(texts, w.Text)
		}
	}
	var kept []Word
	for k, m := range markupcore.SearchWords(plainText, texts, false) {
		if m.Start >= 0 && (len(kept) == 0 || timed[k].Start >= kept[len(kept)-1].Start) {
			kept = append(kept, timed[k])
		}
	}
	return kept
}
