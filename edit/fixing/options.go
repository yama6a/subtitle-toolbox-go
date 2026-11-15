package fixing

import (
	"slices"
	"strconv"

	"github.com/yama6a/subtitle-toolbox-go/v5/model"
)

// EllipsisStyle is the ellipsis that the ellipsis rule writes.
type EllipsisStyle uint8

// The ellipsis styles. EllipsisThreeDots is the zero value.
const (
	// EllipsisThreeDots writes "...".
	EllipsisThreeDots EllipsisStyle = iota
	// EllipsisUnicode writes U+2026. It also replaces "...".
	EllipsisUnicode
)

// String returns the constant name, such as "EllipsisThreeDots".
func (e EllipsisStyle) String() string {
	switch e {
	case EllipsisThreeDots:
		return "EllipsisThreeDots"
	case EllipsisUnicode:
		return "EllipsisUnicode"
	}
	return "EllipsisStyle(" + strconv.Itoa(int(e)) + ")"
}

// Options holds the settings of Apply and Preview. The zero value runs the default rules with the language
// metadata of the subtitle.
type Options struct {
	// Language is a code such as "en", "de-AT" or "fra". The empty string takes the language metadata.
	Language string
	// Enable lists rules that are off by default to run too. Nil runs the default rules only.
	Enable []Rule
	// Disable lists rules that are on by default to skip. Nil skips none.
	Disable []Rule
	// DialogueDashStyle is the dash that the dialogueDashes and dialogueOnOneLine rules write. The zero value is
	// a hyphen-minus and a space.
	DialogueDashStyle model.DialogueDashStyle
	// Ellipsis is the ellipsis that the ellipsis rule writes. The zero value writes "...".
	Ellipsis EllipsisStyle
	// ReplaceList holds the OCR replace list for the replaceList rule. Nil turns the rule off.
	ReplaceList *ReplaceList
}

// Validate returns an error that matches model.ErrInvalidArgument for an unknown rule name, a rule in both
// Enable and Disable, replaceList in Enable without a list, a regular expression that does not compile, or a style outside its constants.
func (o Options) Validate() error {
	for _, list := range []struct {
		name  string
		rules []Rule
	}{{"Enable", o.Enable}, {"Disable", o.Disable}} {
		for _, r := range list.rules {
			if !slices.Contains(Rules(), r) {
				return invalid(list.name, `Unknown rule "`+string(r)+`".`)
			}
		}
	}
	for _, r := range o.Enable {
		if slices.Contains(o.Disable, r) {
			return invalid("Enable", "the rule "+string(r)+" is in both Enable and Disable")
		}
	}
	if err := o.validateReplaceList(); err != nil {
		return err
	}
	if o.DialogueDashStyle.Text() == "" {
		return invalid("DialogueDashStyle", "the style "+o.DialogueDashStyle.String()+" is no dialogue dash style")
	}
	if o.Ellipsis > EllipsisUnicode {
		return invalid("Ellipsis", "the style "+o.Ellipsis.String()+" is not EllipsisThreeDots or EllipsisUnicode")
	}
	return nil
}

func (o Options) validateReplaceList() error {
	if o.ReplaceList == nil {
		if slices.Contains(o.Enable, RuleReplaceList) {
			return invalid("Enable", "the rule replaceList needs a replace list")
		}
		return nil
	}
	return o.ReplaceList.Validate()
}

func invalid(field, msg string) error {
	return &model.InvalidArgumentError{Option: "fixing.Options." + field, Msg: msg}
}

// runs reports whether the options turn the rule r on, before the language check.
func (o Options) runs(r Rule) bool {
	if r == RuleReplaceList {
		return o.ReplaceList != nil && !slices.Contains(o.Disable, r)
	}
	return slices.Contains(o.Enable, r) || r.Default() && !slices.Contains(o.Disable, r)
}
