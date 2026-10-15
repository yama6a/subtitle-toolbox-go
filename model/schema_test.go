package model_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const (
	fileLevel = "file"
	cueLevel  = "cue"
)

// setFormatData calls SetFormatData at level and returns the message of its error, or "".
func setFormatData(t *testing.T, level, key string, d *model.Data) string {
	t.Helper()

	var err error
	if level == cueLevel {
		var c model.Cue
		err = c.SetFormatData(key, d)
	} else {
		err = (&model.Subtitle{}).SetFormatData(key, d)
	}
	if err == nil {
		return ""
	}
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	var argErr *model.InvalidArgumentError
	require.ErrorAs(t, err, &argErr)
	assert.Empty(t, argErr.Option)
	return argErr.Msg
}

func data(fields ...any) *model.Data {
	d := model.NewData()
	for i := 0; i+1 < len(fields); i += 2 {
		key, _ := fields[i].(string)
		d.Set(key, fields[i+1])
	}
	return d
}

// csvData returns csv file data with its required fields plus fields.
func csvData(fields ...any) *model.Data {
	return data(append([]any{"header", nil, "roles", data(), "width", int64(3)}, fields...)...)
}

var (
	hex256  = strings.Repeat("0123456789abcdeF", 16)
	ttiList = []any{hex256}
)

type schemaCase struct {
	name  string
	level string
	key   string
	data  *model.Data
	want  string
}

func runSchemaCases(t *testing.T, tests []schemaCase) {
	t.Helper()

	for _, tt := range tests {
		t.Run(tt.level+" "+tt.key+" "+tt.name, func(t *testing.T) {
			t.Parallel()

			got := setFormatData(t, tt.level, tt.key, tt.data)
			assert.Equal(t, tt.want, got)
			if got != "" {
				// The library JSON reader puts "cues[N]." after "The field " for cue data.
				assert.True(t, strings.HasPrefix(got, "The field formatData."+tt.key), got)
			}
		})
	}
}

