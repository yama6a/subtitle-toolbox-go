package cli

import (
	"slices"
	"strings"

	"github.com/dlclark/regexp2/v2"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

func replaceGroup() group {
	return group{
		name:    "replace",
		summary: "Replace words or patterns in the text.",
		options: []option{
			{
				name: "replace", value: "FROM=TO", repeatable: true,
				help: "Replace FROM with TO in the text between tags, for example --replace colour=color. Repeatable.",
			},
			flagOption("replace-regex",
				`Read each FROM of --replace as a regular expression with delimiters, such as /\.{4,}/. TO can use $1.`),
			flagOption("replace-ignore-case", "Match FROM of --replace in any case."),
		},
		build: buildReplace,
	}
}

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildReplace(a *args) (edit, error) {
	if err := needs(a, "replace", "replace-regex", "replace-ignore-case"); err != nil {
		return nil, err
	}
	regex := a.Has("replace-regex")
	var st steps
	for _, pair := range a.Values("replace") {
		from, to, ok := strings.Cut(pair, "=")
		if !ok || from == "" {
			return nil, usage("The option --replace needs FROM=TO, got " + quote(pair) + ".")
		}
		opts := model.ReplaceOptions{Regex: regex, IgnoreCase: a.Has("replace-ignore-case")}
		if regex {
			p, hasGroup, err := regexReplacement(from)
			if err != nil {
				return nil, err
			}
			from, to = p.pattern, convertReplacement(to, hasGroup)
			opts.IgnoreCase = opts.IgnoreCase || p.ignoreCase
		}
		st = append(st, inPlace(func(s *stbox.Subtitle) error { return s.ReplaceText(from, to, opts) }))
	}
	return stepsEdit(st), nil
}

// regexReplacement converts from and compiles it once to find a bad pattern before the first read. ReplaceText
// compiles it again for each input. hasGroup reports whether the pattern has the group number n.
func regexReplacement(from string) (p pcrePattern, hasGroup func(n int) bool, err error) {
	invalid := "The option --replace has the invalid regular expression " + quote(from) + "."
	p, badFlag, ok := convertPCRE(from)
	switch {
	case badFlag != "":
		return p, nil, usage(invalid + " The flag " + badFlag + " is not supported.")
	case !ok:
		return p, nil, usage(invalid)
	}
	options := regexp2.None
	if p.ignoreCase {
		options = regexp2.IgnoreCase
	}
	re, err := regexp2.Compile(p.pattern, options, regexp2.OptionMaintainCaptureOrder())
	if err != nil {
		return p, nil, usage(invalid)
	}
	groups := re.GetGroupNumbers()
	return p, func(n int) bool { return slices.Contains(groups, n) }, nil
}
