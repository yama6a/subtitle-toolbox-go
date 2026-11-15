package cli

import (
	"context"
	"strings"

	stbox "github.com/yama6a/subtitle-toolbox-go/v5"
)

// group is one option group of convert.
type group struct {
	name    string
	summary string
	options []option
	// build checks the options of the group and returns its edit, or nil when no trigger option is set. It reads
	// no file. The ass group has no build, because it sets write options and runs no edit.
	build func(a *args) (edit, error)
}

// edit is the work of one group on one input.
type edit interface {
	// apply returns s after the edit, or a new subtitle. label names the input in messages.
	apply(ctx context.Context, s *stbox.Subtitle, c *console, label string) (*stbox.Subtitle, error)
}

// fileEdit is an edit that reads the files that its options name. convert checks their paths with the
// inputs, then loads each file once per run, before the first input.
type fileEdit interface {
	edit
	sideFiles() []string
	load(ctx context.Context, c *console) error
}

// step is one library call of an edit. It returns s or a new subtitle.
type step func(s *stbox.Subtitle) (*stbox.Subtitle, error)

// steps is an edit that runs its library calls in order.
type steps []step

var _ edit = steps{}

func (st steps) apply(_ context.Context, s *stbox.Subtitle, _ *console, _ string) (*stbox.Subtitle, error) {
	for _, call := range st {
		var err error
		if s, err = call(s); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func inPlace(call func(s *stbox.Subtitle) error) step {
	return func(s *stbox.Subtitle) (*stbox.Subtitle, error) {
		return s, call(s)
	}
}

// stepsEdit returns st as an edit, or nil when st is empty.
//
//nolint:ireturn // it returns nil or steps as the edit interface
func stepsEdit(st steps) edit {
	if len(st) == 0 {
		return nil
	}
	return st
}

// flagStep is a library call that runs when the command line holds the flag name.
type flagStep struct {
	name string
	call func(s *stbox.Subtitle) error
}

func flagSteps(a *args, flags []flagStep) steps {
	var st steps
	for _, f := range flags {
		if a.Has(f.name) {
			st = append(st, inPlace(f.call))
		}
	}
	return st
}

// convertGroups lists the groups in pipeline order.
func convertGroups() []group {
	return []group{forcedGroup(), ocrGroup(), errorsGroup(), sdhGroup(), replaceGroup(), textGroup(), structureGroup(), retimeGroup(), snapGroup(), timingGroup(), maskingGroup(), karaokeGroup(), assGroup()}
}

func needs(a *args, needed string, opts ...string) error {
	for _, o := range opts {
		if a.Has(o) && !a.Has(needed) {
			return usage("Pass --" + needed + " with --" + o + ".")
		}
	}
	return nil
}

func needsOneOf(a *args, needed []string, opt string) error {
	if !a.Has(opt) {
		return nil
	}
	for _, n := range needed {
		if a.Has(n) {
			return nil
		}
	}
	list := "--" + needed[len(needed)-1]
	if len(needed) > 1 {
		list = "--" + strings.Join(needed[:len(needed)-1], ", --") + " or " + list
	}
	return usage("Pass " + list + " with --" + opt + ".")
}