func TestSchemaTypeLanguage(t *testing.T) {
	t.Parallel()

	runSchemaCases(t, []schemaCase{
		{"string good", fileLevel, "vtt", data("header", "Kind: captions"), ""},
		{"string bad", fileLevel, "vtt", data("header", int64(1)), "The field formatData.vtt.header must be a string."},

		{"int good", cueLevel, "stl", data("subtitleGroupNumber", int64(-3)), ""},
		{"int bad float", cueLevel, "stl", data("subtitleGroupNumber", 1.0), "The field formatData.stl.subtitleGroupNumber must be an integer."},
		{"int bad string", cueLevel, "stl", data("subtitleGroupNumber", "1"), "The field formatData.stl.subtitleGroupNumber must be an integer."},

		{"bool good", fileLevel, "scc", data("dropFrame", false), ""},
		{"bool bad", fileLevel, "scc", data("dropFrame", "yes"), "The field formatData.scc.dropFrame must be a boolean."},

		{"number good int", fileLevel, "microdvd", data("frameRate", int64(25)), ""},
		{"number good float", fileLevel, "microdvd", data("frameRate", 23.976), ""},
		{"number bad infinity", fileLevel, "microdvd", data("frameRate", math.Inf(1)), "The field formatData.microdvd.frameRate must be a finite number."},
		{"number bad NaN", fileLevel, "microdvd", data("frameRate", math.NaN()), "The field formatData.microdvd.frameRate must be a finite number."},
		{"number bad string", fileLevel, "microdvd", data("frameRate", "25"), "The field formatData.microdvd.frameRate must be a finite number."},

		{"scalar good", cueLevel, "csv", data("columns", []any{"a", int64(1), 1.5, true, nil, math.Inf(-1)}), ""},
		{"scalar bad list", cueLevel, "csv", data("columns", []any{"a", []any{}}), "The field formatData.csv.columns[1] must be a string, a number, a boolean or null."},
		{"scalar bad object", cueLevel, "csv", data("columns", []any{data()}), "The field formatData.csv.columns[0] must be a string, a number, a boolean or null."},

		{"ttiBlock good", cueLevel, "stl", data("blocks", ttiList), ""},
		{"ttiBlock bad short", cueLevel, "stl", data("blocks", []any{hex256[1:]}), "The field formatData.stl.blocks[0] must be a TTI block of 256 hexadecimal digits."},
		{"ttiBlock bad digit", cueLevel, "stl", data("blocks", []any{hex256, "g" + hex256[1:]}), "The field formatData.stl.blocks[1] must be a TTI block of 256 hexadecimal digits."},
		{"ttiBlock good final LF", cueLevel, "stl", data("blocks", []any{hex256 + "\n"}), ""},
		{"ttiBlock bad 2 final LF", cueLevel, "stl", data("blocks", []any{hex256 + "\n\n"}), "The field formatData.stl.blocks[0] must be a TTI block of 256 hexadecimal digits."},
		{"ttiBlock bad final CR LF", cueLevel, "stl", data("blocks", []any{hex256 + "\r\n"}), "The field formatData.stl.blocks[0] must be a TTI block of 256 hexadecimal digits."},
		{"ttiBlock bad int", cueLevel, "stl", data("blocks", []any{int64(1)}), "The field formatData.stl.blocks[0] must be a TTI block of 256 hexadecimal digits."},

		{"timeBase good", cueLevel, "ffmeta-chapters", data("timeBase", "1/1000"), ""},
		{"timeBase good long", cueLevel, "ffmeta-chapters", data("timeBase", "1001/30000"), ""},
		{"timeBase bad zero", cueLevel, "ffmeta-chapters", data("timeBase", "0/1000"), `The field formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`},
		{"timeBase bad leading zero", cueLevel, "ffmeta-chapters", data("timeBase", "1/01"), `The field formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`},
		{"timeBase bad no slash", cueLevel, "ffmeta-chapters", data("timeBase", "1000"), `The field formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`},
		{"timeBase bad 2 slashes", cueLevel, "ffmeta-chapters", data("timeBase", "1/2/3"), `The field formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`},
		{"timeBase good final LF", cueLevel, "ffmeta-chapters", data("timeBase", "1/1000\n"), ""},
		{"timeBase bad 2 final LF", cueLevel, "ffmeta-chapters", data("timeBase", "1/1000\n\n"), `The field formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`},
		{"timeBase bad LF before the slash", cueLevel, "ffmeta-chapters", data("timeBase", "1\n/1000"), `The field formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`},
		{"timeBase bad int", cueLevel, "ffmeta-chapters", data("timeBase", int64(1)), `The field formatData.ffmeta-chapters.timeBase must be a time base such as "1/1000".`},

		{"range good low", fileLevel, "dcp", data("timeCodeRate", int64(1)), ""},
		{"range good high", fileLevel, "dcp", data("timeCodeRate", int64(1000)), ""},
		{"range bad low", fileLevel, "dcp", data("timeCodeRate", int64(0)), "The field formatData.dcp.timeCodeRate must be an integer from 1 to 1000."},
		{"range bad high", fileLevel, "dcp", data("timeCodeRate", int64(1001)), "The field formatData.dcp.timeCodeRate must be an integer from 1 to 1000."},
		{"range bad float", fileLevel, "dcp", data("timeCodeRate", 24.0), "The field formatData.dcp.timeCodeRate must be an integer from 1 to 1000."},

		{"oneOf good", fileLevel, "csv", csvData("delimiter", "\t"), ""},
		{"oneOf bad", fileLevel, "csv", data("delimiter", "|"), `The field formatData.csv.delimiter must be one of ",", ";", "\t".`},
		{"oneOf bad case", fileLevel, "dcp", data("dialect", "SMPTE"), `The field formatData.dcp.dialect must be one of "interop", "smpte".`},
		{"oneOf bad int", fileLevel, "mcc", data("frameRate", int64(24)), `The field formatData.mcc.frameRate must be one of "23.976", "24", "25", "29.97", "30", "50", "59.94", "60".`},

		{"list good list", fileLevel, "vtt", data("headerLines", []any{"a", "b"}), ""},
		{"list good object", fileLevel, "vtt", data("headerLines", data("x", "a", "12", "b")), ""},
		{"list good empty", fileLevel, "vtt", data("headerLines", []any{}), ""},
		{"list bad", fileLevel, "vtt", data("headerLines", "a"), "The field formatData.vtt.headerLines must be a list or an object."},
		{"list bad item", fileLevel, "vtt", data("headerLines", []any{"a", int64(1)}), "The field formatData.vtt.headerLines[1] must be a string."},
		{"list bad object item", fileLevel, "vtt", data("headerLines", data("x", int64(1))), "The field formatData.vtt.headerLines.x must be a string."},
		{"list bad number key item", fileLevel, "vtt", data("headerLines", data("12", int64(1))), "The field formatData.vtt.headerLines[12] must be a string."},
		{"list bad negative key item", fileLevel, "vtt", data("headerLines", data("-5", int64(1))), "The field formatData.vtt.headerLines[-5] must be a string."},
		{"list bad leading zero key item", fileLevel, "vtt", data("headerLines", data("012", int64(1))), "The field formatData.vtt.headerLines.012 must be a string."},

		{"names good", fileLevel, "ttml", data("namespaces", data("tts", "http://www.w3.org/ns/ttml#styling")), ""},
		{"names good empty list", fileLevel, "ttml", data("namespaces", []any{}), ""},
		{"names good empty object", fileLevel, "ttml", data("namespaces", data()), ""},
		{"names good leading zero", fileLevel, "ttml", data("namespaces", data("012", "a")), ""},
		{"names good plus", fileLevel, "ttml", data("namespaces", data("+5", "a")), ""},
		{"names good decimal", fileLevel, "ttml", data("namespaces", data("1.5", "a")), ""},
		{"names good minus zero", fileLevel, "ttml", data("namespaces", data("-0", "a")), ""},
		{"names good beyond int64", fileLevel, "ttml", data("namespaces", data("9223372036854775808", "a")), ""},
		{"names bad", fileLevel, "ttml", data("namespaces", "a"), "The field formatData.ttml.namespaces must be an object."},
		{"names bad number", fileLevel, "ttml", data("namespaces", data("a", "x", "12", "y")), "The field formatData.ttml.namespaces must be an object whose keys are names, not numbers."},
		{"names bad zero", fileLevel, "ttml", data("namespaces", data("0", "y")), "The field formatData.ttml.namespaces must be an object whose keys are names, not numbers."},
		{"names bad negative", fileLevel, "ttml", data("namespaces", data("-5", "y")), "The field formatData.ttml.namespaces must be an object whose keys are names, not numbers."},
		{"names bad list", fileLevel, "ttml", data("namespaces", []any{"a"}), "The field formatData.ttml.namespaces must be an object whose keys are names, not numbers."},
		{"names bad value", fileLevel, "ttml", data("namespaces", data("tts", int64(1))), "The field formatData.ttml.namespaces.tts must be a string."},

		{"keys good", fileLevel, "csv", csvData("roles", data("start", int64(0), "text", int64(999))), ""},
		{"keys good empty list", fileLevel, "csv", data("header", nil, "roles", []any{}, "width", int64(0)), ""},
		{"keys bad", fileLevel, "csv", data("header", nil, "roles", "x"), "The field formatData.csv.roles must be an object."},
		{"keys bad key", fileLevel, "csv", data("header", nil, "roles", data("start", int64(0), "color", int64(1))), "The field formatData.csv.roles must hold only the keys identifier, start, end, duration, speaker, text."},
		{"keys bad list", fileLevel, "csv", data("header", nil, "roles", []any{int64(0)}), "The field formatData.csv.roles must hold only the keys identifier, start, end, duration, speaker, text."},
		{"keys bad value", fileLevel, "csv", data("header", nil, "roles", data("text", int64(1000))), "The field formatData.csv.roles.text must be an integer from 0 to 999."},

		{"object good", cueLevel, "srt", data("coordinates", data("x1", int64(1), "x2", int64(2), "y1", int64(3), "y2", int64(4))), ""},
		{"object bad", cueLevel, "srt", data("coordinates", "x"), "The field formatData.srt.coordinates must be an object."},
		{"object bad list", cueLevel, "srt", data("coordinates", []any{int64(1)}), "The field formatData.srt.coordinates.x1 is missing."},
		{"object bad field", cueLevel, "srt", data("coordinates", data("x1", int64(1), "x2", "2")), "The field formatData.srt.coordinates.x2 must be an integer."},
		{"object optional field missing", cueLevel, "microdvd", data("lines", []any{data("codes", "", "color", nil, "tags", []any{})}), ""},
		{"object optional field bad", cueLevel, "microdvd", data("lines", []any{data("codes", "", "color", nil, "tags", []any{}, "otherCodes", int64(1))}), "The field formatData.microdvd.lines[0].otherCodes must be a string."},
		{"object list of objects", fileLevel, "dcp", data("loadFonts", []any{data("attributes", data("Id", "f1"))}), ""},
		{"object list of objects bad", fileLevel, "dcp", data("loadFonts", []any{data("text", int64(1))}), "The field formatData.dcp.loadFonts[0].text must be a string."},

		{"nullable nil", fileLevel, "sami", data("style", nil, "class", nil, "samiParam", nil), ""},
		{"nullable string", fileLevel, "sami", data("style", "P {}"), ""},
		{"nullable bad", fileLevel, "sami", data("class", int64(1)), "The field formatData.sami.class must be a string."},
		{"nullable int nil", fileLevel, "stl", data("firstSubtitleNumber", nil), ""},
		{"nullable int bad", fileLevel, "stl", data("firstSubtitleNumber", "1"), "The field formatData.stl.firstSubtitleNumber must be an integer."},
		{"nullable list nil", fileLevel, "ass", data("styleFormat", nil), ""},
		{"nullable list bad", fileLevel, "ass", data("styleFormat", "Name"), "The field formatData.ass.styleFormat must be a list or an object."},
		{"nullable number nil", fileLevel, "csv", csvData("frameRate", nil), ""},
		{"nullable list item nil", fileLevel, "ffmeta-chapters", data("tags", []any{nil, "a"}), ""},
		{"nullable list item bad", fileLevel, "ffmeta-chapters", data("tags", []any{nil, int64(1)}), "The field formatData.ffmeta-chapters.tags[1] must be a string."},
		{"null where not nullable", fileLevel, "vtt", data("header", nil), "The field formatData.vtt.header must be a string."},

		{"strings top level good", fileLevel, "mpsub", data("FORMAT", "TIME", "TITLE", "x"), ""},
		{"strings top level bad", fileLevel, "mpsub", data("FORMAT", int64(1)), "The field formatData.mpsub.FORMAT must be a string."},
		{"strings top level number key", fileLevel, "mpsub", data("12", nil), "The field formatData.mpsub[12] must be a string."},
	})
}

