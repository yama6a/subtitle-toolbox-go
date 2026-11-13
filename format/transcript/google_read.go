package transcript

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const googleKey = string(model.GoogleSpeech)

// ReadGoogleSpeech reads the JSON response of Google Cloud Speech-to-Text V1 and V2 from r, also inside a long-running
// operation. It makes one cue per result. With speaker diarization it groups the words of the last result into cues.
// A field error names the path of the field, such as results[0].alternatives[0].words[3].startTime.
func ReadGoogleSpeech(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	st, root, ok, err := readRoot(r, model.GoogleSpeech, common)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	operation, response := orderedjson.ObjectOf(), root
	if v := cloudGet(root, "response"); v.Kind() == orderedjson.Object || v.Kind() == orderedjson.Array {
		operation, response = root.Without("response"), v
	}
	list := cloudGet(response, "results")
	if list.Kind() != orderedjson.Array {
		return nil, nil, &model.ParseError{Format: model.GoogleSpeech, Msg: `The JSON has no "results" list.`}
	}
	results := list.Items()
	rd := &googleReader{st: st, cues: wordgroup.NewReader(st, wordgroup.Options{
		WordTimestamps: opts.WordTimestamps, SpeakerVoices: opts.SpeakerVoices,
	}, googleKey)}

	cues, fileData, err := rd.cuesOf(results, operation)
	if err != nil {
		return nil, nil, err
	}
	fileData = orderedjson.Union(fileData, response.Without("results"))
	s, err := rd.cues.Transcript(cues, googleLanguage(results), cloudData(fileData))
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

// cuesOf reads the cues of the results. It returns the file data so far: the operation members, then the diarized result.
func (rd *googleReader) cuesOf(results []orderedjson.Value, operation orderedjson.Value) ([]*model.Cue, orderedjson.Value, error) {
	last := len(results) - 1
	if last < 0 || !googleDiarized(results[last]) {
		cues, err := rd.results(results)
		return cues, operation, err
	}
	words, err := rd.words(googleWords(results[last]), "results["+strconv.Itoa(last)+"].alternatives[0].words")
	if err != nil {
		return nil, operation, err
	}
	cues, err := rd.cues.CuesFromWords(words, "words", nil)
	kept := orderedjson.ObjectOf(member("results", orderedjson.ArrayOf(googleWithoutWords(results[last]))))
	return cues, orderedjson.Union(operation, kept), err
}

type googleReader struct {
	st   *parsing.State
	cues *wordgroup.Reader
}

// googleWords returns the words of the first alternative of a result.
func googleWords(result orderedjson.Value) []orderedjson.Value {
	alt := cloudGet(result, "alternatives").Items()
	if len(alt) == 0 {
		return nil
	}
	return cloudGet(alt[0], "words").Items()
}

// googleDiarized reports whether a word of the result has a speakerLabel, or a speakerTag that PHP empty() does not
// count as empty. Each diarized result repeats the words of the results before it, so the last result holds all words.
func googleDiarized(result orderedjson.Value) bool {
	for _, w := range googleWords(result) {
		if cloudGet(w, "speakerLabel").Kind() != orderedjson.Null || !googleFalsy(cloudGet(w, "speakerTag")) {
			return true
		}
	}
	return false
}

// googleFalsy reports whether PHP empty() is true for v: null, false, 0, "", "0" and an empty array.
func googleFalsy(v orderedjson.Value) bool {
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

// googleWithoutWords returns the members of a result and of its first alternative, without the transcript and the words.
func googleWithoutWords(result orderedjson.Value) orderedjson.Value {
	alt := cloudGet(result, "alternatives").Items()
	if result.Kind() != orderedjson.Object || len(alt) == 0 || alt[0].Kind() != orderedjson.Object {
		return result
	}
	first := orderedjson.ObjectOf(member("alternatives", orderedjson.ArrayOf(alt[0].Without("transcript", "words"))))
	return orderedjson.Union(first, result)
}

// googleLanguage returns the first languageCode of the results that is a string and not empty.
func googleLanguage(results []orderedjson.Value) orderedjson.Value {
	for _, r := range results {
		if s, ok := cloudGet(r, "languageCode").AsString(); ok && s != "" {
			return orderedjson.StringValue(s)
		}
	}
	return orderedjson.Value{}
}

func (rd *googleReader) words(items []orderedjson.Value, path string) ([]wordgroup.Word, error) {
	return rd.cues.WordList(items, path, googleWord)
}

// googleEither returns the first member that is not null, then converts a protobuf Duration to seconds.
func googleEither(item orderedjson.Value, first, second string) orderedjson.Value {
	v := cloudGet(item, first)
	if v.Kind() == orderedjson.Null {
		v = cloudGet(item, second)
	}
	if d, ok := googleDuration(v, true); ok {
		return d
	}
	return v
}

func googleWord(item orderedjson.Value, path string) (wordgroup.Word, error) {
	text, err := wordgroup.Text(item, "word", path)
	if err != nil {
		return wordgroup.Word{}, err
	}
	start, err := wordgroup.Seconds(googleEither(item, "startTime", "startOffset"), path+".startTime", time.Second)
	if err != nil {
		return wordgroup.Word{}, err
	}
	end, err := wordgroup.Seconds(googleEither(item, "endTime", "endOffset"), path+".endTime", time.Second)
	if err != nil {
		return wordgroup.Word{}, err
	}
	w := wordgroup.Word{
		Text: strings.Trim(text, markupcore.PHPTrimSet), Start: start, Timed: true, End: end, HasEnd: true,
		Confidence: cloudConfidence(cloudGet(item, "confidence")),
	}
	label := cloudGet(item, "speakerLabel")
	if label.Kind() == orderedjson.Null {
		if tag := cloudGet(item, "speakerTag"); tag.Kind() != orderedjson.Number || !googleIsZero(tag) {
			label = tag
		}
	}
	w.Speaker, w.HasSpeaker = wordgroup.Speaker(label)
	return w, nil
}

// googleIsZero reports whether v is the integer 0.
func googleIsZero(v orderedjson.Value) bool {
	i, ok := v.AsInt64()
	return ok && i == 0
}

// results makes one cue for each result. A result without words starts where the result before it ended.
func (rd *googleReader) results(results []orderedjson.Value) ([]*model.Cue, error) {
	var cues []*model.Cue
	var previousEnd time.Duration
	for i, result := range results {
		c, resultEnd, err := rd.result(i, result, previousEnd)
		if err != nil {
			return nil, err
		}
		if c.failed {
			continue
		}
		previousEnd = resultEnd
		if c.cue != nil {
			cues = append(cues, c.cue)
		}
	}
	return cues, nil
}

type googleCue struct {
	cue    *model.Cue
	failed bool
}

func (rd *googleReader) result(i int, result orderedjson.Value, previousEnd time.Duration) (googleCue, time.Duration, error) {
	path := "results[" + strconv.Itoa(i) + "]"
	alternative, words, start, end, text, resultEnd, err := rd.fields(path, result, previousEnd)
	if err != nil {
		return googleCue{failed: true}, 0, cloudFail(rd.st, err, i, result)
	}
	data := orderedjson.Union(result.Without("alternatives"), alternative)
	c, err := rd.cues.Cue(start, end, text, words, "", cloudData(data), i, result)
	return googleCue{cue: c}, resultEnd, err
}

func (rd *googleReader) fields(path string, result orderedjson.Value, previousEnd time.Duration) (
	alternative orderedjson.Value, words []wordgroup.Word, start, end time.Duration, text string, resultEnd time.Duration, err error,
) {
	end0 := googleEither(result, "resultEndTime", "resultEndOffset")
	if alternative, err = googleFirstAlternative(result, path); err != nil {
		return
	}
	if words, err = rd.words(cloudGet(alternative, "words").Items(), path+".alternatives[0].words"); err != nil {
		return
	}
	if len(words) == 0 {
		start = previousEnd
		if end, err = wordgroup.Seconds(end0, path+".resultEndTime", time.Second); err != nil {
			return
		}
	} else {
		start, end = words[0].Start, words[len(words)-1].End
	}
	if alternative.Kind() == orderedjson.Object {
		if text, err = wordgroup.Text(alternative, "transcript", path+".alternatives[0]"); err != nil {
			return
		}
	}
	resultEnd, err = googleResultEnd(end0, path, end)
	return
}

// googleResultEnd returns the end of the result when it is a time, else the end of the cue.
func googleResultEnd(v orderedjson.Value, path string, end time.Duration) (time.Duration, error) {
	d, err := wordgroup.Seconds(v, path+".resultEndTime", time.Second)
	if err == nil {
		return d, nil
	}
	var perr *model.ParseError
	if errors.As(err, &perr) && strings.HasSuffix(perr.Msg, "must be a time.") {
		return end, nil
	}
	return 0, err
}

// googleFirstAlternative returns the first alternative of a result, or null for a result without alternatives.
func googleFirstAlternative(result orderedjson.Value, path string) (orderedjson.Value, error) {
	alternatives := cloudGet(result, "alternatives")
	switch alternatives.Kind() {
	case orderedjson.Null:
		return orderedjson.Value{}, nil
	case orderedjson.Array:
		if items := alternatives.Items(); len(items) == 0 {
			return orderedjson.Value{}, nil
		} else if items[0].Kind() == orderedjson.Object {
			return items[0], nil
		}
	case orderedjson.Object:
		if len(alternatives.Members()) == 0 {
			return orderedjson.Value{}, nil
		}
	case orderedjson.Bool, orderedjson.Number, orderedjson.String:
	}
	return orderedjson.Value{}, &model.ParseError{Msg: "The field " + path + ".alternatives must be a list of objects."}
}
