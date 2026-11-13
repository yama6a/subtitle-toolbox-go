package transcript

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// WriteDeepgram writes s to w as the JSON response of the Deepgram pre-recorded audio API. Each channel gets the
// transcript and the words of its cues, and the paragraphs when the source had them. It writes one utterance per
// cue when the source had utterances, or when a cue outside paragraphs has no words, because the words alone cannot
// hold such a cue. A value that a cue did not change stays as the reader stored it.
func WriteDeepgram(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.Deepgram, common)
	if err != nil {
		return err
	}
	doc, err := deepgramDocument(s)
	if err != nil {
		return err
	}
	body, err := encodeDocument(model.Deepgram, "Deepgram JSON", doc, false)
	if err != nil {
		return err
	}
	return writing.Write(w, model.Deepgram, body, common, false)
}

// deepgramEntry is the written form of one cue.
type deepgramEntry struct {
	start, end orderedjson.Value
	text       string
	speaker    orderedjson.Value // null, an integer or a string
	words      []orderedjson.Value
	sentence   orderedjson.Value // the stored sentence members of a sentence cue, else an empty object
	paragraph  orderedjson.Value // the stored paragraph object, else null
}

func deepgramDocument(s *model.Subtitle) (orderedjson.Value, error) {
	file, err := cloudFileData(s, model.Deepgram)
	if err != nil {
		return file, err
	}
	results, _ := file.Get("results")
	if results.Kind() != orderedjson.Object {
		results = orderedjson.ObjectOf()
	}
	channels := append([]orderedjson.Value(nil), cloudGet(results, "channels").Items()...)

	var utterances []orderedjson.Value
	byChannel := map[int][]deepgramEntry{}
	hasUtterances := false
	for _, c := range s.All() {
		data := storedObject(&c, deepgramKey)
		channel := deepgramChannel(data, len(channels)+s.Len())
		e := deepgramCue(&c, data)
		transcript := cloudGet(data, "transcript").Kind() != orderedjson.Null
		hasUtterances = hasUtterances || transcript || (len(e.words) == 0 && e.paragraph.Kind() == orderedjson.Null)
		utterances = append(utterances, deepgramUtterance(data, e, channel, transcript))
		byChannel[channel] = append(byChannel[channel], e)
	}

	language, hasLanguage := s.Metadata(model.MetadataLanguage)
	if !hasLanguage {
		language = ""
	}
	channels = deepgramChannels(channels, byChannel, language, hasLanguage)

	out := orderedjson.ObjectOf(member("channels", orderedjson.ArrayOf(channels...)))
	if hasUtterances {
		out = orderedjson.Union(out, orderedjson.ObjectOf(member("utterances", orderedjson.ArrayOf(utterances...))))
	}
	out = orderedjson.Union(out, results)
	return orderedjson.Union(orderedjson.ObjectOf(member("results", out)), file), nil
}

// deepgramChannels returns one channel object for each stored channel and each channel that a cue names.
func deepgramChannels(stored []orderedjson.Value, byChannel map[int][]deepgramEntry, language string, hasLanguage bool) []orderedjson.Value {
	count := max(len(stored), 1)
	for ch := range byChannel {
		count = max(count, ch+1)
	}
	channels := make([]orderedjson.Value, count)
	for i := range channels {
		channels[i] = orderedjson.ObjectOf()
		if i < len(stored) && stored[i].Kind() == orderedjson.Object {
			channels[i] = stored[i]
		}
		if i == 0 && hasLanguage {
			channels[i] = orderedjson.Union(channels[i], orderedjson.ObjectOf(member("detected_language", orderedjson.StringValue(language))))
		}
		channels[i] = deepgramChannelOut(channels[i], byChannel[i])
	}
	return channels
}

// deepgramChannel returns the channel of the cue: the stored integer from 0 to limit minus 1, else 0.
func deepgramChannel(data orderedjson.Value, limit int) int {
	if n, ok := cloudGet(data, "channel").AsInt64(); ok && n >= 0 && n < int64(limit) {
		return int(n)
	}
	return 0
}