func TestSchemaRequiredFields(t *testing.T) {
	t.Parallel()

	microDVDLine := func(drop string) *model.Data {
		d := data("codes", "{y:i}", "color", nil, "tags", []any{"i"})
		d.Delete(drop)
		return data("lines", []any{d})
	}
	samiParagraph := func(drop string) *model.Data {
		d := data("attributes", data("Class", "ENCC"), "html", "Hello")
		d.Delete(drop)
		return data("paragraphs", []any{d})
	}
	coordinates := func(drop string) *model.Data {
		d := data("x1", int64(1), "x2", int64(2), "y1", int64(3), "y2", int64(4))
		d.Delete(drop)
		return data("coordinates", d)
	}
	stlComment := func(drop string) *model.Data {
		d := data("text", "note", "blocks", ttiList)
		d.Delete(drop)
		return data("comments", []any{d})
	}
	csvWithout := func(drop string) *model.Data {
		d := csvData()
		d.Delete(drop)
		return d
	}

	runSchemaCases(t, []schemaCase{
		{"all present", fileLevel, "csv", csvData(), ""},
		{"header", fileLevel, "csv", csvWithout("header"), "The field formatData.csv.header is missing."},
		{"roles", fileLevel, "csv", csvWithout("roles"), "The field formatData.csv.roles is missing."},
		{"width", fileLevel, "csv", csvWithout("width"), "The field formatData.csv.width is missing."},
		{"only optional fields", fileLevel, "csv", data("delimiter", ";"), "The field formatData.csv.header is missing."},

		{"all present", cueLevel, "microdvd", microDVDLine(""), ""},
		{"codes", cueLevel, "microdvd", microDVDLine("codes"), "The field formatData.microdvd.lines[0].codes is missing."},
		{"color", cueLevel, "microdvd", microDVDLine("color"), "The field formatData.microdvd.lines[0].color is missing."},
		{"tags", cueLevel, "microdvd", microDVDLine("tags"), "The field formatData.microdvd.lines[0].tags is missing."},

		{"all present", cueLevel, "sami", samiParagraph(""), ""},
		{"attributes", cueLevel, "sami", samiParagraph("attributes"), "The field formatData.sami.paragraphs[0].attributes is missing."},
		{"html", cueLevel, "sami", samiParagraph("html"), "The field formatData.sami.paragraphs[0].html is missing."},

		{"all present", cueLevel, "srt", coordinates(""), ""},
		{"x1", cueLevel, "srt", coordinates("x1"), "The field formatData.srt.coordinates.x1 is missing."},
		{"x2", cueLevel, "srt", coordinates("x2"), "The field formatData.srt.coordinates.x2 is missing."},
		{"y1", cueLevel, "srt", coordinates("y1"), "The field formatData.srt.coordinates.y1 is missing."},
		{"y2", cueLevel, "srt", coordinates("y2"), "The field formatData.srt.coordinates.y2 is missing."},

		{"all present", fileLevel, "stl", stlComment(""), ""},
		{"text", fileLevel, "stl", stlComment("text"), "The field formatData.stl.comments[0].text is missing."},
		{"blocks", fileLevel, "stl", stlComment("blocks"), "The field formatData.stl.comments[0].blocks is missing."},
	})
}

