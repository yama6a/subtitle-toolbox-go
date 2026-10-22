package ass

import (
	"regexp"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

const styleOption = "WriteOptions.ASS.Style"

var stylePairRegexp = regexp.MustCompile(`^[\t\n\v\f\r ]*([^=\t\n\v\f\r ]+)[\t\n\v\f\r ]*=[\t\n\v\f\r ]*([^\x00-\x1f]*?)[\t\n\v\f\r ]*$`)

// parseStyle returns the values of the Style option by the ASS style field name, in pair order. A later pair for
// the same field wins.
func parseStyle(style string) (record, error) {
	fields := assStyleFields[1:]
	var values record
	for pair := range strings.SplitSeq(style, ",") {
		m := stylePairRegexp.FindStringSubmatch(pair)
		if m == nil {
			return record{}, &model.InvalidArgumentError{
				Option: styleOption,
				Msg:    `The ASS style "` + style + `" must hold Field=Value pairs such as Fontsize=48, got "` + pair + `".`,
			}
		}
		field, ok := findName(fields, m[1])
		if !ok {
			return record{}, &model.InvalidArgumentError{
				Option: styleOption,
				Msg:    `The ASS style field "` + m[1] + `" does not exist. Use one of: ` + strings.Join(fields, ", ") + ".",
			}
		}
		values.set(field, m[2])
	}
	return values, nil
}

func findName(names []string, name string) (string, bool) {
	i := slices.IndexFunc(names, func(n string) bool { return equalFold(n, name) })
	if i < 0 {
		return "", false
	}
	return names[i], true
}

// applyStyle sets the values of the Style option in each style named Default, and adds a Default style when the
// file has none. A file without a styles section gets one before [Events].
func (f *fileData) applyStyle(style string) error {
	if !f.hasStylesSection {
		f.addStylesSection()
	}
	if f.styleFormat == nil {
		f.styleFormat = assStyleFields
	}
	values, err := f.styleValues(style)
	if err != nil {
		return err
	}
	styles := make([]record, 0, len(f.styles)+1)
	found := false
	for _, s := range f.styles {
		if isDefaultStyle(&s) {
			s = replaced(s, values)
			found = true
		}
		styles = append(styles, s)
	}
	if !found {
		styles = append([]record{replaced(f.newDefaultStyle(), values)}, styles...)
	}
	f.styles = styles
	return nil
}

func (f *fileData) addStylesSection() {
	f.stylesSection, f.styleFormat = assStylesSection, assStyleFields
	if isSSA(f.scriptInfo, "") {
		f.stylesSection, f.styleFormat = ssaStylesSection, ssaStyleFields
	}
	f.hasStylesSection = true
	at := slices.IndexFunc(f.sectionOrder, func(n string) bool { return equalFold(n, eventsSection) })
	if at < 0 {
		at = len(f.sectionOrder)
	}
	f.sectionOrder = slices.Insert(slices.Clone(f.sectionOrder), at, f.stylesSection)
}

// styleValues returns the values of the Style option by the field names of the Format line of the styles.
func (f *fileData) styleValues(style string) (record, error) {
	parsed, err := parseStyle(style)
	if err != nil {
		return record{}, err
	}
	var values record
	for i, field := range parsed.names {
		name, ok := findName(f.styleFormat, field)
		if !ok {
			return record{}, &model.UnwritableError{
				Format:   model.ASS,
				CueIndex: -1,
				Reason:   "The Format line of the styles has no field " + field + ". It holds: " + strings.Join(f.styleFormat, ", ") + ".",
			}
		}
		values.set(name, parsed.values[i])
	}
	return values, nil
}

// isDefaultStyle reports whether the Name of s is Default in any case, with leading stars ignored.
func isDefaultStyle(s *record) bool {
	nameField, ok := findName(s.names, "Name")
	if !ok {
		return false
	}
	name, _ := s.get(nameField)
	return equalFold(strings.TrimLeft(phpTrim(name), "*"), "Default")
}

// newDefaultStyle returns a Default style with the fields of the Format line. Each field takes the value of the
// default ASS style of that name, or 0.
func (f *fileData) newDefaultStyle() record {
	var added record
	for _, field := range f.styleFormat {
		v := "0"
		if i := slices.IndexFunc(assStyleFields, func(n string) bool { return equalFold(n, field) }); i >= 0 {
			v = defaultStyle[i]
		}
		added.set(field, v)
	}
	return added
}

// replaced returns a copy of r with the values set, as PHP array_replace does.
func replaced(r, values record) record {
	out := record{names: slices.Clone(r.names), values: slices.Clone(r.values)}
	for i, n := range values.names {
		out.set(n, values.values[i])
	}
	return out
}
