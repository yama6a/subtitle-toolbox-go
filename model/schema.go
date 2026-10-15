package model

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

// valueType is one type of the type language of the format data schema.
type valueType struct {
	desc     string
	accepts  func(v any) bool
	inner    func(v any, path string) string // checks the values of a list or object, nil for other types
	nullable bool
}

type schemaField struct {
	name     string
	required bool
	typ      valueType
}

func is[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func isContainer(v any) bool { return is[[]any](v) || is[*Data](v) }

func str() valueType { return valueType{desc: "a string", accepts: is[string]} }

func integer() valueType { return valueType{desc: "an integer", accepts: is[int64]} }

func boolean() valueType { return valueType{desc: "a boolean", accepts: is[bool]} }

func number() valueType {
	return valueType{desc: "a finite number", accepts: func(v any) bool {
		f, ok := v.(float64)
		return is[int64](v) || ok && !math.IsInf(f, 0) && !math.IsNaN(f)
	}}
}

func scalar() valueType {
	return valueType{desc: "a string, a number, a boolean or null", accepts: func(v any) bool {
		return v == nil || is[string](v) || is[int64](v) || is[float64](v) || is[bool](v)
	}}
}

func ttiBlock() valueType {
	const digits = 256
	return valueType{desc: "a TTI block of 256 hexadecimal digits", accepts: func(v any) bool {
		s, ok := v.(string)
		return ok && len(s) == digits && strings.Trim(s, "0123456789abcdefABCDEF") == ""
	}}
}

func timeBase() valueType {
	return valueType{desc: `a time base such as "1/1000"`, accepts: func(v any) bool {
		s, _ := v.(string)
		num, den, found := strings.Cut(s, "/")
		return found && isPositiveDecimal(num) && isPositiveDecimal(den)
	}}
}

func isPositiveDecimal(s string) bool {
	return s != "" && s[0] != '0' && strings.Trim(s, "0123456789") == ""
}

func rangeOf(lo, hi int64) valueType {
	return valueType{desc: fmt.Sprintf("an integer from %d to %d", lo, hi), accepts: func(v any) bool {
		n, ok := v.(int64)
		return ok && n >= lo && n <= hi
	}}
}

func oneOf(values ...string) valueType {
	quoted := make([]string, len(values))
	for i, s := range values {
		quoted[i] = strconv.Quote(s)
	}
	return valueType{desc: "one of " + strings.Join(quoted, ", "), accepts: func(v any) bool {
		s, ok := v.(string)
		return ok && slices.Contains(values, s)
	}}
}

func nullable(t valueType) valueType {
	t.nullable = true
	return t
}

func strs() valueType { return listOf(str()) }

func attributes() valueType { return namesOf(str()) }

func listOf(elem valueType) valueType {
	return valueType{desc: "a list or an object", accepts: isContainer, inner: func(v any, path string) string {
		vals, _ := items(v)
		for i, item := range vals {
			if msg := elem.check(item, childPath(v, path, i)); msg != "" {
				return msg
			}
		}
		return ""
	}}
}

func namesOf(elem valueType) valueType {
	return namedValues(elem, func(key string) bool { return !isIntKey(key) },
		func(path string) string {
			return "The field " + path + " must be an object whose keys are names, not numbers."
		})
}

func keysOf(names []string, elem valueType) valueType {
	return namedValues(elem, func(key string) bool { return slices.Contains(names, key) },
		func(path string) string {
			return "The field " + path + " must hold only the keys " + strings.Join(names, ", ") + "."
		})
}

// namedValues is an object whose keys pass allowed. A list has the keys 0, 1 and so on, which no name allows,
// so only an empty list passes.
func namedValues(elem valueType, allowed func(string) bool, badKey func(path string) string) valueType {
	return valueType{desc: "an object", accepts: isContainer, inner: func(v any, path string) string {
		d, ok := v.(*Data)
		if !ok {
			if l, _ := v.([]any); len(l) > 0 {
				return badKey(path)
			}
			return ""
		}
		for _, k := range d.Keys() {
			if !allowed(k) {
				return badKey(path)
			}
			if msg := elem.check(d.values[k], path+"."+k); msg != "" {
				return msg
			}
		}
		return ""
	}}
}

func objectOf(fields ...schemaField) valueType {
	return valueType{desc: "an object", accepts: isContainer, inner: func(v any, path string) string {
		d, _ := v.(*Data)
		for _, f := range fields {
			item, ok := d.Get(f.name)
			if !ok {
				if f.required {
					return "The field " + path + "." + f.name + " is missing."
				}
				continue
			}
			if msg := f.typ.check(item, path+"."+f.name); msg != "" {
				return msg
			}
		}
		return ""
	}}
}

func optional(name string, t valueType) schemaField { return schemaField{name: name, typ: t} }

func required(name string, t valueType) schemaField {
	return schemaField{name: name, required: true, typ: t}
}

func (t valueType) check(v any, path string) string {
	switch {
	case t.nullable && v == nil:
		return ""
	case !t.accepts(v):
		return "The field " + path + " must be " + t.desc + "."
	case t.inner != nil:
		return t.inner(v, path)
	default:
		return ""
	}
}

func checkFormatData(schema map[string]valueType, key string, d *Data) error {
	path := "formatData." + key
	if msg := checkGoTypes(d, path, nil); msg != "" {
		return invalidArgument(msg)
	}
	t, ok := schema[key]
	if !ok || d.Len() == 0 {
		return nil
	}
	if msg := t.check(d, path); msg != "" {
		return invalidArgument(msg)
	}
	return nil
}

// checkGoTypes returns a message for the first value outside the closed value set of Data, or for a list
// or object that contains itself, which would make Clone run without end. Open holds the lists and objects
// on the way to v.
func checkGoTypes(v any, path string, open []any) string {
	var vals []any
	switch v := v.(type) {
	case nil, bool, int64, float64, string:
		return ""
	case []any:
		if len(v) == 0 {
			return ""
		}
		vals = v
	case *Data:
		if v.Len() == 0 {
			return ""
		}
		vals, _ = items(v)
	default:
		return fmt.Sprintf("The field %s has the unsupported Go type %T.", path, v)
	}

	id := identity(v)
	if slices.Contains(open, id) {
		return "The field " + path + " holds itself."
	}
	open = append(open, id)
	for i, item := range vals {
		if msg := checkGoTypes(item, childPath(v, path, i), open); msg != "" {
			return msg
		}
	}
	return ""
}

// identity returns a comparable value that is the same for a list or object and the copies of its header.
func identity(v any) any {
	if l, ok := v.([]any); ok {
		return &l[0]
	}
	return v
}

// childPath returns the path of item i of the list or object v at path.
func childPath(v any, path string, i int) string {
	d, ok := v.(*Data)
	if !ok {
		return path + "[" + strconv.Itoa(i) + "]"
	}
	return keyPath(path, d.keys[i])
}

// keyPath returns the path of the object key at path. A key that PHP turns into an integer gets the list form.
func keyPath(path, key string) string {
	if isIntKey(key) {
		return path + "[" + key + "]"
	}
	return path + "." + key
}

// isIntKey reports whether PHP turns the array key into an integer: a canonical decimal int64.
func isIntKey(key string) bool {
	n, err := strconv.ParseInt(key, 10, 64)
	return err == nil && strconv.FormatInt(n, 10) == key
}

const csvMaxColumns = 1000

// fileSchema holds the file format data fields that writers read, by format data key.
//
//nolint:gochecknoglobals // lookup table of the type checks
var fileSchema = map[string]valueType{
	"ass": objectOf(
		optional("sectionOrder", strs()),
		optional("scriptInfoComments", strs()),
		optional("scriptInfo", strs()),
		optional("stylesSection", nullable(str())),
		optional("styleFormat", nullable(strs())),
		optional("styles", listOf(strs())),
		optional("eventFormat", strs()),
		optional("commentEvents", listOf(attributes())),
		optional("sections", listOf(strs())),
	),
	"csv": objectOf(
		optional("delimiter", oneOf(",", ";", "\t")),
		required("header", nullable(strs())),
		required("roles", keysOf([]string{"identifier", "start", "end", "duration", "speaker", "text"}, rangeOf(0, csvMaxColumns-1))),
		required("width", rangeOf(0, csvMaxColumns)),
		optional("timeFormat", oneOf("seconds", "hh:mm:ss.mmm", "hh:mm:ss,mmm", "hh:mm:ss:ff", "milliseconds")),
		optional("frameRate", nullable(number())),
		optional("quotedColumns", listOf(rangeOf(0, csvMaxColumns-1))),
	),
	"dcp": objectOf(
		optional("dialect", oneOf("interop", "smpte")),
		optional("namespace", str()),
		optional("prefix", str()),
		optional("timeCodeRate", rangeOf(1, 1000)),
		optional("attributes", attributes()),
		optional("header", attributes()),
		optional("loadFonts", listOf(objectOf(optional("attributes", attributes()), optional("text", str())))),
		optional("font", attributes()),
	),
	"ffmeta-chapters": objectOf(
		optional("tags", listOf(nullable(str()))),
		optional("streams", listOf(strs())),
	),
	"itt": objectOf(
		optional("frameRate", str()),
		optional("frameRateMultiplier", str()),
	),
	"lrc": objectOf(
		optional("idTags", strs()),
		optional("spacedWordTimes", boolean()),
	),
	"mcc": objectOf(
		optional("version", oneOf("1.0", "2.0")),
		optional("frameRate", oneOf("23.976", "24", "25", "29.97", "30", "50", "59.94", "60")),
		optional("dropFrame", boolean()),
		optional("header", strs()),
	),
	"microdvd": objectOf(
		optional("frameRate", number()),
		optional("frameRateLine", boolean()),
	),
	"mpsub": strs(),
	"qttext": objectOf(
		optional("header", strs()),
		optional("pairedTimestamps", boolean()),
	),
	"sami": objectOf(
		optional("style", nullable(str())),
		optional("class", nullable(str())),
		optional("samiParam", nullable(str())),
	),
	"scc": objectOf(
		optional("dropFrame", boolean()),
	),
	"spruce-stl": objectOf(
		optional("frameRate", number()),
		optional("header", strs()),
		optional("separator", str()),
		optional("lineBreak", str()),
	),
	"stl": objectOf(
		optional("gsi", strs()),
		optional("startOfProgrammeSubtracted", boolean()),
		optional("firstSubtitleNumber", nullable(integer())),
		optional("comments", listOf(objectOf(required("text", str()), required("blocks", listOf(ttiBlock()))))),
	),
	"subviewer": objectOf(
		optional("header", strs()),
		optional("style", nullable(str())),
	),
	"ttml": objectOf(
		optional("namespace", str()),
		optional("namespaces", attributes()),
		optional("head", str()),
		optional("attributes", attributes()),
		optional("body", attributes()),
	),
	"vtt": objectOf(
		optional("header", str()),
		optional("headerLines", strs()),
		optional("regions", listOf(strs())),
		optional("styles", strs()),
	),
}

// cueSchema holds the cue format data fields that writers read, by format data key.
//
//nolint:gochecknoglobals // lookup table of the type checks
var cueSchema = map[string]valueType{
	"ass": objectOf(
		optional("fields", attributes()),
		optional("text", str()),
	),
	"csv": objectOf(
		optional("columns", listOf(scalar())),
	),
	"dcp": objectOf(
		optional("subtitle", attributes()),
		optional("texts", listOf(attributes())),
	),
	"ffmeta-chapters": objectOf(
		optional("timeBase", timeBase()),
		optional("tags", strs()),
	),
	ImageKey: objectOf(
		optional("png", str()),
		optional("x", integer()),
		optional("y", integer()),
		optional("width", integer()),
		optional("height", integer()),
		optional("screenWidth", integer()),
		optional("screenHeight", integer()),
		optional("forced", boolean()),
	),
	"lrc": objectOf(
		optional("endLine", boolean()),
	),
	"microdvd": objectOf(
		optional("lines", listOf(objectOf(
			required("codes", str()),
			required("color", nullable(str())),
			required("tags", strs()),
			optional("otherCodes", str()),
		))),
	),
	"qttext": objectOf(
		optional("source", strs()),
	),
	"sami": objectOf(
		optional("paragraphs", listOf(objectOf(required("attributes", attributes()), required("html", str())))),
	),
	"spruce-stl": objectOf(
		optional("commands", strs()),
	),
	"srt": objectOf(
		optional("coordinates", objectOf(
			required("x1", integer()),
			required("x2", integer()),
			required("y1", integer()),
			required("y2", integer()),
		)),
	),
	"stl": objectOf(
		optional("subtitleGroupNumber", integer()),
		optional("cumulativeStatus", integer()),
		optional("verticalPosition", integer()),
		optional("justificationCode", integer()),
		optional("blocks", listOf(ttiBlock())),
	),
	"ttml": objectOf(
		optional("attributes", attributes()),
		optional("div", attributes()),
	),
	"vtt": objectOf(
		optional("vertical", str()),
		optional("line", str()),
		optional("position", str()),
		optional("size", str()),
		optional("align", str()),
		optional("region", str()),
	),
}