// TestFileSchemaFields checks one wrong value of each field of the file schema table.
func TestFileSchemaFields(t *testing.T) {
	t.Parallel()

	const f = fileLevel
	runSchemaCases(t, []schemaCase{
		{"sectionOrder", f, "ass", data("sectionOrder", "x"), "The field formatData.ass.sectionOrder must be a list or an object."},
		{"scriptInfoComments", f, "ass", data("scriptInfoComments", []any{true}), "The field formatData.ass.scriptInfoComments[0] must be a string."},
		{"scriptInfo", f, "ass", data("scriptInfo", data("Title", int64(1))), "The field formatData.ass.scriptInfo.Title must be a string."},
		{"stylesSection", f, "ass", data("stylesSection", int64(4)), "The field formatData.ass.stylesSection must be a string."},
		{"styleFormat", f, "ass", data("styleFormat", []any{nil}), "The field formatData.ass.styleFormat[0] must be a string."},
		{"styles", f, "ass", data("styles", []any{"Default"}), "The field formatData.ass.styles[0] must be a list or an object."},
		{"styles item", f, "ass", data("styles", []any{data("Name", false)}), "The field formatData.ass.styles[0].Name must be a string."},
		{"eventFormat", f, "ass", data("eventFormat", "Layer"), "The field formatData.ass.eventFormat must be a list or an object."},
		{"commentEvents", f, "ass", data("commentEvents", []any{[]any{"x"}}), "The field formatData.ass.commentEvents[0] must be an object whose keys are names, not numbers."},
		{"commentEvents value", f, "ass", data("commentEvents", []any{data("Text", int64(1))}), "The field formatData.ass.commentEvents[0].Text must be a string."},
		{"sections", f, "ass", data("sections", []any{"x"}), "The field formatData.ass.sections[0] must be a list or an object."},
		{"table order", f, "ass", data("styles", "x", "sectionOrder", "x"), "The field formatData.ass.sectionOrder must be a list or an object."},

		{"header", f, "csv", data("header", []any{int64(1)}), "The field formatData.csv.header[0] must be a string."},
		{"width", f, "csv", data("header", nil, "roles", data(), "width", int64(1001)), "The field formatData.csv.width must be an integer from 0 to 1000."},
		{"timeFormat", f, "csv", csvData("timeFormat", "frames"), `The field formatData.csv.timeFormat must be one of "seconds", "hh:mm:ss.mmm", "hh:mm:ss,mmm", "hh:mm:ss:ff", "milliseconds".`},
		{"timeFormat good", f, "csv", csvData("timeFormat", "hh:mm:ss:ff"), ""},
		{"frameRate", f, "csv", csvData("frameRate", "25"), "The field formatData.csv.frameRate must be a finite number."},
		{"quotedColumns", f, "csv", csvData("quotedColumns", []any{int64(0), int64(1000)}), "The field formatData.csv.quotedColumns[1] must be an integer from 0 to 999."},

		{"idTags", f, "lrc", data("idTags", data("ti", int64(1))), "The field formatData.lrc.idTags.ti must be a string."},
		{"spacedWordTimes", f, "lrc", data("spacedWordTimes", int64(1)), "The field formatData.lrc.spacedWordTimes must be a boolean."},

		{"frameRateLine", f, "microdvd", data("frameRateLine", "true"), "The field formatData.microdvd.frameRateLine must be a boolean."},

		{"header", f, "qttext", data("header", "x"), "The field formatData.qttext.header must be a list or an object."},
		{"pairedTimestamps", f, "qttext", data("pairedTimestamps", nil), "The field formatData.qttext.pairedTimestamps must be a boolean."},

		{"style", f, "sami", data("style", []any{}), "The field formatData.sami.style must be a string."},
		{"samiParam", f, "sami", data("samiParam", true), "The field formatData.sami.samiParam must be a string."},

		{"header", f, "subviewer", data("header", []any{int64(1)}), "The field formatData.subviewer.header[0] must be a string."},
		{"style", f, "subviewer", data("style", int64(1)), "The field formatData.subviewer.style must be a string."},

		{"namespace", f, "ttml", data("namespace", nil), "The field formatData.ttml.namespace must be a string."},
		{"head", f, "ttml", data("head", int64(1)), "The field formatData.ttml.head must be a string."},
		{"attributes", f, "ttml", data("attributes", data("1", "x")), "The field formatData.ttml.attributes must be an object whose keys are names, not numbers."},
		{"body", f, "ttml", data("body", data("style", int64(1))), "The field formatData.ttml.body.style must be a string."},

		{"regions", f, "vtt", data("regions", []any{"id:fred"}), "The field formatData.vtt.regions[0] must be a list or an object."},
		{"regions item", f, "vtt", data("regions", []any{data("id", int64(1))}), "The field formatData.vtt.regions[0].id must be a string."},
		{"styles", f, "vtt", data("styles", "::cue {}"), "The field formatData.vtt.styles must be a list or an object."},

		{"dialect good", f, "dcp", data("dialect", "interop"), ""},
		{"namespace", f, "dcp", data("namespace", int64(1)), "The field formatData.dcp.namespace must be a string."},
		{"prefix", f, "dcp", data("prefix", int64(1)), "The field formatData.dcp.prefix must be a string."},
		{"attributes", f, "dcp", data("attributes", "x"), "The field formatData.dcp.attributes must be an object."},
		{"header", f, "dcp", data("header", data("Language", int64(1))), "The field formatData.dcp.header.Language must be a string."},
		{"loadFonts", f, "dcp", data("loadFonts", []any{"x"}), "The field formatData.dcp.loadFonts[0] must be an object."},
		{"loadFonts attributes", f, "dcp", data("loadFonts", []any{data("attributes", []any{"x"})}), "The field formatData.dcp.loadFonts[0].attributes must be an object whose keys are names, not numbers."},
		{"font", f, "dcp", data("font", data("Size", int64(42))), "The field formatData.dcp.font.Size must be a string."},

		{"frameRate", f, "itt", data("frameRate", int64(30)), "The field formatData.itt.frameRate must be a string."},
		{"frameRateMultiplier", f, "itt", data("frameRateMultiplier", int64(1)), "The field formatData.itt.frameRateMultiplier must be a string."},

		{"version", f, "mcc", data("version", "3.0"), `The field formatData.mcc.version must be one of "1.0", "2.0".`},
		{"frameRate good", f, "mcc", data("frameRate", "29.97"), ""},
		{"dropFrame", f, "mcc", data("dropFrame", int64(0)), "The field formatData.mcc.dropFrame must be a boolean."},
		{"header", f, "mcc", data("header", []any{int64(1)}), "The field formatData.mcc.header[0] must be a string."},

		{"frameRate", f, "spruce-stl", data("frameRate", nil), "The field formatData.spruce-stl.frameRate must be a finite number."},
		{"header", f, "spruce-stl", data("header", "x"), "The field formatData.spruce-stl.header must be a list or an object."},
		{"separator", f, "spruce-stl", data("separator", int64(1)), "The field formatData.spruce-stl.separator must be a string."},
		{"lineBreak", f, "spruce-stl", data("lineBreak", int64(1)), "The field formatData.spruce-stl.lineBreak must be a string."},

		{"gsi", f, "stl", data("gsi", data("CPN", int64(850))), "The field formatData.stl.gsi.CPN must be a string."},
		{"startOfProgrammeSubtracted", f, "stl", data("startOfProgrammeSubtracted", "no"), "The field formatData.stl.startOfProgrammeSubtracted must be a boolean."},
		{"comments", f, "stl", data("comments", "x"), "The field formatData.stl.comments must be a list or an object."},
		{"comments blocks", f, "stl", data("comments", []any{data("text", "", "blocks", []any{"00"})}), "The field formatData.stl.comments[0].blocks[0] must be a TTI block of 256 hexadecimal digits."},

		{"tags", f, "ffmeta-chapters", data("tags", "x"), "The field formatData.ffmeta-chapters.tags must be a list or an object."},
		{"streams", f, "ffmeta-chapters", data("streams", []any{data("title", int64(1))}), "The field formatData.ffmeta-chapters.streams[0].title must be a string."},
	})
}

