package itt

import (
	"io"
	"slices"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/ttml"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/ttmlwrite"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

//nolint:gochecknoglobals // the timing parameters that the itt format data keeps, in this order
var timingParameters = []string{"timeBase", "frameRate", "frameRateMultiplier", "dropMode"}

// Read reads an iTT file as TTML and keeps the timing parameters of the root element in the itt format data. The
// result without the itt data equals the result of ttml.Read. A parameter that the root names twice, such as
// ttp:frameRate and p2:frameRate with 2 parameter namespaces, keeps its first value, which is also the one that the
// TTML reader reads the times with.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	s, warnings, err := ttml.Read(r, common, ttml.ReadOptions{})
	if err != nil {
		return nil, nil, err
	}
	data, _ := s.FormatData(string(model.TTML))
	found := map[string]string{}
	attributes, _ := data.Object("attributes")
	namespaces, _ := data.Object("namespaces")
	for _, name := range attributes.Keys() {
		prefix, local, hasPrefix := strings.Cut(name, ":")
		if !hasPrefix {
			prefix, local = "", name
		}
		if _, seen := found[local]; seen || !slices.Contains(timingParameters, local) || !isParameterPrefix(namespaces, prefix) {
			continue
		}
		value, _ := attributes.String(name)
		found[local] = strings.Trim(value, markupcore.PHPTrimSet)
	}
	var fields []model.Field
	for _, local := range timingParameters {
		if value, ok := found[local]; ok {
			fields = append(fields, model.Field{Key: local, Value: value})
		}
	}
	if len(fields) > 0 {
		_ = s.SetFormatData(formatDataKey, model.NewData(fields...)) // the data has the shape of the schema
	}
	return s, warnings, nil
}

// isParameterPrefix reports whether prefix maps to a TTML parameter namespace, or is ttp and has no mapping.
func isParameterPrefix(namespaces *model.Data, prefix string) bool {
	uri, ok := namespaces.String(prefix)
	if !ok {
		return prefix == "ttp"
	}
	return slices.Contains(ttmlwrite.ParameterNamespaces, uri)
}
