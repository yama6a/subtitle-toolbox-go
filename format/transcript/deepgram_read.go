package transcript

import (
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

const deepgramKey = string(model.Deepgram)

// ReadDeepgram reads the JSON response of the Deepgram pre-recorded audio API from r. It makes one cue per utterance,
// else one per paragraph sentence, else cues grouped from the words. A field error names the path of the field, such
// as results.channels[0].alternatives[0].words[3].end.
func ReadDeepgram(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	counted := &countingReader{r: r}
	st, root, ok, err := readRoot(counted, model.Deepgram, common)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	results, _ := root.Get("results")
	channels := cloudGet(results, "channels")
	if results.Kind() != orderedjson.Object || channels.Kind() != orderedjson.Array {
		return nil, nil, &model.ParseError{Format: model.Deepgram, Msg: `The JSON has no "results.channels" list.`}
	}
	rd := &deepgramReader{st: st, size: counted.n, cues: wordgroup.NewReader(st, wordgroup.Options{
		WordTimestamps: opts.WordTimestamps, SpeakerVoices: opts.SpeakerVoices,
	}, deepgramKey)}
	cues, err := rd.cuesOf(results, channels.Items())
	if err != nil {
		return nil, nil, err
	}
	reduced := make([]orderedjson.Value, len(channels.Items()))
	for i, c := range channels.Items() {
		reduced[i] = deepgramChannelData(c)
	}
	var language orderedjson.Value
	if len(reduced) > 0 {
		language = cloudGet(reduced[0], "detected_language")
	}
	results = results.With("channels", orderedjson.ArrayOf(reduced...))
	s, err := rd.cues.Transcript(cues, language, cloudData(wordgroup.WithResults(root, results, "utterances")))
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

// cuesOf makes one cue for each utterance, or reads the channels when there are no utterances.
func (rd *deepgramReader) cuesOf(results orderedjson.Value, channels []orderedjson.Value) ([]*model.Cue, error) {
	if utterances := cloudGet(results, "utterances"); utterances.Kind() == orderedjson.Array && len(utterances.Items()) > 0 {
		return rd.cues.Utterances(utterances.Items(), "results.utterances", "transcript", time.Second, rd.words)
	}
	return rd.channels(channels)
}

type deepgramReader struct {
	st   *parsing.State
	cues *wordgroup.Reader
	// size is the length of the file in bytes.
	size int
	// copied is the size in bytes of the paragraph data that the cues hold so far, and limit is its upper bound.
	copied, limit int
}

// paragraphCopyFactor and paragraphCopyBase give the upper bound of the paragraph data that all cues of a file hold:
// the factor times the size of the file, plus the base. A real file has 3 or 4 paragraph members, so it stays far below.
const (
	paragraphCopyFactor = 4
	paragraphCopyBase   = 1 << 20
)

// deepgramChannelData returns the members of a channel and of its first alternative, without the transcript, the
// words and the paragraphs.
func deepgramChannelData(channel orderedjson.Value) orderedjson.Value {
	alt := cloudGet(channel, "alternatives").Items()
	if channel.Kind() != orderedjson.Object || len(alt) == 0 || alt[0].Kind() != orderedjson.Object {
		return channel
	}
	first := orderedjson.ObjectOf(member("alternatives", orderedjson.ArrayOf(alt[0].Without("transcript", "words", "paragraphs"))))
	return orderedjson.Union(first, channel)
}

func (rd *deepgramReader) words(items []orderedjson.Value, path string) ([]wordgroup.Word, error) {
	return rd.cues.WordList(items, path, deepgramWord)
}

func deepgramWord(item orderedjson.Value, path string) (wordgroup.Word, error) {
	key := "word"
	if cloudGet(item, "punctuated_word").Kind() != orderedjson.Null {
		key = "punctuated_word"
	}
	text, err := wordgroup.Text(item, key, path)
	if err != nil {
		return wordgroup.Word{}, err
	}
	start, err := wordgroup.Seconds(cloudGet(item, "start"), path+".start", time.Second)
	if err != nil {
		return wordgroup.Word{}, err
	}
	end, err := wordgroup.Seconds(cloudGet(item, "end"), path+".end", time.Second)
	if err != nil {
		return wordgroup.Word{}, err
	}
	w := wordgroup.Word{
		Text: strings.Trim(text, markupcore.PHPTrimSet), Start: start, Timed: true, End: end, HasEnd: true,
		Confidence: cloudConfidence(cloudGet(item, "confidence")),
	}
	w.Speaker, w.HasSpeaker = wordgroup.Speaker(cloudGet(item, "speaker"))
	return w, nil
}

func (rd *deepgramReader) channels(channels []orderedjson.Value) ([]*model.Cue, error) {
	var cues []*model.Cue
	for c, channel := range channels {
		alt := cloudGet(channel, "alternatives").Items()
		if len(alt) == 0 || alt[0].Kind() != orderedjson.Object {
			continue
		}
		path := "results.channels[" + strconv.Itoa(c) + "].alternatives[0]"
		words, err := rd.words(cloudGet(alt[0], "words").Items(), path+".words")
		if err != nil {
			return nil, err
		}
		paragraphs := cloudGet(cloudGet(alt[0], "paragraphs"), "paragraphs").Items()
		var got []*model.Cue
		if len(paragraphs) == 0 {
			extra := model.NewData(model.Field{Key: "channel", Value: int64(c)})
			got, err = rd.cues.CuesFromWords(words, "words", extra)
		} else {
			got, err = rd.paragraphCues(paragraphs, words, path, c)
		}
		if err != nil {
			return nil, err
		}
		cues = append(cues, got...)
	}
	return cues, nil
}

// paragraphCues makes a cue for each sentence of the paragraphs, with the words between its start and its end.
func (rd *deepgramReader) paragraphCues(paragraphs []orderedjson.Value, words []wordgroup.Word, path string, channel int) ([]*model.Cue, error) {
	var cues []*model.Cue
	index := 0
	if rd.limit == 0 {
		rd.limit = paragraphCopyFactor*rd.size + paragraphCopyBase
	}
	for p, paragraph := range paragraphs {
		speaker, hasSpeaker := wordgroup.Speaker(cloudGet(paragraph, "speaker"))
		rest := paragraph.Without("sentences")
		size := len(orderedjson.Block(rest))
		for s, sentence := range cloudGet(paragraph, "sentences").Items() {
			sentencePath := path + ".paragraphs.paragraphs[" + strconv.Itoa(p) + "].sentences[" + strconv.Itoa(s) + "]"
			start, end, text, err := deepgramSentenceFields(sentence, sentencePath)
			if err != nil {
				if err = cloudFail(rd.st, err, s, sentence); err != nil {
					return nil, err
				}
				continue
			}
			if rd.copied += size; rd.copied > rd.limit {
				return nil, &model.ParseError{Format: model.Deepgram, Msg: "The sentences copy more than " + strconv.Itoa(rd.limit) +
					" bytes of paragraph data into the cues. Each cue of a sentence holds a copy of its paragraph."}
			}
			between := wordgroup.Between(words, &index, start, end)
			data := deepgramSentenceData(sentence, channel, paragraph, rest, between, hasSpeaker)
			c, err := rd.cues.Cue(start, end, text, between, speaker, cloudData(data), s, sentence)
			if err != nil {
				return nil, err
			}
			if c != nil {
				cues = append(cues, c)
			}
		}
	}
	return cues, nil
}

// deepgramSentenceData returns the cue data of a sentence: its members, then the channel, the speaker of the paragraph,
// the paragraph without its sentences, and the raw words of the sentence.
func deepgramSentenceData(sentence orderedjson.Value, channel int, paragraph, rest orderedjson.Value, words []wordgroup.Word, hasSpeaker bool) orderedjson.Value {
	data := orderedjson.Union(sentence, orderedjson.ObjectOf(member("channel", orderedjson.Int(int64(channel)))))
	if hasSpeaker {
		data = orderedjson.Union(data, orderedjson.ObjectOf(member("speaker", cloudGet(paragraph, "speaker"))))
	}
	raw := make([]orderedjson.Value, 0, len(words))
	for _, w := range words {
		raw = append(raw, w.Source...)
	}
	data = orderedjson.Union(data, orderedjson.ObjectOf(member("paragraph", rest)))
	return orderedjson.Union(data, orderedjson.ObjectOf(member("words", orderedjson.ArrayOf(raw...))))
}

func deepgramSentenceFields(sentence orderedjson.Value, path string) (start, end time.Duration, text string, err error) {
	if start, err = wordgroup.Seconds(cloudGet(sentence, "start"), path+".start", time.Second); err != nil {
		return 0, 0, "", err
	}
	if end, err = wordgroup.Seconds(cloudGet(sentence, "end"), path+".end", time.Second); err != nil {
		return 0, 0, "", err
	}
	text, err = wordgroup.Text(sentence, "text", path)
	return start, end, text, err
}

// countingReader counts the bytes that it reads.
type countingReader struct {
	r io.Reader
	n int
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += n
	return n, err
}
