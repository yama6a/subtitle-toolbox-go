package transcript

import (
	"io"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// WriteAssemblyAI writes s to w as the JSON of an AssemblyAI transcript: the text and the words of the cues, with
// times in whole milliseconds. It writes one utterance per cue when the source had utterances, when a cue has a
// speaker, or when a cue has no words, because the words alone cannot hold such a cue. Else "utterances" is null, as
// AssemblyAI writes it. A value that a cue did not change stays as the reader stored it.
func WriteAssemblyAI(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.AssemblyAI, common)
	if err != nil {
		return err
	}
	doc, err := assemblyAIDocument(s)
	if err != nil {
		return err
	}
	body, err := encodeDocument(model.AssemblyAI, "AssemblyAI JSON", doc, false)
	if err != nil {
		return err
	}
	return writing.Write(w, model.AssemblyAI, body, common, false)
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
		data := storedObject(&c, assemblyAIKey)
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
		member("text", orderedjson.StringValue(strings.Join(texts, " "))),
		member("words", orderedjson.ArrayOf(words...)),
		member("utterances", list),
	)), nil
}

func assemblyAIUtterance(c *model.Cue, data orderedjson.Value) orderedjson.Value {
	stored := cloudGet(data, "words").Items()
	grouped := cloudGet(data, "text").Kind() == orderedjson.Null
	startV, endV, textV := cloudSources(data, "text", stored, grouped, assemblyAIWordText)
	sameStart := wordgroup.SameTime(cloudNumber(startV), time.Millisecond, c.Start)
	sameEnd := wordgroup.SameTime(cloudNumber(endV), time.Millisecond, c.End)
	sameText := wordgroup.SameText(c, textV)
	speaker := wordgroup.CueSpeaker(c)

	words := assemblyAIWords(c, stored, sameStart && sameEnd, sameText, speaker)
	storedSpeaker := cloudGet(data, "speaker")
	if name, isString := storedSpeaker.AsString(); speaker != "" && (!isString || name != speaker) {
		words = cloudWithSpeaker(words, orderedjson.StringValue(speaker))
	}
	base := data
	if grouped {
		base = orderedjson.ObjectOf(member("confidence", orderedjson.Float(1)))
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
		member("end", endV), member("speaker", outSpeaker), member("start", startV),
		member("text", text), member("words", orderedjson.ArrayOf(words...)),
	))
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

func assemblyAIWordText(w orderedjson.Value) string {
	s, _ := cloudGet(w, "text").AsString()
	return strings.Trim(s, markupcore.PHPTrimSet)
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
				member("text", orderedjson.StringValue(ch.New.Text)), member("start", assemblyAIMillis(ch.New.Start)),
				member("end", assemblyAIMillis(ch.New.End)), member("confidence", orderedjson.Float(ch.New.Confidence)),
				member("speaker", label),
			))
			continue
		}
		for _, i := range ch.Source {
			item := stored[i]
			if ch.Word != nil {
				item = cloudTyped(item, ch.Word, time.Millisecond, assemblyAIMillis)
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
