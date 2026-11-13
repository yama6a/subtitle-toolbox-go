package transcript

import (
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// googleOperationFields are the members of a long-running operation that stay around the response.
var googleOperationFields = []string{"name", "metadata", "done", "error"} //nolint:gochecknoglobals // a lookup table

// WriteGoogleSpeech writes s to w as the JSON response of Google Cloud Speech-to-Text with the field names of V1 or V2
// as the source has them, V1 without a source. It writes one result per cue. A source with speaker diarization gives
// one result with all words and their speakers. A long-running operation of the source stays around the response.
// A value that a cue did not change stays as the reader stored it.
func WriteGoogleSpeech(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.GoogleSpeech, common)
	if err != nil {
		return err
	}
	doc, err := googleDocument(s)
	if err != nil {
		return err
	}
	body, err := encodeDocument(model.GoogleSpeech, "Google Speech-to-Text JSON", doc, false)
	if err != nil {
		return err
	}
	return writing.Write(w, model.GoogleSpeech, body, common, false)
}

func googleDocument(s *model.Subtitle) (orderedjson.Value, error) {
	file, err := cloudFileData(s, model.GoogleSpeech)
	if err != nil {
		return file, err
	}
	var cues []*model.Cue
	var data []orderedjson.Value
	for _, c := range s.All() {
		cues = append(cues, &c)
		data = append(data, storedObject(&c, googleKey))
	}
	g := &googleWriter{v2: googleIsV2(file, data)}
	var results []orderedjson.Value
	if stored := cloudGet(file, "results"); stored.Kind() == orderedjson.Array || stored.Kind() == orderedjson.Object {
		result, err := g.diarized(cues, data, googleFirstResult(stored))
		if err != nil {
			return result, err
		}
		results = []orderedjson.Value{result}
	} else {
		language, hasLanguage := s.Metadata(model.MetadataLanguage)
		for i, c := range cues {
			results = append(results, g.result(c, data[i], language, hasLanguage))
		}
	}
	return googleEnvelope(file, results), nil
}

// googleEnvelope returns the response with the results, inside the operation of the source when it has one.
func googleEnvelope(file orderedjson.Value, results []orderedjson.Value) orderedjson.Value {
	response := file.Without("results")
	isOperation := cloudGet(file, "done").Kind() != orderedjson.Null || cloudGet(file, "name").Kind() != orderedjson.Null
	if isOperation {
		response = response.Without(googleOperationFields...)
	}
	response = orderedjson.Union(orderedjson.ObjectOf(member("results", orderedjson.ArrayOf(results...))), response)
	if !isOperation {
		return response
	}
	var operation []orderedjson.Member
	for _, m := range file.Members() {
		if slices.Contains(googleOperationFields, m.Key) {
			operation = append(operation, m)
		}
	}
	return orderedjson.Union(orderedjson.ObjectOf(operation...), orderedjson.ObjectOf(member("response", response)))
}

func googleFirstResult(stored orderedjson.Value) orderedjson.Value {
	if items := stored.Items(); len(items) > 0 && items[0].Kind() == orderedjson.Object {
		return items[0]
	}
	return orderedjson.ObjectOf()
}

// googleIsV2 reports whether a stored result or a cue has a resultEndOffset or a first word with a startOffset.
func googleIsV2(file orderedjson.Value, cues []orderedjson.Value) bool {
	for _, list := range [][]orderedjson.Value{cloudGet(file, "results").Items(), cues} {
		for _, d := range list {
			word := orderedjson.Value{}
			if words := cloudGet(d, "words").Items(); len(words) > 0 {
				word = words[0]
			}
			if cloudGet(d, "resultEndOffset").Kind() != orderedjson.Null || cloudGet(word, "startOffset").Kind() != orderedjson.Null {
				return true
			}
		}
	}
	return false
}

type googleWriter struct {
	v2          bool
	previousEnd time.Duration
}

// googleEntry is the written form of one cue: its text, its words, and whether it has the times of its source.
type googleEntry struct {
	text      string
	words     []orderedjson.Value
	sameTimes bool
}

// result returns the result of a cue: the stored members of the result, and its first alternative with the transcript
// and the words. The result end of an overlapping cue moves to the end of the result before it, because results
// follow each other in the audio.
func (g *googleWriter) result(c *model.Cue, data orderedjson.Value, language string, hasLanguage bool) orderedjson.Value {
	e := g.entry(c, data, false)
	endKey := "resultEndOffset"
	if cloudGet(data, "resultEndTime").Kind() != orderedjson.Null || (!g.v2 && cloudGet(data, "resultEndOffset").Kind() == orderedjson.Null) {
		endKey = "resultEndTime"
	}
	fields := orderedjson.Union(data.Without("transcript", "confidence", "words"), orderedjson.ObjectOf(member(endKey, orderedjson.Value{})))
	if len(data.Members()) == 0 && hasLanguage {
		fields = orderedjson.Union(fields, orderedjson.ObjectOf(member("languageCode", orderedjson.StringValue(language))))
	}
	end := max(g.previousEnd, c.End)
	written := orderedjson.StringValue(googleDurationText(end))
	if stored := cloudGet(data, endKey); e.sameTimes && stored.Kind() != orderedjson.Null {
		written = stored
	}
	g.previousEnd = end
	if d, ok := googleStoredDuration(written); ok && googleStoredTime(d) != nil {
		g.previousEnd = *googleStoredTime(d)
	}
	return orderedjson.Union(orderedjson.ObjectOf(member("alternatives", orderedjson.ArrayOf(googleAlternative(data, e)))), fields.With(endKey, written))
}