// TestCueSchemaFields checks one wrong value of each field of the cue schema table.
func TestCueSchemaFields(t *testing.T) {
	t.Parallel()

	const c = cueLevel
	runSchemaCases(t, []schemaCase{
		{"fields", c, "ass", data("fields", data("Layer", int64(0))), "The field formatData.ass.fields.Layer must be a string."},
		{"text", c, "ass", data("text", int64(1)), "The field formatData.ass.text must be a string."},

		{"columns", c, "csv", data("columns", "x"), "The field formatData.csv.columns must be a list or an object."},

		{"endLine", c, "lrc", data("endLine", "x"), "The field formatData.lrc.endLine must be a boolean."},

		{"lines", c, "microdvd", data("lines", "x"), "The field formatData.microdvd.lines must be a list or an object."},
		{"lines codes", c, "microdvd", data("lines", []any{data("codes", int64(1))}), "The field formatData.microdvd.lines[0].codes must be a string."},
		{"lines color", c, "microdvd", data("lines", []any{data("codes", "", "color", int64(1))}), "The field formatData.microdvd.lines[0].color must be a string."},
		{"lines tags", c, "microdvd", data("lines", []any{data("codes", "", "color", "$0000FF", "tags", []any{int64(1)})}), "The field formatData.microdvd.lines[0].tags[0] must be a string."},

		{"source", c, "qttext", data("source", []any{int64(1)}), "The field formatData.qttext.source[0] must be a string."},

		{"paragraphs", c, "sami", data("paragraphs", []any{"x"}), "The field formatData.sami.paragraphs[0] must be an object."},
		{"paragraphs attributes", c, "sami", data("paragraphs", []any{data("attributes", data("Class", int64(1)))}), "The field formatData.sami.paragraphs[0].attributes.Class must be a string."},
		{"paragraphs html", c, "sami", data("paragraphs", []any{data("attributes", data(), "html", int64(1))}), "The field formatData.sami.paragraphs[0].html must be a string."},

		{"coordinates y2", c, "srt", data("coordinates", data("x1", int64(1), "x2", int64(2), "y1", int64(3), "y2", 4.0)), "The field formatData.srt.coordinates.y2 must be an integer."},

		{"attributes", c, "ttml", data("attributes", "x"), "The field formatData.ttml.attributes must be an object."},
		{"div", c, "ttml", data("div", data("region", int64(1))), "The field formatData.ttml.div.region must be a string."},

		{"vertical", c, "vtt", data("vertical", int64(1)), "The field formatData.vtt.vertical must be a string."},
		{"line", c, "vtt", data("line", int64(0)), "The field formatData.vtt.line must be a string."},
		{"position", c, "vtt", data("position", 0.3), "The field formatData.vtt.position must be a string."},
		{"size", c, "vtt", data("size", nil), "The field formatData.vtt.size must be a string."},
		{"align", c, "vtt", data("align", true), "The field formatData.vtt.align must be a string."},
		{"region", c, "vtt", data("region", []any{}), "The field formatData.vtt.region must be a string."},
		{"alignment is unchecked", c, "vtt", data("alignment", "7"), ""},

		{"png", c, model.ImageKey, data("png", int64(1)), "The field formatData.image.png must be a string."},
		{"x", c, model.ImageKey, data("x", 1.0), "The field formatData.image.x must be an integer."},
		{"y", c, model.ImageKey, data("y", "1"), "The field formatData.image.y must be an integer."},
		{"width", c, model.ImageKey, data("width", nil), "The field formatData.image.width must be an integer."},
		{"height", c, model.ImageKey, data("height", true), "The field formatData.image.height must be an integer."},
		{"screenWidth", c, model.ImageKey, data("screenWidth", "720"), "The field formatData.image.screenWidth must be an integer."},
		{"screenHeight", c, model.ImageKey, data("screenHeight", "576"), "The field formatData.image.screenHeight must be an integer."},
		{"forced", c, model.ImageKey, data("forced", int64(1)), "The field formatData.image.forced must be a boolean."},

		{"subtitle", c, "dcp", data("subtitle", data("SpotNumber", int64(1))), "The field formatData.dcp.subtitle.SpotNumber must be a string."},
		{"texts", c, "dcp", data("texts", []any{data("VPosition", int64(10))}), "The field formatData.dcp.texts[0].VPosition must be a string."},

		{"commands", c, "spruce-stl", data("commands", "x"), "The field formatData.spruce-stl.commands must be a list or an object."},

		{"cumulativeStatus", c, "stl", data("cumulativeStatus", "0"), "The field formatData.stl.cumulativeStatus must be an integer."},
		{"verticalPosition", c, "stl", data("verticalPosition", 1.5), "The field formatData.stl.verticalPosition must be an integer."},
		{"justificationCode", c, "stl", data("justificationCode", nil), "The field formatData.stl.justificationCode must be an integer."},
		{"blocks", c, "stl", data("blocks", "x"), "The field formatData.stl.blocks must be a list or an object."},

		{"tags", c, "ffmeta-chapters", data("tags", []any{nil}), "The field formatData.ffmeta-chapters.tags[0] must be a string."},
	})
}

