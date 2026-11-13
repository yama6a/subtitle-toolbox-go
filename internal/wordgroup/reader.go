package wordgroup

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// Reader holds the state of one read: the parse state, the options and the format data key.
type Reader struct {
	st   *parsing.State
	opts Options
	key  string
}

// NewReader returns a reader that stores the format data of cues and file under key.
func NewReader(st *parsing.State, opts Options, key string) *Reader {
	return &Reader{st: st, opts: opts, key: key}
}

// Cue builds one cue, or returns nil for an empty text. block and source name the item in a warning.
// A cue that ends before it starts is an error in strict mode. Lenient mode swaps the times or skips the cue.
func (r *Reader) Cue(start, end time.Duration, text string, words []Word, speaker string,
	data *model.Data, block int, source orderedjson.Value,
) (*model.Cue, error) {
	text = strings.Trim(text, markupcore.PHPTrimSet)
	if text == "" {
		return nil, nil //nolint:nilnil // a nil cue is an empty text, not an error
	}
	lines := []string{orderedjson.Block(source)}
	start, end, err := r.st.OrderedTimes(start, end, model.NoLine, block, lines, true)
	if err != nil {
		var perr *model.ParseError
		if !errors.As(err, &perr) {
			return nil, err
		}
		return nil, r.st.Fail(perr, model.NoLine, block, lines)
	}
	c := model.Cue{Start: start, End: end}
	c.SetText(markupcore.EscapeText(text))
	if r.opts.WordTimestamps {
		c.Words = CueWords(c.PlainText(), words, c.End)
	}
	if name := strings.Trim(speaker, markupcore.PHPTrimSet); r.opts.SpeakerVoices && name != "" {
		c.Speaker = name
	}
	if err := c.SetFormatData(r.key, data); err != nil {
		return nil, fmt.Errorf("%s cue data: %w", r.key, err)
	}
	return &c, nil
}

// CuesFromWords groups words with Group and builds one cue per group. The format data of a cue holds the members of
// extra, then the source values of the group under wordsKey, unless extra has wordsKey.
func (r *Reader) CuesFromWords(words []Word, wordsKey string, extra *model.Data) ([]*model.Cue, error) {
	var cues []*model.Cue
	for i, group := range Group(words) {
		var sources []orderedjson.Value
		texts := make([]string, len(group))
		for k, w := range group {
			sources = append(sources, w.Source...)
			texts[k] = w.Text
		}
		data := model.NewData()
		for _, k := range extra.Keys() {
			v, _ := extra.Get(k)
			data.Set(k, v)
		}
		if _, ok := data.Get(wordsKey); !ok {
			list := make([]any, len(sources))
			for k, s := range sources {
				list[k] = jsondata.ToData(s)
			}
			data.Set(wordsKey, list)
		}
		last := group[len(group)-1]
		c, err := r.Cue(group[0].Start, last.end(), strings.Join(texts, " "), group, group[0].Speaker,
			data, i, orderedjson.ArrayOf(sources...))
		if err != nil {
			return nil, err
		}
		if c != nil {
			cues = append(cues, c)
		}
	}
	return cues, nil
}

// WordList reads each item with read. A failure goes to State.Fail, so strict mode ends the read and lenient mode
// skips the word with a warning.
func (r *Reader) WordList(items []orderedjson.Value, path string,
	read func(item orderedjson.Value, path string) (Word, error),
) ([]Word, error) {
	var words []Word
	for i, item := range items {
		w, err := read(item, path+"["+strconv.Itoa(i)+"]")
		if err != nil {
			if err = r.fail(err, i, item); err != nil {
				return nil, err
			}
			continue
		}
		w.Source = []orderedjson.Value{item}
		words = append(words, w)
	}
	return words, nil
}

// Utterances builds one cue per item, with the fields start, end, textKey, speaker and words. The whole item is the
// format data of the cue.
func (r *Reader) Utterances(items []orderedjson.Value, path, textKey string, unit time.Duration,
	readWords func(items []orderedjson.Value, path string) ([]Word, error),
) ([]*model.Cue, error) {
	var cues []*model.Cue
	for i, item := range items {
		start, end, text, words, err := readUtterance(item, path+"["+strconv.Itoa(i)+"]", textKey, unit, readWords)
		if err != nil {
			if err = r.fail(err, i, item); err != nil {
				return nil, err
			}
			continue
		}
		var speaker string
		if v, ok := item.Get("speaker"); ok {
			speaker, _ = Speaker(v)
		}
		data, _ := jsondata.ToData(item).(*model.Data)
		c, err := r.Cue(start, end, text, words, speaker, data, i, item)
		if err != nil {
			return nil, err
		}
		if c != nil {
			cues = append(cues, c)
		}
	}
	return cues, nil
}

func readUtterance(item orderedjson.Value, path, textKey string, unit time.Duration,
	readWords func(items []orderedjson.Value, path string) ([]Word, error),
) (start, end time.Duration, text string, words []Word, err error) {
	v, _ := item.Get("start")
	if start, err = Seconds(v, path+".start", unit); err != nil {
		return 0, 0, "", nil, err
	}
	v, _ = item.Get("end")
	if end, err = Seconds(v, path+".end", unit); err != nil {
		return 0, 0, "", nil, err
	}
	if text, err = Text(item, textKey, path); err != nil {
		return 0, 0, "", nil, err
	}
	if v, _ = item.Get("words"); v.Kind() == orderedjson.Array {
		words, err = readWords(v.Items(), path+".words")
	}
	return start, end, text, words, err
}

// fail passes a parse error to State.Fail. An error of another kind ends the read.
func (r *Reader) fail(err error, block int, item orderedjson.Value) error {
	var perr *model.ParseError
	if !errors.As(err, &perr) {
		return err
	}
	return r.st.Fail(perr, model.NoLine, block, []string{orderedjson.Block(item)})
}

// Transcript returns a subtitle with the cues in start order and fileData as the format data of the file.
// language sets the language metadata when it is a string that is not empty.
func (r *Reader) Transcript(cues []*model.Cue, language orderedjson.Value, fileData *model.Data) (*model.Subtitle, error) {
	s := &model.Subtitle{}
	if lang, ok := language.AsString(); ok && lang != "" {
		s.SetMetadata(model.MetadataLanguage, lang)
	}
	if err := s.SetFormatData(r.key, fileData); err != nil {
		return nil, fmt.Errorf("%s file data: %w", r.key, err)
	}
	added := make([]model.Cue, len(cues))
	for i, c := range cues {
		added[i] = *c
	}
	s.AddCues(added...)
	return s, nil
}