func deepgramUtterance(data orderedjson.Value, e deepgramEntry, channel int, transcript bool) orderedjson.Value {
	base := orderedjson.ObjectOf(
		member("start", orderedjson.Value{}), member("end", orderedjson.Value{}), member("confidence", orderedjson.Float(1)),
	)
	stored := orderedjson.ObjectOf()
	if transcript {
		stored = data
	}
	with := []orderedjson.Value{stored, orderedjson.ObjectOf(
		member("start", e.start), member("end", e.end), member("channel", orderedjson.Int(int64(channel))),
		member("transcript", orderedjson.StringValue(e.text)), member("words", orderedjson.ArrayOf(e.words...)),
	)}
	if e.speaker.Kind() != orderedjson.Null {
		with = append(with, orderedjson.ObjectOf(member("speaker", e.speaker)))
	}
	return orderedjson.Replace(base, with...)
}

// deepgramCue returns the start, end, text, speaker, words, sentence and paragraph of the cue. A field keeps its
// stored form when the cue did not change it.
func deepgramCue(c *model.Cue, data orderedjson.Value) deepgramEntry {
	stored := cloudGet(data, "words").Items()
	textKey := "text"
	if cloudGet(data, "transcript").Kind() != orderedjson.Null {
		textKey = "transcript"
	}
	grouped := cloudGet(data, textKey).Kind() == orderedjson.Null
	startV, endV, textV := deepgramSources(data, textKey, stored, grouped)
	sameStart := wordgroup.SameTime(deepgramNumber(startV), time.Second, c.Start)
	sameEnd := wordgroup.SameTime(deepgramNumber(endV), time.Second, c.End)
	sameText := wordgroup.SameText(c, textV)
	speaker := deepgramSpeaker(wordgroup.CueSpeaker(c))

	e := deepgramEntry{
		start: startV, end: endV, speaker: speaker, text: deepgramString(textV),
		words: deepgramWords(c, stored, sameStart && sameEnd, sameText, speaker),
	}
	if !sameStart {
		e.start = orderedjson.Seconds(max(0, c.Start))
	}
	if !sameEnd {
		e.end = orderedjson.Seconds(max(0, c.End))
	}
	if !sameText {
		e.text = wordgroup.CueText(c)
	}
	if storedSpeaker := cloudGet(data, "speaker"); speaker.Kind() != orderedjson.Null && !orderedjson.Equal(speaker, storedSpeaker) {
		e.words = deepgramWithSpeaker(e.words, speaker)
	}
	e.sentence, e.paragraph = deepgramShape(data, textKey == "text" && !grouped)
	return e
}

// deepgramShape returns the stored sentence members of a sentence cue, else an empty object, and the stored paragraph
// object, else null.
func deepgramShape(data orderedjson.Value, sentenceCue bool) (sentence, paragraph orderedjson.Value) {
	sentence = orderedjson.ObjectOf()
	if sentenceCue {
		sentence = data.Without("channel", "speaker", "paragraph", "words")
	}
	if p := cloudGet(data, "paragraph"); p.Kind() == orderedjson.Object {
		paragraph = p
	}
	return sentence, paragraph
}

// deepgramSources returns the stored start, end and text of the cue. A grouped cue has no stored text, so the values
// come from its words.
func deepgramSources(data orderedjson.Value, textKey string, stored []orderedjson.Value, grouped bool) (start, end, text orderedjson.Value) {
	if !grouped {
		return cloudGet(data, "start"), cloudGet(data, "end"), cloudGet(data, textKey)
	}
	if len(stored) > 0 {
		start, end = cloudGet(stored[0], "start"), cloudGet(stored[len(stored)-1], "end")
	}
	texts := make([]string, len(stored))
	for i, w := range stored {
		texts[i] = deepgramWordText(w)
	}
	return start, end, orderedjson.StringValue(strings.Join(texts, " "))
}

func deepgramWithSpeaker(words []orderedjson.Value, speaker orderedjson.Value) []orderedjson.Value {
	for i, w := range words {
		if w.Kind() == orderedjson.Object {
			words[i] = w.With("speaker", speaker)
		}
	}
	return words
}