func TestSchemaLevelsAreApart(t *testing.T) {
	t.Parallel()

	runSchemaCases(t, []schemaCase{
		{"file field at cue level", cueLevel, "scc", data("dropFrame", "yes"), ""},
		{"cue field at file level", fileLevel, "srt", data("coordinates", "x"), ""},
		{"cue mpsub is free", cueLevel, "mpsub", data("x", int64(1)), ""},
		{"file image is free", fileLevel, model.ImageKey, data("png", int64(1)), ""},
	})
}

func TestSchemaPassesUncheckedData(t *testing.T) {
	t.Parallel()

	tests := make([]schemaCase, 0, 32)
	for _, key := range []string{"aws-transcribe", "assemblyai", "deepgram", "google-speech", "whisper", "youtube", "podcast-chapters", "podcast-transcript", "pac", "cavena890", "srt", "sbv", ""} {
		tests = append(tests, schemaCase{"any data", fileLevel, key, data("dropFrame", "yes", "x", []any{data("1", nil)}), ""})
	}
	for _, key := range []string{"aws-transcribe", "whisper", "youtube", "pac", "cavena890", "scc"} {
		tests = append(tests, schemaCase{"any data", cueLevel, key, data("coordinates", "x", "lines", int64(1)), ""})
	}
	tests = append(tests,
		schemaCase{"field outside the schema", fileLevel, "scc", data("dropFrame", true, "note", int64(5)), ""},
		schemaCase{"alignment outside the schema", cueLevel, "ass", data("alignment", "x", "lines", int64(1)), ""},
		schemaCase{"empty data with required fields", fileLevel, "csv", data(), ""},
		schemaCase{"nil data", cueLevel, "srt", nil, ""},
	)
	runSchemaCases(t, tests)
}

