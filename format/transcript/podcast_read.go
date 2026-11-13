package transcript

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// podcastItem is a segment of a Podcasting 2.0 transcript as the reader reads it.
type podcastItem struct {
	index      int               // the place in the segments list, for warnings
	raw        orderedjson.Value // the segment, for warnings
	start, end time.Duration
	hasEnd     bool
	speaker    string
	body       string
	other      orderedjson.Value // the members besides speaker, startTime, endTime and body
}

// ReadPodcast reads a Podcasting 2.0 JSON transcript from r. Unless opts.KeepSegments is set, it joins the
// one-word segments of a speaker into cues that end at a sentence end or at a pause of 1 s or more.
// A field error names the path of the field, such as segments[3].startTime.
func ReadPodcast(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.PodcastTranscript, err)
	}
	st := parsing.NewState(model.PodcastTranscript, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return &model.Subtitle{}, st.Warnings(), nil
	}
	root, err := decodeObject(content, model.PodcastTranscript)
	if err != nil {
		return nil, nil, err
	}
	list, _ := root.Get("segments")
	if list.Kind() != orderedjson.Array {
		return nil, nil, &model.ParseError{Format: model.PodcastTranscript, Msg: `The JSON has no "segments" list.`}
	}
	segments, err := readPodcastItems(st, list.Items())
	if err != nil {
		return nil, nil, err
	}
	segments = resolvePodcastItems(st, segments)

	var cues []*model.Cue
	for _, group := range podcastGroups(segments, opts.KeepSegments) {
		warnLateStart(st, group)
		cue, err := podcastCue(group, opts.WordTimestamps)
		if err != nil {
			return nil, nil, err
		}
		cues = append(cues, cue)
	}
	data, _ := jsondata.ToData(root.Without("segments")).(*model.Data)
	s, err := wordgroup.NewReader(st, wordgroup.Options{}, podcastKey).Transcript(cues, orderedjson.Value{}, data)
	if err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

func readPodcastItems(st *parsing.State, items []orderedjson.Value) ([]podcastItem, error) {
	segments := make([]podcastItem, 0, len(items))
	for i, item := range items {
		path := "segments[" + strconv.Itoa(i) + "]"
		seg, perr := readPodcastItem(item, path)
		seg.index, seg.raw = i, item
		if perr == nil && seg.hasEnd && seg.end < seg.start {
			var err error
			seg.start, seg.end, err = st.OrderedTimes(seg.start, seg.end, model.NoLine, i, []string{orderedjson.Block(item)}, true)
			perr = asParseError(err)
		}
		if perr != nil {
			if err := st.Fail(perr, model.NoLine, i, []string{orderedjson.Block(item)}); err != nil {
				return nil, err
			}
			continue
		}
		segments = append(segments, seg)
	}
	return segments, nil
}

func readPodcastItem(item orderedjson.Value, path string) (podcastItem, *model.ParseError) {
	if item.Kind() != orderedjson.Object {
		return podcastItem{}, fieldError("The field " + path + " must be an object.")
	}
	start, startState, perr := podcastTime(item, "startTime", path, true)
	if perr != nil {
		return podcastItem{}, perr
	}
	end, endState, perr := podcastTime(item, "endTime", path, false)
	if perr != nil {
		return podcastItem{}, perr
	}
	speaker, perr := podcastString(item, "speaker", path)
	if perr != nil {
		return podcastItem{}, perr
	}
	body, perr := podcastString(item, "body", path)
	if perr != nil {
		return podcastItem{}, perr
	}
	if startState == overLimit {
		return podcastItem{}, overLimitError(path + ".startTime")
	}
	if endState == overLimit {
		return podcastItem{}, overLimitError(path + ".endTime")
	}
	return podcastItem{
		start:   start,
		end:     end,
		hasEnd:  endState == validTime,
		speaker: strings.Trim(speaker, markupcore.PHPTrimSet),
		body:    strings.Trim(collapseSpace(body), markupcore.PHPTrimSet),
		other:   item.Without("speaker", "startTime", "endTime", "body"),
	}, nil
}

// podcastTime reads a time in seconds. A missing or null optional time has the state noTime and no error.
func podcastTime(item orderedjson.Value, key, path string, required bool) (time.Duration, timeState, *model.ParseError) {
	v, _ := item.Get(key)
	if !required && v.Kind() == orderedjson.Null {
		return 0, noTime, nil
	}
	d, state := timeOf(v, time.Second)
	if state == noTime {
		return 0, noTime, fieldError("The field " + path + "." + key + " must be a number.")
	}
	return d, state, nil
}