// deepgramSpeaker returns the voice name as an integer when it has only the digits 0 to 9, else as a string.
func deepgramSpeaker(name string) orderedjson.Value {
	if name == "" {
		return orderedjson.Value{}
	}
	if n, err := strconv.ParseInt(name, 10, 64); err == nil && strings.Trim(name, "0123456789") == "" {
		return orderedjson.Int(n)
	}
	return orderedjson.StringValue(name)
}

// deepgramNumber returns v when it is a number, else null.
func deepgramNumber(v orderedjson.Value) orderedjson.Value {
	if v.Kind() == orderedjson.Number {
		return v
	}
	return orderedjson.Value{}
}

func deepgramWordText(w orderedjson.Value) string {
	v := cloudGet(w, "punctuated_word")
	if v.Kind() == orderedjson.Null {
		v = cloudGet(w, "word")
	}
	s, _ := v.AsString()
	return strings.Trim(s, markupcore.PHPTrimSet)
}

func deepgramWords(c *model.Cue, stored []orderedjson.Value, sameTimes, sameText bool, speaker orderedjson.Value) []orderedjson.Value {
	source := make([]wordgroup.SourceWord, len(stored))
	for i, w := range stored {
		source[i] = wordgroup.SourceWord{Text: deepgramWordText(w), Confidence: cloudConfidence(cloudGet(w, "confidence"))}
		source[i].Start = cloudStoredTime(deepgramNumber(cloudGet(w, "start")))
		source[i].End = cloudStoredTime(deepgramNumber(cloudGet(w, "end")))
	}
	choices, ok := wordgroup.Reuse(c, source, sameTimes, sameText)
	if !ok {
		return []orderedjson.Value{}
	}
	words := []orderedjson.Value{}
	for _, ch := range choices {
		if ch.New != nil {
			words = append(words, deepgramNewWord(*ch.New, speaker))
			continue
		}
		for _, i := range ch.Source {
			item := stored[i]
			if ch.Word != nil {
				item = deepgramTyped(item, ch.Word)
			}
			words = append(words, item)
		}
	}
	return words
}

// deepgramTyped gives a reused word the End and the Confidence of its entry in Cue.Words, for each member that the
// word has and that differs.
func deepgramTyped(item orderedjson.Value, w *model.Word) orderedjson.Value {
	if item.Kind() != orderedjson.Object {
		return item
	}
	if v, ok := item.Get("end"); ok && !wordgroup.SameTime(deepgramNumber(v), time.Second, w.End) {
		item = item.With("end", orderedjson.Seconds(w.End))
	}
	if v, ok := item.Get("confidence"); ok && w.Confidence != nil {
		if f, isNumber := v.AsFloat64(); !isNumber || f != *w.Confidence {
			item = item.With("confidence", orderedjson.Float(*w.Confidence))
		}
	}
	return item
}

func deepgramNewWord(n wordgroup.NewWord, speaker orderedjson.Value) orderedjson.Value {
	word := strings.Trim(strings.ToLower(n.Text), " \t\n\r\x00\x0b.,?!;:\"'()[]")
	if word == "" {
		word = n.Text
	}
	m := []orderedjson.Member{
		member("word", orderedjson.StringValue(word)), member("start", orderedjson.Seconds(n.Start)),
		member("end", orderedjson.Seconds(n.End)), member("confidence", orderedjson.Float(n.Confidence)),
	}
	if speaker.Kind() != orderedjson.Null {
		m = append(m, member("speaker", speaker))
	}
	return orderedjson.ObjectOf(append(m, member("punctuated_word", orderedjson.StringValue(n.Text)))...)
}

