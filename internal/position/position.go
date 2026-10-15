// Package position holds the rule that an explicit cue alignment wins over an exact position that a reader stored.
package position

import "github.com/yama6a/subtitle-toolbox-go/model"

// AlignmentUnchanged reports whether a equals the numpad alignment in the field "alignment" of the cue format data d.
func AlignmentUnchanged(d *model.Data, a model.Alignment) bool {
	n, ok := d.Int("alignment")
	if !ok {
		n = int64(model.BottomCenter.Numpad())
	}
	return n == int64(a.Numpad())
}
