package transcript

import (
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// WriteAWSTranscribe writes s to w as the JSON of an Amazon Transcribe batch job. It writes the transcript, one item
// for each word and punctuation mark, and the speaker segments when a cue has a speaker. It writes one audio segment
// for each cue when the source had them, or when a cue has no words, because the items alone cannot hold such a cue.
// A value that a cue did not change stays as the reader stored it.
func WriteAWSTranscribe(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	return cloudWrite(w, s, model.AWSTranscribe, "Amazon Transcribe JSON", common, awsDocument)
}

func awsDocument(s *model.Subtitle) (orderedjson.Value, error) {
	file, err := cloudFileData(s, model.AWSTranscribe)
	if err != nil {
		return file, err
	}
	results, _ := file.Get("results")
	if results.Kind() != orderedjson.Object {
		results = orderedjson.ObjectOf()
	}
	if lang, ok := s.Metadata(model.MetadataLanguage); ok {
		if cloudGet(results, "language_code").Kind() == orderedjson.Null {
			results = results.With("language_code", orderedjson.StringValue(lang))
		}
	}

	var items, segments []orderedjson.Value
	var texts []string
	hasSegments := false
	for _, c := range s.All() {
		seg, cueItems := awsSegment(&c, len(items))
		items = append(items, cueItems...)
		text, _ := cloudGet(seg, "transcript").AsString()
		texts = append(texts, text)
		id := cloudMember("id", orderedjson.Int(int64(len(segments))))
		segments = append(segments, orderedjson.Union(orderedjson.ObjectOf(id), seg))
		stored := cloudGet(cloudCueData(&c, model.AWSTranscribe), "transcript")
		hasSegments = hasSegments || stored.Kind() != orderedjson.Null || len(cueItems) == 0
	}

	out := orderedjson.ObjectOf(cloudMember("transcripts", orderedjson.ArrayOf(orderedjson.ObjectOf(
		cloudMember("transcript", orderedjson.StringValue(strings.Join(texts, " "))),
	))))
	if runs := awsSpeakerRuns(items); len(runs) > 0 {
		labels := orderedjson.ObjectOf(cloudMember("speaker_labels", awsSpeakerLabels(cloudGet(results, "speaker_labels"), runs)))
		out = orderedjson.Union(out, labels)
	}
	out = orderedjson.Union(out, orderedjson.ObjectOf(cloudMember("items", orderedjson.ArrayOf(items...))))
	if hasSegments {
		out = orderedjson.Union(out, orderedjson.ObjectOf(cloudMember("audio_segments", orderedjson.ArrayOf(segments...))))
	}
	out = orderedjson.Union(out, results.Without("speaker_labels"))
	return orderedjson.Union(orderedjson.ObjectOf(cloudMember("results", out)), file), nil
}

// awsSpeakerLabels returns the stored speaker_labels object with the number of speakers and the runs.
func awsSpeakerLabels(stored orderedjson.Value, runs []orderedjson.Value) orderedjson.Value {
	if stored.Kind() != orderedjson.Object {
		stored = orderedjson.ObjectOf()
	}
	return orderedjson.Replace(stored, orderedjson.ObjectOf(
		cloudMember("speakers", orderedjson.Int(int64(awsDistinctLabels(runs)))),
		cloudMember("segments", orderedjson.ArrayOf(runs...)),
	))
}

// awsSegment returns the audio segment of the cue without its id, and the items of the cue with the ids from firstID on.
func awsSegment(c *model.Cue, firstID int) (orderedjson.Value, []orderedjson.Value) {
	data := cloudCueData(c, model.AWSTranscribe)
	groups := awsWordGroups(cloudGet(data, "items").Items())
	startV, endV, textV := awsSourceValues(data, groups)
	sameStart := wordgroup.SameTime(startV, time.Second, c.Start)
	sameEnd := wordgroup.SameTime(endV, time.Second, c.End)
	sameText := wordgroup.SameText(c, textV)
	speaker := wordgroup.CueSpeaker(c)
	items, ids := awsNumberItems(awsReuse(c, groups, sameStart && sameEnd, sameText, speaker), data, speaker, firstID)

	transcript := textV
	if !sameText {
		transcript = orderedjson.StringValue(wordgroup.CueText(c))
	}
	if !sameStart {
		startV = orderedjson.StringValue(awsTime(c.Start))
	}
	if !sameEnd {
		endV = orderedjson.StringValue(awsTime(c.End))
	}
	with := []orderedjson.Value{orderedjson.ObjectOf(
		cloudMember("transcript", transcript), cloudMember("start_time", startV), cloudMember("end_time", endV),
	)}
	if speaker != "" {
		with = append(with, orderedjson.ObjectOf(cloudMember("speaker_label", orderedjson.StringValue(speaker))))
	}
	with = append(with, orderedjson.ObjectOf(cloudMember("items", orderedjson.ArrayOf(ids...))))
	return orderedjson.Replace(data.Without("id"), with...), items
}

// awsSourceValues returns the stored start, end and text of the cue. A grouped cue has no stored text, so the values
// come from its words.
func awsSourceValues(data orderedjson.Value, groups [][]orderedjson.Value) (start, end, text orderedjson.Value) {
	if text = cloudGet(data, "transcript"); text.Kind() != orderedjson.Null {
		return cloudGet(data, "start_time"), cloudGet(data, "end_time"), text
	}
	if len(groups) > 0 {
		start = awsFirstMember(groups[0], "start_time")
	}
	texts := make([]string, len(groups))
	for i, g := range groups {
		texts[i] = awsWordText(g)
	}
	return start, awsLastEnd(groups), orderedjson.StringValue(strings.Join(texts, " "))
}

// awsReuse returns the items of the cue before the ids and the speaker are set.
func awsReuse(c *model.Cue, groups [][]orderedjson.Value, sameTimes, sameText bool, speaker string) []orderedjson.Value {
	source := make([]wordgroup.SourceWord, len(groups))
	for i, g := range groups {
		source[i] = awsSourceWord(g)
	}
	choices, ok := wordgroup.Reuse(c, source, sameTimes, sameText)
	if !ok {
		return nil
	}
	var items []orderedjson.Value
	for _, ch := range choices {
		if ch.New != nil {
			items = append(items, awsNewItems(*ch.New, speaker)...)
			continue
		}
		for _, i := range ch.Source {
			if ch.Word != nil {
				items = append(items, awsTyped(groups[i], ch.Word)...)
				continue
			}
			items = append(items, groups[i]...)
		}
	}
	return items
}

// awsNumberItems gives each item its id, and the cue speaker when the item differs from the stored segment label or
// has none. It returns the items and their ids.
func awsNumberItems(items []orderedjson.Value, data orderedjson.Value, speaker string, firstID int) (out, ids []orderedjson.Value) {
	storedLabel, _ := cloudGet(data, "speaker_label").AsString()
	sameLabel := storedLabel == speaker && cloudGet(data, "speaker_label").Kind() == orderedjson.String
	out = make([]orderedjson.Value, len(items))
	ids = make([]orderedjson.Value, len(items))
	for k, item := range items {
		if item.Kind() != orderedjson.Object {
			item = orderedjson.ObjectOf()
		}
		if speaker != "" && (!sameLabel || cloudGet(item, "speaker_label").Kind() == orderedjson.Null) {
			item = item.With("speaker_label", orderedjson.StringValue(speaker))
		}
		id := orderedjson.Int(int64(firstID + k))
		first := orderedjson.ObjectOf(cloudMember("id", id))
		out[k], ids[k] = orderedjson.Replace(first, item, first), id
	}
	return out, ids
}

// awsWordGroups splits the stored items into words, as the reader joins them: each pronunciation item with the
// punctuation items after it. Punctuation before the first word joins the first word.
func awsWordGroups(items []orderedjson.Value) [][]orderedjson.Value {
	var words [][]orderedjson.Value
	var leading []orderedjson.Value
	for _, item := range items {
		switch {
		case !awsIsPunctuation(item):
			words = append(words, append(leading, item))
			leading = leading[:0:0]
		case len(words) == 0:
			leading = append(leading, item)
		default:
			words[len(words)-1] = append(words[len(words)-1], item)
		}
	}
	if len(leading) > 0 {
		words = append(words, leading)
	}
	return words
}

// awsFirstMember returns the first member key of the items that has it, also with a null value.
func awsFirstMember(group []orderedjson.Value, key string) orderedjson.Value {
	for _, item := range group {
		if v, ok := item.Get(key); ok {
			return v
		}
	}
	return orderedjson.Value{}
}

func awsLastMember(group []orderedjson.Value, key string) orderedjson.Value {
	for i := len(group) - 1; i >= 0; i-- {
		if v, ok := group[i].Get(key); ok {
			return v
		}
	}
	return orderedjson.Value{}
}

// awsLastEnd returns the last end_time of all items that PHP array_filter keeps.
func awsLastEnd(groups [][]orderedjson.Value) orderedjson.Value {
	var last orderedjson.Value
	for _, g := range groups {
		for _, item := range g {
			if v, ok := item.Get("end_time"); ok && !phpFalsy(v) {
				last = v
			}
		}
	}
	return last
}

func phpFalsy(v orderedjson.Value) bool {
	switch v.Kind() {
	case orderedjson.Null:
		return true
	case orderedjson.Bool:
		b, _ := v.AsBool()
		return !b
	case orderedjson.Number:
		f, _ := v.AsFloat64()
		return f == 0
	case orderedjson.String:
		s, _ := v.AsString()
		return s == "" || s == "0"
	case orderedjson.Array:
		return len(v.Items()) == 0
	case orderedjson.Object:
		return len(v.Members()) == 0
	}
	return false
}

func awsContent(item orderedjson.Value) string {
	alt, _ := item.Get("alternatives")
	if len(alt.Items()) == 0 {
		return ""
	}
	v, _ := alt.Items()[0].Get("content")
	s, _ := v.AsString()
	return s
}

func awsWordText(group []orderedjson.Value) string {
	var b strings.Builder
	for _, item := range group {
		b.WriteString(awsContent(item))
	}
	return b.String()
}

// awsPronunciation returns the index of the first item that is not punctuation, or -1.
func awsPronunciation(group []orderedjson.Value) int {
	for i, item := range group {
		if !awsIsPunctuation(item) {
			return i
		}
	}
	return -1
}

func awsSourceWord(group []orderedjson.Value) wordgroup.SourceWord {
	w := wordgroup.SourceWord{Text: strings.Trim(awsWordText(group), markupcore.PHPTrimSet)}
	w.Start = awsStoredTime(awsFirstMember(group, "start_time"))
	w.End = awsStoredTime(awsLastMember(group, "end_time"))
	if i := awsPronunciation(group); i >= 0 {
		alt, _ := group[i].Get("alternatives")
		if len(alt.Items()) > 0 {
			conf, _ := alt.Items()[0].Get("confidence")
			w.Confidence = awsConfidence(conf)
		}
	}
	return w
}

// awsStoredTime returns a number or a numeric string as whole milliseconds, or nil.
func awsStoredTime(v orderedjson.Value) *time.Duration {
	if k := v.Kind(); k != orderedjson.Number && k != orderedjson.String {
		return nil
	}
	d, ok := v.Duration(time.Second)
	if !ok {
		return nil
	}
	return &d
}

// awsTyped returns the items of a reused word with the End and the Confidence of its entry in Cue.Words, for each
// value that the pronunciation item has and that differs.
func awsTyped(group []orderedjson.Value, w *model.Word) []orderedjson.Value {
	i := awsPronunciation(group)
	if i < 0 || group[i].Kind() != orderedjson.Object {
		return group
	}
	item := group[i]
	if v, ok := item.Get("end_time"); ok && !wordgroup.SameTime(v, time.Second, w.End) {
		item = item.With("end_time", orderedjson.StringValue(awsTime(w.End)))
	}
	if alt, _ := item.Get("alternatives"); w.Confidence != nil && len(alt.Items()) > 0 {
		first := alt.Items()[0]
		if v, ok := first.Get("confidence"); ok {
			if stored := awsConfidence(v); stored == nil || *stored != *w.Confidence {
				alts := append([]orderedjson.Value{first.With("confidence", orderedjson.StringValue(awsConfidenceText(*w.Confidence)))}, alt.Items()[1:]...)
				item = item.With("alternatives", orderedjson.ArrayOf(alts...))
			}
		}
	}
	out := append([]orderedjson.Value(nil), group...)
	out[i] = item
	return out
}

// awsNewItems returns a pronunciation item for a word, and a punctuation item for the punctuation at its end.
func awsNewItems(n wordgroup.NewWord, speaker string) []orderedjson.Value {
	body := n.Text
	for body != "" {
		r, size := utf8.DecodeLastRuneInString(body)
		if !unicode.IsPunct(r) {
			break
		}
		body = body[:len(body)-size]
	}
	punct := n.Text[len(body):]
	label := func() []orderedjson.Member {
		if speaker == "" {
			return nil
		}
		return []orderedjson.Member{cloudMember("speaker_label", orderedjson.StringValue(speaker))}
	}
	alternative := func(confidence, content string) orderedjson.Member {
		return cloudMember("alternatives", orderedjson.ArrayOf(orderedjson.ObjectOf(
			cloudMember("confidence", orderedjson.StringValue(confidence)), cloudMember("content", orderedjson.StringValue(content)),
		)))
	}
	var items []orderedjson.Value
	if body != "" || punct == "" {
		m := make([]orderedjson.Member, 0, 5)
		m = append(m, cloudMember("start_time", orderedjson.StringValue(awsTime(n.Start))))
		m = append(m, label()...)
		m = append(m, cloudMember("end_time", orderedjson.StringValue(awsTime(n.End))),
			alternative(awsConfidenceText(n.Confidence), body), cloudMember("type", orderedjson.StringValue("pronunciation")))
		items = append(items, orderedjson.ObjectOf(m...))
	}
	if punct != "" {
		m := append(label(), alternative("0.0", punct), cloudMember("type", orderedjson.StringValue("punctuation")))
		items = append(items, orderedjson.ObjectOf(m...))
	}
	return items
}

// awsSpeakerRuns returns the speaker turns of the items: the runs of items with a start, an end and the same speaker.
func awsSpeakerRuns(items []orderedjson.Value) []orderedjson.Value {
	type run struct {
		start, label, end orderedjson.Value
		entries           []orderedjson.Value
	}
	var runs []run
	for _, item := range items {
		start, _ := item.Get("start_time")
		end, _ := item.Get("end_time")
		label, _ := item.Get("speaker_label")
		if start.Kind() == orderedjson.Null || end.Kind() == orderedjson.Null || label.Kind() == orderedjson.Null {
			continue
		}
		entry := orderedjson.ObjectOf(cloudMember("start_time", start), cloudMember("speaker_label", label), cloudMember("end_time", end))
		if n := len(runs); n > 0 && orderedjson.Equal(runs[n-1].label, label) {
			runs[n-1].end = end
			runs[n-1].entries = append(runs[n-1].entries, entry)
			continue
		}
		runs = append(runs, run{start: start, label: label, end: end, entries: []orderedjson.Value{entry}})
	}
	out := make([]orderedjson.Value, len(runs))
	for i, r := range runs {
		out[i] = orderedjson.ObjectOf(
			cloudMember("start_time", r.start), cloudMember("speaker_label", r.label), cloudMember("end_time", r.end),
			cloudMember("items", orderedjson.ArrayOf(r.entries...)),
		)
	}
	return out
}

func awsDistinctLabels(runs []orderedjson.Value) int {
	seen := map[string]bool{}
	for _, r := range runs {
		label, _ := r.Get("speaker_label")
		key, ok := label.AsString()
		if !ok {
			key = orderedjson.Block(label)
		}
		seen[key] = true
	}
	return len(seen)
}

// awsTime returns d in seconds with at most 3 decimals and at least one, as Amazon Transcribe writes it:
// 2 s is "2.0", 520 ms is "0.52".
func awsTime(d time.Duration) string {
	ms := max(0, d).Round(time.Millisecond).Milliseconds()
	text := strings.TrimRight(fmt.Sprintf("%d.%03d", ms/1000, ms%1000), "0")
	if strings.HasSuffix(text, ".") {
		text += "0"
	}
	return text
}

// awsConfidenceText returns the shortest decimal of f with at least one decimal, such as "1.0" and "0.9989".
func awsConfidenceText(f float64) string {
	text, _ := orderedjson.Float(f).Text()
	return text
}
