package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v4"
	"github.com/yama6a/subtitle-toolbox-go/v4/fixing"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
)

func errorsGroup() group {
	return group{
		name:    "errors",
		summary: "Fix spacing, punctuation and OCR errors.",
		options: []option{
			flagOption("errors-fix", "Fix spacing, punctuation, dash, tag and OCR errors such as lt's for It's."),
			valueOption("errors-replace-list", "FILE", "Also apply this Subtitle Edit OCR replace list, an XML file, with --errors-fix."),
			valueOption("errors-enable", "NAME[,NAME]", "Also apply these rules that are off by default with --errors-fix: "+
				ruleNames(fixing.OptionalRules())+"."),
			flagOption("errors-list-fixes", "Print each change of --errors-fix to standard error."),
		},
		build: buildErrors,
	}
}

func ruleNames(rules []fixing.Rule) string {
	names := make([]string, len(rules))
	for i, r := range rules {
		names[i] = string(r)
	}
	return strings.Join(names, ", ")
}

//nolint:ireturn // a group build returns its edit, which is one of several types
func buildErrors(a *args) (edit, error) {
	if err := needs(a, "errors-fix", "errors-replace-list", "errors-enable", "errors-list-fixes"); err != nil {
		return nil, err
	}
	if !a.Has("errors-fix") {
		return nil, nil //nolint:nilnil // a nil edit means the group is off
	}
	language, _ := a.Value("language")
	e := &errorsEdit{opts: fixing.Options{Language: language}, list: a.Has("errors-list-fixes")}
	e.listPath, _ = a.Value("errors-replace-list")
	optional := fixing.OptionalRules()
	names, _ := a.Value("errors-enable")
	for name := range strings.SplitSeq(names, ",") {
		name = strings.Trim(name, markupcore.TrimSet)
		if name == "" {
			continue
		}
		i := slices.IndexFunc(optional, func(r fixing.Rule) bool { return asciiLower(string(r)) == asciiLower(name) })
		if i < 0 {
			return nil, usage("Unknown rule " + quote(name) + " in --errors-enable. The valid names are " + ruleNames(optional) + ".")
		}
		if !slices.Contains(e.opts.Enable, optional[i]) {
			e.opts.Enable = append(e.opts.Enable, optional[i])
		}
	}
	return e, nil
}

// errorsEdit runs fixing.Apply and prints each fix with list. listPath names the replace list file, or is "".
type errorsEdit struct {
	opts     fixing.Options
	list     bool
	listPath string
}

var _ fileEdit = &errorsEdit{}

func (e *errorsEdit) sideFiles() []string {
	if e.listPath == "" {
		return nil
	}
	return []string{e.listPath}
}

func (e *errorsEdit) load(_ context.Context, c *console) error {
	if e.listPath == "" {
		return nil
	}
	content, err := readSideText(c, e.listPath)
	if err != nil {
		return err
	}
	l, err := fixing.ReadReplaceList(bytes.NewReader(content))
	if err != nil {
		return &fileError{msg: e.listPath + ": " + message(err)}
	}
	e.opts.ReplaceList = l
	return nil
}

func (e *errorsEdit) apply(_ context.Context, s *stbox.Subtitle, c *console, label string) (*stbox.Subtitle, error) {
	report, err := fixing.Apply(s, e.opts)
	if err != nil {
		return nil, err
	}
	if !e.list {
		return s, nil
	}
	for _, f := range report.Fixes {
		c.err(label + ": cue " + strconv.Itoa(f.CueIndex+1) + ": " + string(f.Rule) + ": " + jsonString(f.Before) + " -> " +
			jsonString(f.After) + "\n")
	}
	return s, nil
}

// jsonString writes s as a JSON string. It does not escape "/" or non-ASCII characters.
// Invalid UTF-8 becomes U+FFFD.
func jsonString(s string) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	//nolint:errchkjson // a string encodes without error into a strings.Builder
	_ = enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}
