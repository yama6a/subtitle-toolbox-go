package itt

import (
	"io"
	"regexp"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/timecode"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/ttmlwrite"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/writing"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

const parameterNamespace = "http://www.w3.org/ns/ttml#parameter"

//nolint:gochecknoglobals // the fixed Apple head
var headLines = []string{
	"<head>",
	"    <styling>",
	`      <style xml:id="normal" tts:fontFamily="sansSerif" tts:fontWeight="normal" tts:fontStyle="normal" tts:color="white" tts:fontSize="100%"/>`,
	"    </styling>",
	"    <layout>",
	`      <region xml:id="top" tts:origin="0% 0%" tts:extent="100% 15%" tts:textAlign="center" tts:displayAlign="before"/>`,
	`      <region xml:id="bottom" tts:origin="0% 85%" tts:extent="100% 15%" tts:textAlign="center" tts:displayAlign="after"/>`,
	"    </layout>",
	"  </head>",
}

var fontRegexp = regexp.MustCompile(`(?i)<font\b([^>]*)>`)

// output is the frame rate of a write: the rate that frames the cue times and the 2 strings of the root.
type output struct {
	rate                  model.FrameRate
	frameRate, multiplier string
}

// Write writes s as iTT with SMPTE time codes. WriteOptions.FrameRate wins over the rate of the itt format data.
// A time stops at the last frame below 100000 hours.
// The writer drops the stored head, styles and regions, keeps the top row or else the bottom region of a cue,
// keeps only <b>, <i>, <u> and <font color>, and writes at least 1 frame for every cue.
func Write(w io.Writer, s *model.Subtitle, common model.CommonWriteOptions, opts WriteOptions) (WriteReport, error) {
	if err := opts.Validate(); err != nil {
		return WriteReport{}, err
	}
	data, _ := s.FormatData(formatDataKey)
	out, err := chooseOutput(data, opts)
	if err != nil {
		return WriteReport{}, err
	}
	doc := copyOf(s)
	last := lastFrame(out.rate)
	times := make([]ttmlwrite.Times, 0, s.Len())
	for _, c := range s.All() {
		begin := min(out.rate.Frames(max(0, c.Start)), last-1)
		end := min(max(begin+1, out.rate.Frames(max(0, c.End))), last)
		times = append(times, ttmlwrite.Times{Begin: out.rate.TimecodeOf(begin).String(), End: out.rate.TimecodeOf(end).String()})
	}
	body, err := ttmlwrite.Document(doc, common, ttmlwrite.Options{
		Format: model.ITT,
		RootAttributes: []ttmlwrite.Attr{
			{Name: "ttp:timeBase", Value: "smpte"},
			{Name: "ttp:frameRate", Value: out.frameRate},
			{Name: "ttp:frameRateMultiplier", Value: out.multiplier},
			{Name: "ttp:dropMode", Value: "nonDrop"},
		},
		Times: times,
	})
	if err != nil {
		return WriteReport{}, err
	}
	return WriteReport{}, writing.Write(w, model.ITT, body, common, false)
}

// lastFrame returns the last frame whose time is below 100000 hours, the limit that the TTML reader sets.
func lastFrame(r model.FrameRate) int64 {
	n := r.Frames(timecode.MaxTime)
	for r.Duration(n) >= timecode.MaxTime {
		n--
	}
	return n
}

// chooseOutput picks the frame rate of the write from the option and the itt format data. The strings of a stored
// rate stay, so a multiplier of "1000 1001" is not rewritten when the option names the same frame rate.
func chooseOutput(data *model.Data, opts WriteOptions) (output, error) {
	var stored output
	storedRow := -1
	if frameRate, ok := data.String("frameRate"); ok {
		multiplier, ok := data.String("frameRateMultiplier")
		if !ok {
			multiplier = "1 1"
		}
		if rate, ok := storedRate(frameRate, multiplier); ok {
			if row, near := rowOf(rate); near && profileRates[row].frameRate == frameRate {
				stored, storedRow = output{rate, frameRate, multiplier}, row
			}
		}
	}
	if opts.FrameRate.IsZero() {
		if storedRow < 0 {
			return output{}, &model.InvalidArgumentError{Option: frameRateOption, Msg: needsFrameRate}
		}
		return stored, nil
	}
	row, _ := opts.row()
	if row == storedRow {
		return stored, nil
	}
	p := profileRates[row]
	return output(p), nil
}

// copyOf returns the subtitle that the TTML document writer turns into iTT: the Apple head, a top or bottom region
// for each cue, and the reduced markup.
func copyOf(s *model.Subtitle) *model.Subtitle {
	doc := &model.Subtitle{}
	for _, key := range []string{model.MetadataLanguage, model.MetadataTitle} {
		if v, ok := s.Metadata(key); ok {
			doc.SetMetadata(key, v)
		}
	}
	_ = doc.SetFormatData(string(model.TTML), model.NewData( // the data has the shape of the schema
		model.Field{Key: "namespace", Value: ttmlwrite.NSTTML},
		model.Field{Key: "namespaces", Value: model.NewData(model.Field{Key: "ttp", Value: parameterNamespace})},
		model.Field{Key: "head", Value: strings.Join(headLines, "\n")},
		model.Field{Key: "body", Value: model.NewData(model.Field{Key: "style", Value: "normal"})},
	))
	cues := make([]model.Cue, 0, s.Len())
	for _, c := range s.All() {
		region := "bottom"
		if c.Position.Alignment.Row() == 2 {
			region = "top"
		}
		lines := make([]string, len(c.Lines))
		for i, line := range c.Lines {
			lines[i] = supportedMarkup(line)
		}
		cue := model.Cue{Start: c.Start, End: c.End, Lines: lines, ID: c.ID, Forced: c.Forced}
		_ = cue.SetFormatData(string(model.TTML), model.NewData( // the data has the shape of the schema
			model.Field{Key: "attributes", Value: model.NewData(model.Field{Key: "region", Value: region})},
		))
		cues = append(cues, cue)
	}
	doc.AddCues(cues...)
	return doc
}

// supportedMarkup keeps the tags that iTT holds. A <font> keeps only its color, and a value that is no color
// leaves a plain <font>, which the TTML writer writes without a span.
func supportedMarkup(line string) string {
	line = markupcore.KeepTags(markupcore.WebVTTColorsToFont(markupcore.RubyAsText(line)), "b", "i", "u", "font")
	return fontRegexp.ReplaceAllStringFunc(line, func(tag string) string {
		attrs, _ := markupcore.FontColor(fontRegexp.FindStringSubmatch(tag)[1])
		color := markupcore.ASCIILower(strings.Trim(markupcore.DecodeEntities(attrs), markupcore.PHPTrimSet))
		if c, ok := markupcore.TTMLColor(color); ok {
			return `<font color="` + c + `">`
		}
		return "<font>"
	})
}