// deepgramChannelOut returns the channel with the transcript and the words of its entries.
func deepgramChannelOut(channel orderedjson.Value, entries []deepgramEntry) orderedjson.Value {
	alt := orderedjson.ObjectOf()
	if first := cloudGet(channel, "alternatives").Items(); len(first) > 0 && first[0].Kind() == orderedjson.Object {
		alt = first[0]
	}
	texts := make([]string, len(entries))
	var words []orderedjson.Value
	hasParagraph := false
	for i, e := range entries {
		texts[i] = e.text
		words = append(words, e.words...)
		hasParagraph = hasParagraph || len(e.paragraph.Members()) > 0
	}
	alt = orderedjson.Union(orderedjson.ObjectOf(member("transcript", orderedjson.StringValue(strings.Join(texts, " ")))), alt)
	alt = orderedjson.Union(alt, orderedjson.ObjectOf(member("words", orderedjson.ArrayOf(words...))))
	if hasParagraph {
		alt = alt.With("paragraphs", deepgramParagraphs(entries))
	}
	return orderedjson.Union(orderedjson.ObjectOf(member("alternatives", orderedjson.ArrayOf(alt))), channel)
}

type deepgramParagraph struct {
	source  orderedjson.Value
	speaker orderedjson.Value
	entries []deepgramEntry
}

// deepgramGroup splits the entries into paragraphs.
func deepgramGroup(entries []deepgramEntry) []deepgramParagraph {
	var paragraphs []deepgramParagraph
	var previous orderedjson.Value
	for _, e := range entries {
		n := len(paragraphs)
		join := n > 0
		if join && e.paragraph.Kind() == orderedjson.Null {
			join = orderedjson.Equal(e.speaker, paragraphs[n-1].speaker)
		} else if join {
			join = orderedjson.Equal(e.paragraph, previous)
		}
		if !join {
			source := e.paragraph
			if source.Kind() != orderedjson.Object {
				source = orderedjson.ObjectOf()
			}
			paragraphs = append(paragraphs, deepgramParagraph{source: source, speaker: e.speaker})
			n++
		}
		paragraphs[n-1].entries = append(paragraphs[n-1].entries, e)
		previous = e.paragraph
	}
	return paragraphs
}

// deepgramParagraphs returns the paragraphs of the entries. An entry starts a new paragraph when its stored paragraph
// differs from the one of the entry before it. An entry without a stored paragraph joins the paragraph before it when
// it has the same speaker.
func deepgramParagraphs(entries []deepgramEntry) orderedjson.Value {
	paragraphs := deepgramGroup(entries)
	var transcript strings.Builder
	list := make([]orderedjson.Value, len(paragraphs))
	for i, p := range paragraphs {
		sentences := make([]orderedjson.Value, len(p.entries))
		texts := make([]string, len(p.entries))
		count := 0
		for k, e := range p.entries {
			sentences[k] = orderedjson.Replace(e.sentence, orderedjson.ObjectOf(
				member("text", orderedjson.StringValue(e.text)), member("start", e.start), member("end", e.end),
			))
			texts[k] = e.text
			count += len(e.words)
		}
		first, last := p.entries[0], p.entries[len(p.entries)-1]
		base := orderedjson.ObjectOf(member("sentences", orderedjson.ArrayOf(sentences...)))
		speaker := orderedjson.ObjectOf()
		if p.speaker.Kind() != orderedjson.Null {
			speaker = orderedjson.ObjectOf(member("speaker", p.speaker))
		}
		list[i] = orderedjson.Replace(orderedjson.Union(base, speaker), p.source, orderedjson.ObjectOf(
			member("sentences", orderedjson.ArrayOf(sentences...)), member("num_words", orderedjson.Int(int64(count))),
			member("start", first.start), member("end", last.end),
		), speaker)
		transcript.WriteString("\n")
		if p.speaker.Kind() != orderedjson.Null {
			transcript.WriteString("Speaker " + deepgramSpeakerText(p.speaker) + ": ")
		}
		transcript.WriteString(strings.Join(texts, " ") + "\n")
	}
	text := strings.TrimSuffix(transcript.String(), "\n")
	return orderedjson.ObjectOf(member("transcript", orderedjson.StringValue(text)), member("paragraphs", orderedjson.ArrayOf(list...)))
}

func deepgramSpeakerText(v orderedjson.Value) string {
	if s, ok := v.AsString(); ok {
		return s
	}
	text, _ := v.Text()
	return text
}

func deepgramString(v orderedjson.Value) string {
	s, _ := v.AsString()
	return s
}
