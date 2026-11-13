package transcript

import (
	"io"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// WriteAssemblyAI writes s to w as the JSON of an AssemblyAI transcript: the text and the words of the cues, with
// times in whole milliseconds. It writes one utterance per cue when the source had utterances, when a cue has a
// speaker, or when a cue has no words, because the words alone cannot hold such a cue. Else "utterances" is null, as
// AssemblyAI writes it. A value that a cue did not change stays as the reader stored it.
func WriteAssemblyAI(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	return cloudWrite(w, s, model.AssemblyAI, "AssemblyAI JSON", common, assemblyAIDocument)
}

func assemblyAIDocument(s *model.Subtitle) (orderedjson.Value, error) {
	file, err := cloudFileData(s, model.AssemblyAI)
	if err != nil {
		return file, err
	}
	if lang, ok := s.Metadata(model.MetadataLanguage); ok && cloudGet(file, "language_code").Kind() == orderedjson.Null {
		code := strings.ToLower(strings.ReplaceAll(lang, "-", "_"))
		file = file.With("language_code", orderedjson.StringValue(code))
	}

	var utterances, words []orderedjson.Value
	var texts []string
	hasUtterances := false
	for _, c := range s.All() {
		data := cloudCueData(&c, model.AssemblyAI)
		u := assemblyAIUtterance(&c, data)
		text, _ := cloudGet(u, "text").AsString()
		texts = append(texts, text)
		words = append(words, cloudGet(u, "words").Items()...)
		utterances = append(utterances, u)
		hasUtterances = hasUtterances || len(cloudGet(u, "words").Items()) == 0 ||
			cloudGet(u, "speaker").Kind() != orderedjson.Null || cloudGet(data, "text").Kind() != orderedjson.Null
	}
	list := orderedjson.Value{}
	if hasUtterances {
		list = orderedjson.ArrayOf(utterances...)
	}
	return orderedjson.Replace(file, orderedjson.ObjectOf(
		cloudMember("text", orderedjson.StringValue(strings.Join(texts, " "))),
		cloudMember("words", orderedjson.ArrayOf(words...)),
		cloudMember("utterances", list),
	)), nil
}

func assemblyAIUtterance(c *model.Cue, data orderedjson.Value) orderedjson.Value {
	stored := cloudGet(data, "words").Items()
	grouped := cloudGet(data, "text").Kind() == orderedjson.Null
	startV, endV, textV := assemblyAISources(data, stored, grouped)
	sameStart := wordgroup.SameTime(assemblyAINumber(startV), time.Millisecond, c.Start)
	sameEnd := wordgroup.SameTime(assemblyAINumber(endV), time.Millisecond, c.End)
	sameText := wordgroup.SameText(c, textV)
	speaker := wordgroup.CueSpeaker(c)

	words := assemblyAIWords(c, stored, sameStart && sameEnd, sameText, speaker)
	storedSpeaker := cloudGet(data, "speaker")
	if name, isString := storedSpeaker.AsString(); speaker != "" && (!isString || name != speaker) {
		words = assemblyAIWithSpeaker(words, speaker)
	}
	base := data
	if grouped {
		base = orderedjson.ObjectOf(cloudMember("confidence", orderedjson.Float(1)))
	}
	if !sameStart {
		startV = assemblyAIMillis(c.Start)
	}
	if !sameEnd {
		endV = assemblyAIMillis(c.End)
	}
	text := textV
	if !sameText {
		text = orderedjson.StringValue(wordgroup.CueText(c))
	}
	outSpeaker := assemblyAISpeaker(storedSpeaker, speaker, grouped)
	return orderedjson.Replace(base, orderedjson.ObjectOf(
		cloudMember("end", endV), cloudMember("speaker", outSpeaker), cloudMember("start", startV),
		cloudMember("text", text), cloudMember("words", orderedjson.ArrayOf(words...)),
	))
}

func assemblyAIWithSpeaker(words []orderedjson.Value, speaker string) []orderedjson.Value {
	for i, w := range words {
		if w.Kind() == orderedjson.Object {
			words[i] = w.With("speaker", orderedjson.StringValue(speaker))
		}
	}
	return words
}

// assemblyAISpeaker returns the cue speaker, else the stored speaker of an utterance cue, else null.
func assemblyAISpeaker(stored orderedjson.Value, speaker string, grouped bool) orderedjson.Value {
	switch {
	case speaker != "":
		return orderedjson.StringValue(speaker)
	case grouped:
		return orderedjson.Value{}
	}
	return stored
}

// assemblyAISources returns the stored start, end and text of the cue. A grouped cue has no stored text, so the values
// come from its words.
func assemblyAISources(data orderedjson.Value, stored []orderedjson.Value, grouped bool) (start, end, text orderedjson.Value) {
	if !grouped {
		return cloudGet(data, "start"), cloudGet(data, "end"), cloudGet(data, "text")
	}
	if len(stored) > 0 {
		start, end = cloudGet(stored[0], "start"), cloudGet(stored[len(stored)-1], "end")
	}
	texts := make([]string, len(stored))
	for i, w := range stored {
		texts[i] = assemblyAIWordText(w)
	}
	return start, end, orderedjson.StringValue(strings.Join(texts, " "))
}

func assemblyAIWordText(w orderedjson.Value) string {
	s, _ := cloudGet(w, "text").AsString()
	return strings.Trim(s, markupcore.PHPTrimSet)
}

// assemblyAINumber returns v when it is a number, else null.
func assemblyAINumber(v orderedjson.Value) orderedjson.Value {
	if v.Kind() == orderedjson.Number {
		return v
	}
	return orderedjson.Value{}
}

func assemblyAIMillis(d time.Duration) orderedjson.Value {
	return orderedjson.Int(max(0, d).Round(time.Millisecond).Milliseconds())
}

func assemblyAIWords(c *model.Cue, stored []orderedjson.Value, sameTimes, sameText bool, speaker string) []orderedjson.Value {
	source := make([]wordgroup.SourceWord, len(stored))
	for i, w := range stored {
		source[i] = wordgroup.SourceWord{Text: assemblyAIWordText(w), Confidence: cloudConfidence(cloudGet(w, "confidence"))}
		source[i].Start = assemblyAIStoredTime(cloudGet(w, "start"))
		source[i].End = assemblyAIStoredTime(cloudGet(w, "end"))
	}
	words := []orderedjson.Value{}
	choices, ok := wordgroup.Reuse(c, source, sameTimes, sameText)
	if !ok {
		return words
	}
	label := orderedjson.Value{}
	if speaker != "" {
		label = orderedjson.StringValue(speaker)
	}
	for _, ch := range choices {
		if ch.New != nil {
			words = append(words, orderedjson.ObjectOf(
				cloudMember("text", orderedjson.StringValue(ch.New.Text)), cloudMember("start", assemblyAIMillis(ch.New.Start)),
				cloudMember("end", assemblyAIMillis(ch.New.End)), cloudMember("confidence", orderedjson.Float(ch.New.Confidence)),
				cloudMember("speaker", label),
			))
			continue
		}
		for _, i := range ch.Source {
			item := stored[i]
			if ch.Word != nil {
				item = assemblyAITyped(item, ch.Word)
			}
			words = append(words, item)
		}
	}
	return words
}

func assemblyAIStoredTime(v orderedjson.Value) *time.Duration {
	if v.Kind() != orderedjson.Number {
		return nil
	}
	d, ok := v.Duration(time.Millisecond)
	if !ok {
		return nil
	}
	return &d
}

// assemblyAITyped gives a reused word the End and the Confidence of its entry in Cue.Words, for each member that the
// word has and that differs.
func assemblyAITyped(item orderedjson.Value, w *model.Word) orderedjson.Value {
	if item.Kind() != orderedjson.Object {
		return item
	}
	if v, ok := item.Get("end"); ok && !wordgroup.SameTime(assemblyAINumber(v), time.Millisecond, w.End) {
		item = item.With("end", assemblyAIMillis(w.End))
	}
	if v, ok := item.Get("confidence"); ok && w.Confidence != nil {
		if f, isNumber := v.AsFloat64(); !isNumber || f != *w.Confidence {
			item = item.With("confidence", orderedjson.Float(*w.Confidence))
		}
	}
	return item
}
