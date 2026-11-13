// Package dvdsub decodes the files of DVD subtitles: the .idx index of a VobSub pair and the subpicture units
// (SPU) of the .sub program stream.
//
// The layout follows http://sam.zoy.org/writings/dvd/subtitles/ and http://dvd.sourceforge.net/dvdinfo/spu.html.
// The behaviour follows dvdsubdec.c and mpeg.c of FFmpeg and VobSubFile.cpp of VSFilter. The code is written
// here, because those projects are LGPL and GPL.
package dvdsub

import (
	"bytes"
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/textenc"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

// MaxTrackIndex is the largest index: value of an id line. The sub-stream of a track is 0x20 plus its index.
const MaxTrackIndex = 31

// Index is the content of a .idx file.
type Index struct {
	// Width and Height are the screen size of the size line.
	Width, Height int
	// Palette holds the 16 colours of the palette line as 0xRRGGBB. It is zero when the file has no palette line.
	Palette [16]uint32
	// Custom holds the colours of an enabled custom colors line as 0xRRGGBBAA by pixel value, else nil.
	Custom *[4]uint32
	// Tracks holds the tracks in file order.
	Tracks []Track
}

// Track is one id line of a .idx file with its timestamp lines.
type Track struct {
	// ID is the id value, trimmed. It can be empty.
	ID string
	// Index is the index value, from 0 to MaxTrackIndex.
	Index int
	// Entries holds the timestamp lines in time order.
	Entries []Entry
}

// Entry is one timestamp line.
type Entry struct {
	// Time is the time of the line plus the sum of the delay lines of the track before it.
	Time time.Duration
	// FilePos is the byte offset of the unit in the .sub file.
	FilePos int64
	// Line is the position of the timestamp line among the timestamp lines of its track, counted from 0.
	Line int
}

var (
	sizeRegexp   = regexp.MustCompile(`(?i)^(\d+)\s*x\s*(\d+)$`)
	customRegexp = regexp.MustCompile(`(?i)^(on|off|1|0)\s*,\s*tridx\s*:\s*([01]{4})\s*,\s*colors\s*:\s*(.*)$`)
	idRegexp     = regexp.MustCompile(`(?i)^([^,]*),\s*index:\s*(\d+)$`)
	entryRegexp  = regexp.MustCompile(`(?i)^(.+?),\s*filepos:\s*([0-9a-f]+)$`)
	colorRegexp  = regexp.MustCompile(`(?i)^[0-9a-f]{1,6}$`)
	timeRegexp   = regexp.MustCompile(`^([+-]?)(\d+):(\d{1,2}):(\d{1,2})[:.,](\d{1,3})$`)
)

func parseError(msg string) *model.ParseError {
	return &model.ParseError{Format: model.VobSub, Msg: msg}
}

// parser holds the state of one ParseIndex call.
type parser struct {
	ix         Index
	hasSize    bool
	hasPalette bool
	delay      time.Duration
}

// ParseIndex reads the lines of a .idx file. Every error is a *model.ParseError of the format vobsub.
func ParseIndex(idx []byte) (Index, error) {
	text := strings.Split(markupcore.NormalizeLineEnds(string(textenc.TrimUTF8BOMs(idx))), "\n")
	if !strings.Contains(text[0], "VobSub index file") {
		return Index{}, parseError(`The .idx content does not start with the "VobSub index file" line.`)
	}
	var p parser
	for _, line := range text[1:] {
		line = strings.Trim(line, markupcore.PHPTrimSet)
		if line == "" || line[0] == '#' || !strings.Contains(line, ":") {
			continue
		}
		if err := p.parseLine(line); err != nil {
			return Index{}, err
		}
	}
	return p.finish()
}

const indexFirstLine = "# VobSub index file, v7 (do not modify this line!)\n"

// ParseHeader reads the .idx header that a Matroska file holds in the CodecPrivate of an S_VOBSUB track. The header
// may lack the first line, so ParseHeader adds the line "# VobSub index file" before it when the first line has no
// "VobSub index file". Every error is a *model.ParseError of the format vobsub.
func ParseHeader(private []byte) (Index, error) {
	private = textenc.TrimUTF8BOMs(private)
	first, _, _ := bytes.Cut(private, []byte("\n"))
	if !bytes.Contains(first, []byte("VobSub index file")) {
		private = append([]byte(indexFirstLine), private...)
	}
	return ParseIndex(private)
}

// finish checks that the file gave a size and colours, and puts the entries of each track in time order.
func (p *parser) finish() (Index, error) {
	switch {
	case !p.hasSize:
		return Index{}, parseError("The .idx content has no size line.")
	case !p.hasPalette && p.ix.Custom == nil:
		return Index{}, parseError("The .idx content has no palette line.")
	}
	for i := range p.ix.Tracks {
		slices.SortStableFunc(p.ix.Tracks[i].Entries, func(a, b Entry) int { return cmp.Compare(a.Time, b.Time) })
	}
	return p.ix, nil
}

func (p *parser) parseLine(line string) error {
	key, value, _ := strings.Cut(line, ":")
	value = strings.Trim(value, markupcore.PHPTrimSet)
	var err error
	switch strings.ToLower(strings.Trim(key, markupcore.PHPTrimSet)) {
	case "size":
		err = p.parseSize(value, line)
	case "palette":
		err = p.parsePalette(value, line)
	case "custom colors":
		var custom [4]uint32
		var on bool
		if custom, on, err = parseCustom(value, line); err == nil {
			p.ix.Custom = nil
			if on {
				p.ix.Custom = &custom
			}
		}
	case "id":
		err = p.parseID(value, line)
	case "delay":
		err = p.parseDelay(value, line)
	case "timestamp":
		err = p.parseEntry(value, line)
	}
	return err
}

func (p *parser) parsePalette(value, line string) error {
	colors, err := parseColors(value, 16, line)
	if err == nil {
		copy(p.ix.Palette[:], colors)
		p.hasPalette = true
	}
	return err
}

func (p *parser) parseDelay(value, line string) error {
	d, err := parseTime(value, line)
	if err == nil {
		// The clamp keeps a sum of many delay lines from wrapping. It never changes a time that passes the limit check.
		p.delay = max(min(p.delay+d, 2*timecode.MaxTime), -2*timecode.MaxTime)
	}
	return err
}

func (p *parser) parseSize(value, line string) error {
	m := sizeRegexp.FindStringSubmatch(value)
	if m != nil {
		w, werr := strconv.Atoi(m[1])
		h, herr := strconv.Atoi(m[2])
		if werr == nil && herr == nil && w >= 1 && h >= 1 {
			p.ix.Width, p.ix.Height, p.hasSize = w, h, true
			return nil
		}
	}
	return parseError(`The .idx size line "` + line + `" is not valid.`)
}

// parseCustom reads a custom colors line. on is false for a line that turns the colours off.
func parseCustom(value, line string) (custom [4]uint32, on bool, err error) {
	m := customRegexp.FindStringSubmatch(value)
	if m == nil {
		return custom, false, parseError(`The .idx custom colors line "` + line + `" is not valid.`)
	}
	if state := strings.ToLower(m[1]); state != "on" && state != "1" {
		return custom, false, nil
	}
	colors, err := parseColors(m[3], 4, line)
	if err != nil {
		return custom, false, err
	}
	for i, rgb := range colors {
		custom[i] = rgb << 8
		if m[2][i] != '1' {
			custom[i] |= 0xFF
		}
	}
	return custom, true, nil
}

func parseColors(value string, count int, line string) ([]uint32, error) {
	parts := strings.Split(value, ",")
	colors := make([]uint32, 0, count)
	for _, part := range parts {
		part = strings.Trim(part, markupcore.PHPTrimSet)
		if !colorRegexp.MatchString(part) {
			break
		}
		c, _ := strconv.ParseUint(part, 16, 32)
		colors = append(colors, uint32(c))
	}
	if len(colors) != len(parts) || len(parts) != count {
		return nil, parseError(`The .idx line "` + line + `" needs ` + strconv.Itoa(count) + " colors as hex RGB.")
	}
	return colors, nil
}

func (p *parser) parseID(value, line string) error {
	m := idRegexp.FindStringSubmatch(value)
	if m != nil {
		if n, err := strconv.Atoi(m[2]); err == nil && n <= MaxTrackIndex {
			p.ix.Tracks = append(p.ix.Tracks, Track{ID: strings.Trim(m[1], markupcore.PHPTrimSet), Index: n})
			p.delay = 0
			return nil
		}
	}
	return parseError(`The .idx id line "` + line + `" is not valid.`)
}

func (p *parser) parseEntry(value, line string) error {
	if len(p.ix.Tracks) == 0 {
		return parseError(`The .idx timestamp line "` + line + `" comes before any id line.`)
	}
	m := entryRegexp.FindStringSubmatch(value)
	if m == nil {
		return parseError(`The .idx timestamp line "` + line + `" is not valid.`)
	}
	d, err := parseTime(m[1], line)
	if err != nil {
		return err
	}
	text := line
	if p.delay != 0 {
		text += " with delay " + timecode.Seconds(p.delay) + "s"
	}
	d += p.delay
	if d >= timecode.MaxTime || d <= -timecode.MaxTime {
		return parseError(`The time "` + text + `" is not below ` + strconv.Itoa(timecode.MaxHours) + " hours.")
	}
	pos, err := strconv.ParseInt(m[2], 16, 64)
	if err != nil {
		pos = 1<<63 - 1
	}
	t := &p.ix.Tracks[len(p.ix.Tracks)-1]
	t.Entries = append(t.Entries, Entry{Time: d, FilePos: pos, Line: len(t.Entries)})
	return nil
}

// parseTime reads h:mm:ss and 1 to 3 digits of milliseconds after one of ":.,". Line is the text of the error.
func parseTime(value, line string) (time.Duration, error) {
	m := timeRegexp.FindStringSubmatch(strings.Trim(value, markupcore.PHPTrimSet))
	if m == nil {
		return 0, parseError(`The .idx time "` + line + `" is not valid.`)
	}
	hours, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil || hours >= timecode.MaxHours {
		return 0, parseError(`The time "` + line + `" is not below ` + strconv.Itoa(timecode.MaxHours) + " hours.")
	}
	minutes, _ := strconv.Atoi(m[3])
	seconds, _ := strconv.Atoi(m[4])
	millis, _ := strconv.Atoi(strings.Repeat("0", 3-len(m[5])) + m[5])
	d := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second +
		time.Duration(millis)*time.Millisecond
	if d >= timecode.MaxTime {
		return 0, parseError(`The time "` + line + `" is not below ` + strconv.Itoa(timecode.MaxHours) + " hours.")
	}
	if m[1] == "-" {
		d = -d
	}
	return d, nil
}
