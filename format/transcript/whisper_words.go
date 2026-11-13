package transcript

import (
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
)

type timeState uint8

const (
	noTime    timeState = iota // not a finite number of 0 or more
	validTime                  // a time below 100000 hours
	overLimit                  // a time of 100000 hours or more
)

// timeOf reads v times unit as a time of whole milliseconds.
func timeOf(v orderedjson.Value, unit time.Duration) (time.Duration, timeState) {
	f, ok := v.AsFloat64()
	if !ok || f < 0 {
		return 0, noTime
	}
	d, ok := v.Duration(unit)
	if !ok {
		return 0, overLimit
	}
	return d, validTime
}

// wordText returns the trimmed string under "word", or "" for another value.
func wordText(item orderedjson.Value) string {
	v, _ := item.Get("word")
	if s, ok := v.AsString(); ok {
		return strings.Trim(s, markupcore.PHPTrimSet)
	}
	return ""
}

// wordConfidence returns "probability", else "score", when it is a number from 0 to 1.
func wordConfidence(item orderedjson.Value) *float64 {
	v, ok := item.Get("probability")
	if !ok || v.Kind() == orderedjson.Null {
		v, _ = item.Get("score")
	}
	f, ok := v.AsFloat64()
	if !ok || f < 0 || f > 1 {
		return nil
	}
	return &f
}

// tokenWord is a word of whisper.cpp tokens: the trimmed text, the first token and the last token.
type tokenWord struct {
	text        string
	first, last orderedjson.Value
	firstIndex  int
}

// tokenWords groups the tokens of a transcription entry into words, as should_split_on_word() of whisper.cpp does.
// A token whose text starts with a space starts a new word. Special tokens such as "[_BEG_]" are no words.
func tokenWords(tokens []orderedjson.Value) []tokenWord {
	var words []tokenWord
	var texts []*strings.Builder
	for k, token := range tokens {
		v, _ := token.Get("text")
		text, ok := v.AsString()
		if !ok || strings.HasPrefix(text, "[_") {
			continue
		}
		if len(words) == 0 || strings.HasPrefix(text, " ") {
			words = append(words, tokenWord{first: token, firstIndex: k})
			texts = append(texts, &strings.Builder{})
		}
		last := len(words) - 1
		words[last].last = token
		texts[last].WriteString(text)
	}
	for i := range words {
		words[i].text = strings.Trim(texts[i].String(), markupcore.PHPTrimSet)
	}
	return words
}

// offset returns the member key of the offsets object of token.
func offset(token orderedjson.Value, key string) orderedjson.Value {
	offsets, _ := token.Get("offsets")
	v, _ := offsets.Get(key)
	return v
}
