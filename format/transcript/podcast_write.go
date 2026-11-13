package transcript

import (
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/wordgroup"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const podcastVersion = "1.0.0"

var (
	voiceTagStart = regexp.MustCompile("^" + markupcore.VoiceTagStartPattern + `(?:[ \t\n\r\f\v]+([^>]*))?>$`)
	voiceTagEnd   = regexp.MustCompile(`^</v[ \t\n\r\f\v]*>$`)
)

// podcastSegment is a segment that the writer writes.
type podcastSegment struct {
	cue        int
	speaker    string // empty: the segment has no speaker
	start, end time.Duration
	body       string
	word       int // index in Cue.Words of the word that starts the segment, or -1
}

// WritePodcast writes s to w as a Podcasting 2.0 JSON transcript. It writes one segment per cue and speaker, or
// one segment per word of Cue.Words with opts.Podcast.WordSegments. A cue time below 0 counts as 0.
func WritePodcast(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.PodcastTranscript, common)
	if err != nil {
		return err
	}
	doc, err := podcastDocument(s, opts.Podcast)
	if err != nil {
		return err
	}
	body, err := encodeDocument(model.PodcastTranscript, "Podcasting 2.0 transcript JSON", doc, opts.Podcast.PrettyPrint)
	if err != nil {
		return err
	}
	return writing.Write(w, model.PodcastTranscript, body, common, false)
}

func podcastDocument(s *model.Subtitle, opts PodcastWriteOptions) (orderedjson.Value, error) {
	var file orderedjson.Value
	if d, ok := s.FormatData(podcastKey); ok {
		var err error
		if file, err = jsondata.FromData(d); err != nil {
			return file, &model.UnwritableError{Format: model.PodcastTranscript, CueIndex: -1, Reason: "The Podcasting 2.0 format data is no JSON.", Err: err}
		}
	}
	var segments []orderedjson.Value
	for i, c := range s.All() {
		pieces := cuePieces(&c, i, opts.WordSegments)
		var stored orderedjson.Value
		if d, ok := c.FormatData(podcastKey); ok && len(pieces) == 1 && !opts.WordSegments {
			stored, _ = jsondata.FromData(d)
		}
		for _, p := range pieces {
			segments = append(segments, orderedjson.Union(p.object(), stored))
		}
	}
	version := orderedjson.StringValue(podcastVersion)
	if v, _ := file.Get("version"); v.Kind() != orderedjson.Null {
		version = v
	}
	head := orderedjson.ObjectOf(member("version", version), member("segments", orderedjson.ArrayOf(segments...)))
	return orderedjson.Union(head, file), nil
}

// object returns the JSON of the segment. The end is never before the start, so the output reads back.
func (p podcastSegment) object() orderedjson.Value {
	start := max(0, p.start)
	end := max(start, p.end)
	var members []orderedjson.Member
	if p.speaker != "" {
		members = append(members, member("speaker", orderedjson.StringValue(p.speaker)))
	}
	return orderedjson.ObjectOf(append(members,
		member("startTime", orderedjson.Seconds(start)),
		member("endTime", orderedjson.Seconds(end)),
		member("body", orderedjson.StringValue(p.body)))...)
}

// podcastSegments returns the segments of s without word segments: one for each cue and speaker. The HTML
// transcript writer groups them into paragraphs.
func podcastSegments(s *model.Subtitle) []podcastSegment {
	var out []podcastSegment
	for i, c := range s.All() {
		out = append(out, cuePieces(&c, i, false)...)
	}
	return out
}

// pieceWalk holds the state of the walk along the tags of one cue.
type pieceWalk struct {
	index            int
	speaker          string
	start            time.Duration
	word             int // the word timestamp tag that started the piece, or -1
	stamps           int // the number of word timestamp tags so far
	cueStart, cueEnd time.Duration
	text             strings.Builder
	pieces           []podcastSegment
}

// close adds the text so far as a piece, unless its body is empty.
func (pw *pieceWalk) close() {
	body := strings.Trim(collapseSpace(markupcore.DecodeEntities(pw.text.String())), markupcore.PHPTrimSet)
	if body != "" {
		pw.pieces = append(pw.pieces, podcastSegment{cue: pw.index, speaker: pw.speaker, start: pw.start, end: pw.start, body: body, word: pw.word})
	}
	pw.text.Reset()
}

// tag handles one tag of the cue text. It closes the piece before it when the tag changes the speaker or, with
// wordSegments, is a word timestamp.
func (pw *pieceWalk) tag(tag string, wordSegments bool) {
	voice := voiceTagStart.FindStringSubmatch(tag)
	var stamp time.Duration
	isStamp := false
	if wordSegments {
		stamp, isStamp = markupcore.WordTimestamp(tag)
	}
	if voice == nil && !isStamp && !voiceTagEnd.MatchString(tag) {
		return
	}
	name := pw.speaker
	if !isStamp {
		name = ""
		if voice != nil {
			name = strings.Trim(markupcore.DecodeEntities(voice[2]), markupcore.PHPTrimSet)
		}
		if name == pw.speaker {
			return
		}
	}
	pw.close()
	pw.speaker = name
	pw.word = -1
	if isStamp {
		pw.start = min(max(stamp, pw.cueStart), pw.cueEnd)
		pw.word = pw.stamps
		pw.stamps++
	}
}

// cuePieces splits the cue at each speaker change, and with wordSegments at each word timestamp.
func cuePieces(c *model.Cue, index int, wordSegments bool) []podcastSegment {
	pw := &pieceWalk{index: index, speaker: c.Speaker, start: c.Start, word: -1, cueStart: c.Start, cueEnd: c.End}
	for i, part := range markupcore.SplitTags(markupcore.RubyAsText(strings.Join(wordgroup.TaggedLines(c), " "))) {
		if i%2 == 0 {
			pw.text.WriteString(part)
		} else {
			pw.tag(part, wordSegments)
		}
	}
	pw.close()

	for k := range pw.pieces {
		p := &pw.pieces[k]
		p.end = c.End
		if wordSegments && k+1 < len(pw.pieces) {
			p.end = pw.pieces[k+1].start
		}
		if p.word >= 0 && p.word < len(c.Words) {
			p.end = min(p.end, max(c.Words[p.word].End, p.start))
		}
	}
	return pw.pieces
}
