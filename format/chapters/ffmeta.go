package chapters

import (
	"fmt"
	"io"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const (
	ffmetaKey = string(model.FFMetadataChapters)
	// defaultTimeBase is the time base of FFmpeg for a chapter without a TIMEBASE line: nanoseconds.
	defaultTimeBase = "1/1000000000"
	// writeTimeBase is the time base that the writer uses for a chapter without a stored one: milliseconds.
	writeTimeBase = "1/1000"
	limitMillis   = int64(timecode.MaxHours) * 3600 * 1000
)

// ffmetaMetadataKeys are the global tags that the metadata of the subtitle holds.
//
//nolint:gochecknoglobals // fixed list
var ffmetaMetadataKeys = []string{
	model.MetadataTitle, model.MetadataAuthor, model.MetadataArtist, model.MetadataAlbum, model.MetadataLanguage,
}

// chapterTimes holds the times that a [CHAPTER] section gives.
type chapterTimes struct {
	start, end time.Duration
	hasEnd     bool
	timeBase   string
}

type ffmetaChapter struct {
	chapterTimes
	tags *model.Data
}

// ReadFFMetadata reads the ;FFMETADATA text of FFmpeg. The global tags and the stream tags go to the format data
// "ffmeta-chapters". It ignores the Lenient option for its own lines.
func ReadFFMetadata(r io.Reader, common model.CommonReadOptions, opts ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, err
	}
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.FFMetadataChapters, err)
	}
	st := parsing.NewState(model.FFMetadataChapters, common)
	content, ok, err := st.Prepare(content, false)
	if err != nil {
		return nil, nil, err
	}
	s := &model.Subtitle{}
	if !ok {
		return s, st.Warnings(), nil
	}
	text := lineEnds.Replace(string(content))
	if !strings.HasPrefix(text, ";FFMETADATA") {
		return nil, nil, parseError(model.FFMetadataChapters, 1, "The content does not start with the ;FFMETADATA header.")
	}
	global, streams, chapters, err := readSections(logicalLines(text))
	if err != nil {
		return nil, nil, err
	}
	if err := buildFFMetadata(s, global, streams, chapters, opts.MediaDuration); err != nil {
		return nil, nil, err
	}
	return s, st.Warnings(), nil
}

type logicalLine struct {
	number int
	text   string
}