// googleAlternative returns the first alternative of a result: the stored transcript, confidence and words in their
// order, with the transcript set to the cue text and the words set when the cue has words or the data had them.
func googleAlternative(data orderedjson.Value, e googleEntry) orderedjson.Value {
	with := []orderedjson.Value{
		googleKeep(data, "transcript", "confidence", "words"),
		orderedjson.ObjectOf(member("transcript", orderedjson.StringValue(e.text))),
	}
	if len(e.words) > 0 || cloudGet(data, "words").Kind() != orderedjson.Null {
		with = append(with, orderedjson.ObjectOf(member("words", orderedjson.ArrayOf(e.words...))))
	}
	return orderedjson.Replace(orderedjson.ObjectOf(member("transcript", orderedjson.Value{})), with...)
}

// googleKeep returns the members of data under keys, in the order of data.
func googleKeep(data orderedjson.Value, keys ...string) orderedjson.Value {
	var m []orderedjson.Member
	for _, member := range data.Members() {
		for _, k := range keys {
			if member.Key == k {
				m = append(m, member)
			}
		}
	}
	return orderedjson.ObjectOf(m...)
}

// diarized returns the one result of a source with speaker diarization: all words with their speakers.
func (g *googleWriter) diarized(cues []*model.Cue, data []orderedjson.Value, source orderedjson.Value) (orderedjson.Value, error) {
	var texts []string
	var words []orderedjson.Value
	for i, c := range cues {
		e := g.entry(c, data[i], true)
		if len(e.words) == 0 {
			return source, &model.UnwritableError{
				Format: model.GoogleSpeech, CueIndex: i,
				Reason: "A Google Speech-to-Text result with speaker diarization holds cues only through their words. Cue #" +
					strconv.Itoa(i) + " at " + timecode.Seconds(c.Start) + " s has no words. " +
					"Read the file with word timestamps before you change the number of words.",
			}
		}
		texts = append(texts, e.text)
		words = append(words, e.words...)
	}
	alt := orderedjson.ObjectOf()
	if first := cloudGet(source, "alternatives").Items(); len(first) > 0 && first[0].Kind() == orderedjson.Object {
		alt = first[0]
	}
	alt = orderedjson.Union(orderedjson.ObjectOf(member("transcript", orderedjson.StringValue(strings.Join(texts, " ")))), alt)
	alt = orderedjson.Union(alt, orderedjson.ObjectOf(member("words", orderedjson.ArrayOf(words...))))
	return orderedjson.Union(orderedjson.ObjectOf(member("alternatives", orderedjson.ArrayOf(alt))), source), nil
}

// entry returns the text and the words of the cue, and whether the cue has the times of its source.
func (g *googleWriter) entry(c *model.Cue, data orderedjson.Value, diarized bool) googleEntry {
	stored := cloudGet(data, "words").Items()
	text := cloudGet(data, "transcript")
	if diarized || text.Kind() == orderedjson.Null {
		text = orderedjson.StringValue(googleStoredText(stored))
	}
	sameTimes := googleSameTimes(c, data, stored)
	sameText := wordgroup.SameText(c, text)

	var speaker orderedjson.Value
	if diarized {
		speaker = g.speaker(wordgroup.CueSpeaker(c))
	}
	words := g.words(c, stored, sameTimes, sameText, speaker)
	for i, w := range words {
		if speaker.Kind() == orderedjson.Object && w.Kind() == orderedjson.Object {
			words[i] = orderedjson.Replace(w, speaker)
		}
	}
	out := wordgroup.CueText(c)
	switch s, _ := cloudGet(data, "transcript").AsString(); {
	case sameText:
		out, _ = text.AsString()
	case strings.HasPrefix(s, " "):
		out = " " + out
	}
	return googleEntry{text: out, words: words, sameTimes: sameTimes}
}

// googleSameTimes reports whether the cue has the start of its first stored word and the end of its last stored word.
// Without words, only the stored result end counts.
func googleSameTimes(c *model.Cue, data orderedjson.Value, stored []orderedjson.Value) bool {
	endV := googleEndValue(data)
	if len(stored) > 0 {
		endV = googleWordTime(stored[len(stored)-1], "end")
	}
	sameStart := len(stored) == 0 || wordgroup.SameTime(googleWordTime(stored[0], "start"), time.Second, c.Start)
	return sameStart && wordgroup.SameTime(endV, time.Second, c.End)
}

func googleStoredText(words []orderedjson.Value) string {
	texts := make([]string, len(words))
	for i, w := range words {
		texts[i] = googleWordText(w)
	}
	return strings.Join(texts, " ")
}

