package chapters

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	podcastVersion = "1.2.0"

	malformedUTF8 = "JSON cannot hold the subtitle: Malformed UTF-8 characters, possibly incorrectly encoded. " +
		"Read a file in another encoding with its source encoding."
	infOrNaN = "JSON cannot hold the subtitle: Inf and NaN cannot be JSON encoded."
)

// ReadPodcast reads a Podcasting 2.0 chapters file. The members of the file and of each chapter that the model has
// no field for go to the format data "podcast-chapters". It ignores the Lenient option for its own fields.
func ReadPodcast(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.PodcastChapters, err)
	}
	st := parsing.NewState(model.PodcastChapters, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	if err := readPodcastDocument(s, content, opts); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func readPodcastDocument(s *model.Subtitle, content []byte, opts ReadOptions) error {
	root, err := orderedjson.Decode(content)
	if err != nil {
		return &model.ParseError{Format: model.PodcastChapters, Msg: "The content is not valid JSON: " + err.Error() + ".", Err: err}
	}
	if root.Kind() == orderedjson.Array && len(root.Items()) == 0 {
		root = orderedjson.ObjectOf()
	}
	if root.Kind() != orderedjson.Object {
		return parseError(model.PodcastChapters, 0, "The JSON root must be an object.")
	}
	list, _ := root.Get("chapters")
	if list.Kind() != orderedjson.Array {
		return parseError(model.PodcastChapters, 0, `The JSON has no "chapters" list.`)
	}
	cues := make([]model.Cue, len(list.Items()))
	ends := make([]*time.Duration, len(list.Items()))
	for i, chapter := range list.Items() {
		if cues[i], ends[i], err = podcastChapter(i, chapter); err != nil {
			return err
		}
	}
	s.AddCues(endChapters(cues, ends, opts.MediaDuration)...)
	for _, m := range [...]struct{ field, key string }{{"title", model.MetadataTitle}, {"author", model.MetadataAuthor}} {
		if text, ok := stringMember(root, m.field); ok {
			s.SetMetadata(m.key, text)
		}
	}
	data, _ := jsondata.ToData(root.Without("chapters", "title", "author")).(*model.Data)
	return s.SetFormatData(string(model.PodcastChapters), data)
}

// podcastChapter reads chapter i. It returns the cue and the end that the chapter gives, or nil.
func podcastChapter(i int, chapter orderedjson.Value) (model.Cue, *time.Duration, error) {
	path := "chapters[" + strconv.Itoa(i) + "]."
	startValue, _ := chapter.Get("startTime")
	endValue, hasEnd := chapter.Get("endTime")
	hasEnd = hasEnd && endValue.Kind() != orderedjson.Null
	if !isTime(startValue) {
		return model.Cue{}, nil, parseError(model.PodcastChapters, 0, "The field "+path+"startTime must be a number.")
	}
	if hasEnd && !isTime(endValue) {
		return model.Cue{}, nil, parseError(model.PodcastChapters, 0, "The field "+path+"endTime must be a number.")
	}
	start, err := boundedField(startValue, path+"startTime")
	if err != nil {
		return model.Cue{}, nil, err
	}
	var end *time.Duration
	if hasEnd {
		d, err := boundedField(endValue, path+"endTime")
		if err != nil {
			return model.Cue{}, nil, err
		}
		end = &d
	}
	title, _ := stringMember(chapter, "title")
	cue := escapedCue(start, title)
	data, _ := jsondata.ToData(chapter.Without("startTime", "endTime", "title")).(*model.Data)
	if err := cue.SetFormatData(string(model.PodcastChapters), data); err != nil {
		return model.Cue{}, nil, err
	}
	return cue, end, nil
}

// stringMember returns the member key of object when it is a string.
func stringMember(object orderedjson.Value, key string) (string, bool) {
	v, _ := object.Get(key)
	if v.Kind() != orderedjson.String {
		return "", false
	}
	return v.AsString()
}

// isTime reports whether v is a finite number of 0 or more.
func isTime(v orderedjson.Value) bool {
	f, ok := v.AsFloat64()
	return ok && f >= 0
}

// boundedField returns the seconds of the finite number v as a time below 100000 hours.
func boundedField(v orderedjson.Value, path string) (time.Duration, error) {
	d, ok := v.Duration(time.Second)
	if !ok {
		return 0, parseError(model.PodcastChapters, 0, "The field "+path+" is not below "+strconv.Itoa(timecode.MaxHours)+" hours.")
	}
	return d, nil
}

// WritePodcast writes a Podcasting 2.0 chapters file as pretty JSON. A chapter has an endTime only when its end is
// not the start of the next chapter.
func WritePodcast(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.PodcastChapters, common)
	if err != nil {
		return err
	}
	document, err := podcastDocument(s)
	if err != nil {
		return err
	}
	body, err := orderedjson.Encode(document, true)
	switch {
	case errors.Is(err, orderedjson.ErrInvalidUTF8):
		return &model.UnwritableError{Format: model.PodcastChapters, CueIndex: -1, Reason: malformedUTF8, Err: model.ErrInvalidUTF8}
	case err != nil:
		return &model.UnwritableError{Format: model.PodcastChapters, CueIndex: -1, Reason: infOrNaN}
	}
	return writing.Write(w, model.PodcastChapters, body, common, false)
}