// podcastString reads an optional string. A missing or null value is "".
func podcastString(item orderedjson.Value, key, path string) (string, *model.ParseError) {
	v, _ := item.Get(key)
	if v.Kind() == orderedjson.Null {
		return "", nil
	}
	s, ok := v.AsString()
	if !ok {
		return "", fieldError("The field " + path + "." + key + " must be a string.")
	}
	return s, nil
}

// collapseSpace turns each run of space, tab, CR and LF into one space.
func collapseSpace(s string) string {
	return strings.Join(strings.FieldsFunc(s, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\r' || r == '\n'
	}), " ")
}

// resolvePodcastItems gives each segment without an end the first later start among the segments after it, or the last
// cue duration after its own start. One pass from the back finds that start for every segment. It then drops the
// segments without a body. The HTML transcript reader builds on it.
func resolvePodcastItems(st *parsing.State, segments []podcastItem) []podcastItem {
	var later []int // indexes after i, whose starts rise from the top of the stack to the bottom
	for i := len(segments) - 1; i >= 0; i-- {
		for len(later) > 0 && segments[later[len(later)-1]].start <= segments[i].start {
			later = later[:len(later)-1]
		}
		if !segments[i].hasEnd {
			if len(later) > 0 {
				segments[i].end = segments[later[len(later)-1]].start
			} else {
				segments[i].end = st.LastCueEnd(segments[i].start)
			}
		}
		later = append(later, i)
	}
	kept := segments[:0]
	for _, seg := range segments {
		if seg.body != "" {
			kept = append(kept, seg)
		}
	}
	return kept
}

// podcastGroups splits the segments into runs that become one cue each. A one-word segment joins the group before
// it when the group is open, the first segment of the group has its speaker, and the gap to the end of the last
// segment of the group is below 1 s. A group is open after a one-word segment that does not end a sentence.
func podcastGroups(segments []podcastItem, keepSegments bool) [][]podcastItem {
	var groups [][]podcastItem
	from := 0
	open := false
	for i, seg := range segments {
		isWord := !keepSegments && !strings.Contains(seg.body, " ")
		joins := open && isWord && segments[from].speaker == seg.speaker && seg.start-segments[i-1].end < wordgroup.MaxGap
		if !joins && i > from {
			groups = append(groups, segments[from:i])
			from = i
		}
		open = isWord && !endsSentence(seg.body)
	}
	if from < len(segments) {
		groups = append(groups, segments[from:])
	}
	return groups
}

const (
	closingQuotes = "\"'\xe2\x80\x9d\xe2\x80\x99)]"
	ellipsis      = rune(0x2026)
)

// endsSentence reports whether body ends with ., ?, ! or an ellipsis, before any closing quotes and brackets.
func endsSentence(body string) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(body, closingQuotes))
	return r == '.' || r == '?' || r == '!' || r == ellipsis
}

// warnLateStart warns when the group ends before it starts. That happens when the file lists the segments out of
// order. podcastCue sets the end to the start.
func warnLateStart(st *parsing.State, group []podcastItem) {
	first, last := group[0], group[len(group)-1]
	if last.end >= first.start {
		return
	}
	msg := "The cue ends at " + timecode.Seconds(last.end) + " s, before it starts at " + timecode.Seconds(first.start) + " s. The parser set the end to the start."
	lines := []string{orderedjson.Block(first.raw)}
	if len(group) > 1 {
		lines = append(lines, orderedjson.Block(last.raw))
	}
	st.Warn(msg, model.NoLine, first.index, lines, model.Repaired)
}

func podcastCue(group []podcastItem, wordTimestamps bool) (*model.Cue, error) {
	first, last := group[0], group[len(group)-1]
	texts := make([]string, len(group))
	for i, seg := range group {
		texts[i] = markupcore.EscapeText(seg.body)
	}
	c := model.Cue{Start: first.start, End: max(first.start, last.end), Speaker: first.speaker}
	c.SetText(strings.Join(texts, " "))
	if wordTimestamps && len(group) > 1 {
		words := make([]wordgroup.Word, len(group))
		for i, seg := range group {
			text := strings.TrimFunc(seg.body, markupcore.IsSpace)
			words[i] = wordgroup.Word{Text: text, Start: seg.start, Timed: text != "", End: seg.end, HasEnd: true}
		}
		c.Words = wordgroup.CueWords(c.PlainText(), words, c.End)
	}
	if len(group) == 1 && len(first.other.Members()) > 0 {
		data, _ := jsondata.ToData(first.other).(*model.Data)
		if err := c.SetFormatData(podcastKey, data); err != nil {
			return nil, fmt.Errorf("%s cue data: %w", podcastKey, err)
		}
	}
	return &c, nil
}
