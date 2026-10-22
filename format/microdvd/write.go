package microdvd

import (
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

var fontTagRegexp = regexp.MustCompile(`(?i)</?font\b`)

// storedLine is one entry of the lines list of the cue format data.
type storedLine struct {
	codes, otherCodes string
	style             style
}

// Write writes s as MicroDVD.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	fileData, _ := s.FormatData(formatDataKey)
	fps := opts.FrameRate
	if fps == 0 {
		stored, ok := fileData.Number("frameRate")
		if !ok {
			return &model.InvalidArgumentError{
				Option: "WriteOptions.MicroDVD.FrameRate",
				Msg:    "MicroDVD output needs the frame rate of the video.",
			}
		}
		if err := checkFrameRate(stored, "", "The frame rate"); err != nil {
			return err
		}
		fps = stored
	}

	var body []byte
	if frameRateLine, _ := fileData.Bool("frameRateLine"); opts.WriteFrameRateLine || frameRateLine {
		body = append(body, "{1}{1}"...)
		body = append(body, timecode.Number(fps)...)
		body = append(body, '\n')
	}
	for _, c := range s.All() {
		body = append(body, '{')
		body = appendFrame(body, c.Start, fps)
		body = append(body, "}{"...)
		body = appendFrame(body, c.End, fps)
		body = append(body, '}')
		body = append(body, writeText(c, common.StripTags)...)
		body = append(body, '\n')
	}
	return writing.Write(w, model.MicroDVD, body, common, false)
}

// appendFrame appends the frame of d, rounded half away from zero. The seconds come from whole milliseconds, as
// PHP holds them, because d.Seconds() can differ in the last bit.
func appendFrame(dst []byte, d time.Duration, fps float64) []byte {
	secs := float64(max(d, 0).Milliseconds()) / 1000
	return strconv.AppendFloat(dst, math.Round(secs*fps), 'f', 0, 64)
}

// writeText returns the text of c with control codes for the styles that wrap a whole line. A line keeps its
// stored codes when every line has its stored color and tags.
func writeText(c model.Cue, stripTags bool) string {
	stored := storedLines(c)
	carried := make([]string, len(c.Lines))
	for i, line := range c.Lines {
		carried[i] = markupcore.RubyAsText(line)
	}
	carried = markupcore.CarryOpenTags(carried)
	lines := make([]writtenLine, len(carried))
	keepStored := !stripTags && len(stored) == len(lines)
	for i, line := range carried {
		closed := line + markupcore.CloseCoreTags(markupcore.OpenCoreTags(line, nil))
		lines[i] = readLine(markupcore.KeepTags(closed, "b", "i", "u", "s", "font"))
		keepStored = keepStored && lines[i].style.color == stored[i].style.color && slices.Equal(lines[i].style.tags, stored[i].style.tags)
	}

	texts := make([]string, len(lines))
	for i, line := range lines {
		switch {
		case keepStored:
			texts[i] = stored[i].codes + line.text
		case stripTags:
			texts[i] = line.text
		default:
			var codes strings.Builder
			if line.style.color != "" {
				codes.WriteString("{c:$" + strings.ToUpper(markupcore.RGBToBGR(line.style.color[1:])) + "}")
			}
			for _, tag := range line.style.tags {
				codes.WriteString("{y:" + tag + "}")
			}
			if i < len(stored) {
				codes.WriteString(stored[i].otherCodes)
			}
			texts[i] = codes.String() + line.text
		}
	}
	return strings.Join(texts, "|")
}

// writtenLine is a cue line split into the styles that wrap it whole and its plain text.
type writtenLine struct {
	style style
	text  string
}

// readLine peels the tags that wrap the whole line: <x>...</x> for x in b, i, u and s when the inner text holds
// no other <x> or </x>, and one <font> with a color when the inner text holds no other <font or </font.
func readLine(line string) writtenLine {
	var color string
	var tags []string
	for {
		if tag, inner, ok := wrappingStyleTag(line); ok {
			tags = append(tags, tag)
			line = inner
			continue
		}
		if color == "" {
			if hex, inner, ok := wrappingFont(line); ok {
				color = hex
				line = inner
				continue
			}
		}
		break
	}
	return writtenLine{style: style{color: color, tags: orderedTags(tags, nil)}, text: markupcore.PlainText(line)}
}

func wrappingStyleTag(line string) (tag, inner string, ok bool) {
	if len(line) < 7 || line[0] != '<' || line[2] != '>' {
		return "", "", false
	}
	tag = line[1:2]
	if !slices.Contains(styleTags[:], tag) || !strings.HasSuffix(line, "</"+tag+">") {
		return "", "", false
	}
	inner = line[3 : len(line)-4]
	if strings.Contains(inner, "<"+tag+">") || strings.Contains(inner, "</"+tag+">") {
		return "", "", false
	}
	return tag, inner, true
}

func wrappingFont(line string) (hex, inner string, ok bool) {
	lower := markupcore.ASCIILower(line)
	if !strings.HasPrefix(lower, "<font") || !strings.HasSuffix(lower, "</font>") {
		return "", "", false
	}
	if len(line) > 5 && isWordByte(line[5]) {
		return "", "", false
	}
	end := strings.IndexByte(line, '>')
	if end+1 > len(line)-len("</font>") {
		return "", "", false
	}
	attr, ok := markupcore.FontColor(line[5:end])
	if !ok {
		return "", "", false
	}
	if hex, ok = markupcore.ColorHex(attr); !ok {
		return "", "", false
	}
	inner = line[end+1 : len(line)-len("</font>")]
	if fontTagRegexp.MatchString(inner) {
		return "", "", false
	}
	return hex, inner, true
}

// storedLines returns the lines entry of the cue format data, or nil when it has none of the schema shape.
func storedLines(c model.Cue) []storedLine {
	data, _ := c.FormatData(formatDataKey)
	items, ok := data.List("lines")
	if !ok {
		return nil
	}
	out := make([]storedLine, len(items))
	for i, item := range items {
		d, _ := item.(*model.Data)
		out[i].codes, _ = d.String("codes")
		out[i].otherCodes, _ = d.String("otherCodes")
		out[i].style.color, _ = d.String("color")
		out[i].style.tags, _ = d.Strings("tags")
	}
	return out
}

func isWordByte(c byte) bool {
	return c == '_' || '0' <= c && c <= '9' || isASCIILetter(c)
}