func podcastDocument(s *model.Subtitle) (orderedjson.Value, error) {
	fileData, _ := s.FormatData(string(model.PodcastChapters))
	stored, err := storedMembers(fileData)
	if err != nil {
		return orderedjson.Value{}, err
	}
	version := orderedjson.StringValue(podcastVersion)
	if v, ok := stored.Get("version"); ok && v.Kind() != orderedjson.Null {
		version = v
	}
	members := []orderedjson.Member{{Key: "version", Value: version}}
	for _, m := range []struct{ field, key string }{{"author", model.MetadataAuthor}, {"title", model.MetadataTitle}} {
		if text, ok := s.Metadata(m.key); ok {
			members = append(members, orderedjson.Member{Key: m.field, Value: orderedjson.StringValue(text)})
		}
	}
	chapters := make([]orderedjson.Value, 0, s.Len())
	for i, c := range s.All() {
		chapter, err := podcastChapterValue(s, i, c)
		if err != nil {
			return orderedjson.Value{}, err
		}
		chapters = append(chapters, chapter)
	}
	document := orderedjson.Union(orderedjson.ObjectOf(members...), stored)
	return document.With("chapters", orderedjson.ArrayOf(chapters...)), nil
}

func podcastChapterValue(s *model.Subtitle, i int, c model.Cue) (orderedjson.Value, error) {
	chapterData, _ := c.FormatData(string(model.PodcastChapters))
	own, err := storedMembers(chapterData)
	if err != nil {
		return orderedjson.Value{}, err
	}
	members := []orderedjson.Member{{Key: "startTime", Value: podcastSeconds(c.Start)}}
	implicitEnd := c.Start
	if next, ok := s.Cue(i + 1); ok {
		implicitEnd = next.Start
	}
	if podcastMillis(c.End) != podcastMillis(implicitEnd) {
		members = append(members, orderedjson.Member{Key: "endTime", Value: podcastSeconds(c.End)})
	}
	if title := strings.Join(markupcore.PlainLines(c.Lines), " "); title != "" {
		members = append(members, orderedjson.Member{Key: "title", Value: orderedjson.StringValue(title)})
	}
	return orderedjson.Union(orderedjson.ObjectOf(members...), own), nil
}

func podcastMillis(d time.Duration) int64 {
	return max(d.Round(time.Millisecond), 0).Milliseconds()
}

// podcastSeconds returns d as seconds: an integer for a whole second, else the shortest float.
func podcastSeconds(d time.Duration) orderedjson.Value {
	ms := podcastMillis(d)
	if ms%1000 == 0 {
		return orderedjson.Int(ms / 1000)
	}
	return orderedjson.Seconds(time.Duration(ms) * time.Millisecond)
}

// storedMembers returns the stored format data as an object, empty when there is none.
func storedMembers(d *model.Data) (orderedjson.Value, error) {
	if d == nil {
		return orderedjson.ObjectOf(), nil
	}
	v, err := jsondata.FromData(d)
	if err != nil {
		return orderedjson.Value{}, fmt.Errorf("stored %s data: %w", model.PodcastChapters, err)
	}
	return v, nil
}