// googleEndValue returns the stored result end of a cue, as a duration in seconds, or null.
func googleEndValue(data orderedjson.Value) orderedjson.Value {
	v := cloudGet(data, "resultEndTime")
	if v.Kind() == orderedjson.Null {
		v = cloudGet(data, "resultEndOffset")
	}
	d, ok := googleStoredDuration(v)
	if !ok {
		return orderedjson.Value{}
	}
	return d
}

// speaker returns the speaker members of a voice name: speakerLabel for V2, speakerTag for V1. A V1 name of the
// digits 0 to 9 becomes an integer. A name too large for an integer stays a string.
func (g *googleWriter) speaker(name string) orderedjson.Value {
	switch {
	case name == "":
		return orderedjson.Value{}
	case g.v2:
		return orderedjson.ObjectOf(member("speakerLabel", orderedjson.StringValue(name)))
	}
	if n, err := strconv.ParseInt(name, 10, 64); err == nil && strings.Trim(name, "0123456789") == "" {
		return orderedjson.ObjectOf(member("speakerTag", orderedjson.Int(n)))
	}
	return orderedjson.ObjectOf(member("speakerTag", orderedjson.StringValue(name)))
}

func (g *googleWriter) words(c *model.Cue, stored []orderedjson.Value, sameTimes, sameText bool, speaker orderedjson.Value) []orderedjson.Value {
	confident := false
	source := make([]wordgroup.SourceWord, len(stored))
	for i, w := range stored {
		source[i] = wordgroup.SourceWord{Text: googleWordText(w), Confidence: cloudConfidence(cloudGet(w, "confidence"))}
		source[i].Start = googleStoredTime(googleWordTime(w, "start"))
		source[i].End = googleStoredTime(googleWordTime(w, "end"))
		confident = confident || cloudGet(w, "confidence").Kind() != orderedjson.Null
	}
	words := []orderedjson.Value{}
	choices, ok := wordgroup.Reuse(c, source, sameTimes, sameText)
	if !ok {
		return words
	}
	for _, ch := range choices {
		if ch.New != nil {
			words = append(words, g.newWord(*ch.New, confident, speaker))
			continue
		}
		for _, i := range ch.Source {
			item := stored[i]
			if ch.Word != nil {
				item = googleTyped(item, ch.Word)
			}
			words = append(words, item)
		}
	}
	return words
}

func (g *googleWriter) newWord(n wordgroup.NewWord, confident bool, speaker orderedjson.Value) orderedjson.Value {
	startKey, endKey := "startTime", "endTime"
	if g.v2 {
		startKey, endKey = "startOffset", "endOffset"
	}
	m := []orderedjson.Member{
		member(startKey, orderedjson.StringValue(googleDurationText(n.Start))),
		member(endKey, orderedjson.StringValue(googleDurationText(n.End))),
		member("word", orderedjson.StringValue(n.Text)),
	}
	if confident {
		m = append(m, member("confidence", orderedjson.Float(n.Confidence)))
	}
	return orderedjson.Union(orderedjson.ObjectOf(m...), speaker)
}

// googleTyped gives a reused word the End and the Confidence of its entry in Cue.Words, for each member that the
// word has and that differs.
func googleTyped(item orderedjson.Value, w *model.Word) orderedjson.Value {
	if item.Kind() != orderedjson.Object {
		return item
	}
	for _, key := range []string{"endTime", "endOffset"} {
		if v, ok := item.Get(key); ok {
			if d, isDuration := googleStoredDuration(v); !isDuration || !wordgroup.SameTime(d, time.Second, w.End) {
				item = item.With(key, orderedjson.StringValue(googleDurationText(w.End)))
			}
			break
		}
	}
	if v, ok := item.Get("confidence"); ok && w.Confidence != nil {
		if f, isNumber := v.AsFloat64(); !isNumber || f != *w.Confidence {
			item = item.With("confidence", orderedjson.Float(*w.Confidence))
		}
	}
	return item
}

func googleWordText(w orderedjson.Value) string {
	s, _ := cloudGet(w, "word").AsString()
	return strings.Trim(s, markupcore.PHPTrimSet)
}

// googleWordTime returns the start or the end of a stored word as seconds, or null. edge is "start" or "end".
func googleWordTime(w orderedjson.Value, edge string) orderedjson.Value {
	v := cloudGet(w, edge+"Time")
	if v.Kind() == orderedjson.Null {
		v = cloudGet(w, edge+"Offset")
	}
	d, ok := googleStoredDuration(v)
	if !ok {
		return orderedjson.Value{}
	}
	return d
}

// googleStoredDuration reads a stored duration as a writer does. A plain number is no duration, and a string has no
// minus sign.
func googleStoredDuration(v orderedjson.Value) (orderedjson.Value, bool) {
	if v.Kind() == orderedjson.Number {
		return orderedjson.Value{}, false
	}
	return googleDuration(v, false)
}

// googleStoredTime returns a duration in seconds as whole milliseconds, or nil.
func googleStoredTime(v orderedjson.Value) *time.Duration {
	if v.Kind() == orderedjson.Null {
		return nil
	}
	d, ok := v.Duration(time.Second)
	if !ok {
		return nil
	}
	return &d
}