func TestSchemaRejectsGoTypesOutsideTheValueSet(t *testing.T) {
	t.Parallel()

	runSchemaCases(t, []schemaCase{
		{"int", fileLevel, "srt", data("x", 5), "The field formatData.srt.x has the unsupported Go type int."},
		{"int in a checked field", cueLevel, "stl", data("subtitleGroupNumber", 5), "The field formatData.stl.subtitleGroupNumber has the unsupported Go type int."},
		{"float32 in a list", cueLevel, "whisper", data("words", []any{1.0, float32(1)}), "The field formatData.whisper.words[1] has the unsupported Go type float32."},
		{"map in an object", fileLevel, "ass", data("scriptInfo", data("Title", map[string]any{})), "The field formatData.ass.scriptInfo.Title has the unsupported Go type map[string]interface {}."},
		{"string slice", fileLevel, "vtt", data("headerLines", []string{"a"}), "The field formatData.vtt.headerLines has the unsupported Go type []string."},
		{"Data value", fileLevel, "vtt", data("regions", []any{*data()}), "The field formatData.vtt.regions[0] has the unsupported Go type model.Data."},
		{"number key", fileLevel, "vtt", data("12", int32(1)), "The field formatData.vtt[12] has the unsupported Go type int32."},
		{"before a schema problem", fileLevel, "scc", data("dropFrame", "yes", "x", uint8(1)), "The field formatData.scc.x has the unsupported Go type uint8."},
		{"nil Data value is an empty object", fileLevel, "ttml", data("namespaces", (*model.Data)(nil)), ""},
	})
}

func TestSchemaRejectsCycles(t *testing.T) {
	t.Parallel()

	d := data("a", "x")
	inner := data()
	inner.Set("self", inner)
	d.Set("b", []any{inner})
	assert.Equal(t, "The field formatData.srt.b[0].self holds itself.", setFormatData(t, fileLevel, "srt", d))

	list := []any{nil}
	list[0] = list
	assert.Equal(t, "The field formatData.srt.l[0] holds itself.", setFormatData(t, cueLevel, "srt", data("l", list)))

	sub := []any{"x", nil}
	sub[1] = sub[:1]
	assert.Empty(t, setFormatData(t, cueLevel, "srt", data("l", sub)))

	shared := data("x", "1")
	assert.Empty(t, setFormatData(t, cueLevel, "srt", data("a", shared, "b", []any{shared, shared})))
}