// logicalLines joins each line that ends with an odd number of backslashes to the next line with LF, except the last
// line of text. It drops empty lines and lines that start with ; or #. A line keeps the number of its first physical line.
func logicalLines(text string) []logicalLine {
	var lines []logicalLine
	var pending []string
	first := 0
	physical := strings.Split(text, "\n")
	for i, line := range physical {
		if pending == nil {
			first = i + 1
		}
		pending = append(pending, line)
		if i < len(physical)-1 && (len(line)-len(strings.TrimRight(line, `\`)))%2 == 1 {
			continue
		}
		joined := line
		if len(pending) > 1 {
			joined = strings.Join(pending, "\n")
		}
		pending = nil
		if joined != "" && joined[0] != ';' && joined[0] != '#' {
			lines = append(lines, logicalLine{first, joined})
		}
	}
	return lines
}

func readSections(lines []logicalLine) (global *model.Data, streams []any, chapters []ffmetaChapter, err error) {
	global = model.NewData()
	tags := global
	for i := 0; i < len(lines); i++ {
		switch lines[i].text {
		case "[STREAM]":
			tags = model.NewData()
			streams = append(streams, tags)
		case "[CHAPTER]":
			var times chapterTimes
			var previousEnd time.Duration
			if n := len(chapters); n > 0 && chapters[n-1].hasEnd {
				previousEnd = chapters[n-1].end
			}
			if times, i, err = readChapterTimes(lines, i, previousEnd); err != nil {
				return nil, nil, nil, err
			}
			tags = model.NewData()
			chapters = append(chapters, ffmetaChapter{times, tags})
		default:
			if key, value, ok := splitTag(lines[i].text); ok {
				tags.Set(key, value)
			}
		}
	}
	return global, streams, chapters, nil
}

// readChapterTimes reads the TIMEBASE, START and END lines that follow the [CHAPTER] line at lines[i], in this order
// and each only as the next line. It returns the index of the last line that it used.
func readChapterTimes(lines []logicalLine, i int, previousEnd time.Duration) (chapterTimes, int, error) {
	times := chapterTimes{start: previousEnd, timeBase: defaultTimeBase}
	num, den := int64(1), int64(1_000_000_000)
	if n, d, line, ok := timeBaseLine(lines, i+1); ok {
		var valid bool
		if num, den, valid = parseTimeBase(n, d); !valid {
			return times, i, parseError(model.FFMetadataChapters, line, "The chapter time base "+n+"/"+d+" is not valid.")
		}
		times.timeBase = strconv.FormatInt(num, 10) + "/" + strconv.FormatInt(den, 10)
		i++
	}
	d, ok, err := ticksLine(lines, i+1, "START=", num, den)
	if err != nil {
		return times, i, err
	}
	if ok {
		times.start = d
		i++
	}
	d, ok, err = ticksLine(lines, i+1, "END=", num, den)
	if err != nil {
		return times, i, err
	}
	if ok {
		times.end, times.hasEnd = d, true
		i++
	}
	return times, i, nil
}

// timeBaseLine matches lines[at] against ^TIMEBASE=(\d+)/(\d+). It returns the digits and the line number.
func timeBaseLine(lines []logicalLine, at int) (num, den string, number int, ok bool) {
	if at >= len(lines) || !strings.HasPrefix(lines[at].text, "TIMEBASE=") {
		return "", "", 0, false
	}
	num, den, ok = timeBaseParts(lines[at].text[len("TIMEBASE="):])
	return num, den, lines[at].number, ok
}

// ticksLine matches lines[at] against ^PREFIX(-?\d+) and returns its time.
func ticksLine(lines []logicalLine, at int, prefix string, num, den int64) (time.Duration, bool, error) {
	if at >= len(lines) {
		return 0, false, nil
	}
	text, ticks, ok := signedField(lines[at].text, prefix)
	if !ok {
		return 0, false, nil
	}
	d, err := ticksTime(ticks, text, num, den, lines[at].number)
	return d, err == nil, err
}

// timeBaseParts matches ^(\d+)/(\d+) at the start of s.
func timeBaseParts(s string) (num, den string, ok bool) {
	n := digitsEnd(s, 0)
	if n == 0 || n >= len(s) || s[n] != '/' {
		return "", "", false
	}
	d := digitsEnd(s, n+1)
	if d == n+1 {
		return "", "", false
	}
	return s[:n], s[n+1 : d], true
}

// parseTimeBase returns the numbers of a time base. It fails for a 0 and for a number beyond int64.
func parseTimeBase(n, d string) (num, den int64, ok bool) {
	num, err1 := strconv.ParseInt(n, 10, 64)
	den, err2 := strconv.ParseInt(d, 10, 64)
	return num, den, err1 == nil && err2 == nil && num > 0 && den > 0
}

// signedField matches ^PREFIX(-?\d+) at the start of s. It returns the matched text and the digits with their sign.
func signedField(s, prefix string) (text, ticks string, ok bool) {
	if !strings.HasPrefix(s, prefix) {
		return "", "", false
	}
	p := len(prefix)
	if p < len(s) && s[p] == '-' {
		p++
	}
	end := digitsEnd(s, p)
	if end == p {
		return "", "", false
	}
	return s[:end], s[len(prefix):end], true
}

// ticksTime returns the time of ticks, a decimal integer with an optional sign, in a time base of num / den seconds,
// rounded to whole milliseconds. A positive time of 100000 hours or more fails with an error on line. That includes a
// number beyond int64. A negative time of that size becomes -timecode.MaxValidTime.
func ticksTime(ticks, text string, num, den int64, line int) (time.Duration, error) {
	n, err := strconv.ParseInt(ticks, 10, 64)
	negative := strings.HasPrefix(ticks, "-")
	var ms *big.Int
	if err == nil {
		ms = mulDivRound(big.NewInt(den), magnitude(n), uint64(num), 1000)
	}
	if err != nil || !ms.IsInt64() || ms.Int64() >= limitMillis {
		if negative {
			return -timecode.MaxValidTime, nil
		}
		e := parsing.OverLimit(text, line)
		e.Format = model.FFMetadataChapters
		return 0, e
	}
	d := time.Duration(ms.Int64()) * time.Millisecond
	if negative {
		d = -d
	}
	return d, nil
}

func magnitude(n int64) uint64 {
	if n < 0 {
		return uint64(-(n + 1)) + 1
	}
	return uint64(n)
}

// mulDivRound returns the product of factors divided by den, rounded half up. The factors are 0 or more.
func mulDivRound(den *big.Int, factors ...uint64) *big.Int {
	p := big.NewInt(1)
	for _, f := range factors {
		p.Mul(p, new(big.Int).SetUint64(f))
	}
	q, r := new(big.Int).QuoRem(p, den, new(big.Int))
	if r.Lsh(r, 1).Cmp(den) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	return q
}

// splitTag splits a tag line at the first = that no backslash escapes. It unescapes key and value.
func splitTag(line string) (key, value string, ok bool) {
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '\\':
			if i+1 == len(line) {
				return "", "", false
			}
			i++
		case '=':
			return unescapeTag(line[:i]), unescapeTag(line[i+1:]), true
		}
	}
	return "", "", false
}

// unescapeTag removes each backslash that is followed by another byte.
func unescapeTag(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// tagsValue returns tags, or an empty list when it has no tag, because PHP cannot tell an empty object from an empty list.
func tagsValue(tags *model.Data) any {
	if tags.Len() == 0 {
		return []any{}
	}
	return tags
}

func buildFFMetadata(s *model.Subtitle, global *model.Data, streams []any, chapters []ffmetaChapter, media time.Duration) error {
	for _, key := range global.Keys() {
		if slices.Contains(ffmetaMetadataKeys, key) {
			value, _ := global.String(key)
			s.SetMetadata(key, value)
		}
	}
	if streams == nil {
		streams = []any{}
	}
	if err := s.SetFormatData(ffmetaKey, model.NewData(model.Field{Key: "tags", Value: tagsValue(global)}, model.Field{Key: "streams", Value: streams})); err != nil {
		return err
	}
	cues := make([]model.Cue, len(chapters))
	ends := make([]*time.Duration, len(chapters))
	for i, ch := range chapters {
		title, _ := ch.tags.String("title")
		cues[i] = escapedCue(ch.start, title)
		rest := ch.tags.Clone()
		rest.Delete("title")
		data := model.NewData(model.Field{Key: "timeBase", Value: ch.timeBase}, model.Field{Key: "tags", Value: tagsValue(rest)})
		if err := cues[i].SetFormatData(ffmetaKey, data); err != nil {
			return err
		}
		if ch.hasEnd {
			ends[i] = &ch.end
		}
	}
	s.AddCues(endChapters(cues, ends, media)...)
	return nil
}

// WriteFFMetadata writes the ;FFMETADATA1 text of FFmpeg. It takes the global tags, the stream tags, the time base
// and the other chapter tags from the format data "ffmeta-chapters".
func WriteFFMetadata(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, _ WriteOptions) error {
	s, err := writing.SkipImageCues(s, model.FFMetadataChapters, common)
	if err != nil {
		return err
	}
	fileData, _ := s.FormatData(ffmetaKey)
	var b strings.Builder
	b.WriteString(";FFMETADATA1\n")
	writeTags(&b, globalTags(s, fileData))
	streams, _ := fileData.List("streams")
	for _, stream := range streams {
		tags, _ := stream.(*model.Data)
		b.WriteString("[STREAM]\n")
		writeTags(&b, storedTags(tags))
	}
	for _, c := range s.All() {
		cueData, _ := c.FormatData(ffmetaKey)
		timeBase, _ := cueData.String("timeBase")
		num, den, ok := splitTimeBase(timeBase)
		if !ok {
			timeBase = writeTimeBase
			num, den, _ = splitTimeBase(timeBase)
		}
		fmt.Fprintf(&b, "[CHAPTER]\nTIMEBASE=%s\nSTART=%d\nEND=%d\n", timeBase, timeTicks(c.Start, num, den), timeTicks(c.End, num, den))
		var tags []tag
		if title := strings.Join(markupcore.PlainLines(c.Lines), "\n"); title != "" {
			tags = append(tags, tag{"title", title})
		}
		cueTags, _ := cueData.Object("tags")
		for _, t := range storedTags(cueTags) {
			if t.key != "title" {
				tags = append(tags, t)
			}
		}
		writeTags(&b, tags)
	}
	return writing.Write(w, model.FFMetadataChapters, []byte(b.String()), common, false)
}

type tag struct{ key, value string }

// storedTags returns the tags of d that have a string value, in order.
func storedTags(d *model.Data) []tag {
	var tags []tag
	for _, key := range d.Keys() {
		if value, ok := d.String(key); ok {
			tags = append(tags, tag{key, value})
		}
	}
	return tags
}

// globalTags returns the stored tags in order, then the metadata keys that they lack. A metadata key takes the
// current metadata value, and a key without metadata has no tag.
func globalTags(s *model.Subtitle, fileData *model.Data) []tag {
	stored, _ := fileData.Object("tags")
	keys := stored.Keys()
	for _, key := range ffmetaMetadataKeys {
		if _, ok := stored.Get(key); !ok {
			keys = append(keys, key)
		}
	}
	var tags []tag
	for _, key := range keys {
		if slices.Contains(ffmetaMetadataKeys, key) {
			if value, ok := s.Metadata(key); ok {
				tags = append(tags, tag{key, value})
			}
		} else if value, ok := stored.String(key); ok {
			tags = append(tags, tag{key, value})
		}
	}
	return tags
}

var tagEscaper = strings.NewReplacer("#", `\#`, ";", `\;`, "=", `\=`, `\`, `\\`, "\n", "\\\n") //nolint:gochecknoglobals // a replacer holds no state

func writeTags(b *strings.Builder, tags []tag) {
	for _, t := range tags {
		b.WriteString(tagEscaper.Replace(t.key))
		b.WriteByte('=')
		b.WriteString(tagEscaper.Replace(t.value))
		b.WriteByte('\n')
	}
}

func splitTimeBase(timeBase string) (num, den int64, ok bool) {
	n, d, found := strings.Cut(timeBase, "/")
	if !found {
		return 0, 0, false
	}
	return parseTimeBase(n, d)
}

// timeTicks returns d, 0 or more, in ticks of num / den seconds, rounded half away from zero. A count beyond int64 is
// the largest int64.
func timeTicks(d time.Duration, num, den int64) int64 {
	ms := max(d.Round(time.Millisecond), 0).Milliseconds()
	q := mulDivRound(new(big.Int).Mul(big.NewInt(num), big.NewInt(1000)), uint64(ms), uint64(den))
	if !q.IsInt64() {
		return math.MaxInt64
	}
	return q.Int64()
}
