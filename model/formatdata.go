package model

import (
	"maps"
	"slices"
)

// ImageKey is the cue format data key of image cue data until 3.0.
const ImageKey = "image"

// FormatData returns a deep copy of the file format data of key, or false when s has none.
func (s *Subtitle) FormatData(key string) (*Data, bool) {
	d, ok := s.formatData[key]
	return d.Clone(), ok
}

// SetFormatData stores a deep copy of d as the file format data of key. An empty d removes the key.
// It returns an error that matches ErrInvalidArgument, and keeps the old data, when d fails the type check of key.
func (s *Subtitle) SetFormatData(key string, d *Data) error {
	if err := checkFormatData(fileSchema, key, d); err != nil {
		return err
	}
	if d.Len() == 0 {
		delete(s.formatData, key)
		return nil
	}
	if s.formatData == nil {
		s.formatData = map[string]*Data{}
	}
	s.formatData[key] = d.Clone()
	return nil
}

// DeleteFormatData removes the file format data of key.
func (s *Subtitle) DeleteFormatData(key string) {
	delete(s.formatData, key)
}

// FormatDataKeys returns the keys of the file format data, sorted.
func (s *Subtitle) FormatDataKeys() []string {
	return slices.Sorted(maps.Keys(s.formatData))
}

// FormatData returns a deep copy of the cue format data of key, or false when c has none.
func (c Cue) FormatData(key string) (*Data, bool) {
	d, ok := c.formatData[key]
	return d.Clone(), ok
}

// SetFormatData stores a deep copy of d as the cue format data of key. An empty d removes the key.
// It returns an error that matches ErrInvalidArgument, and keeps the old data, when d fails the type check of key.
func (c *Cue) SetFormatData(key string, d *Data) error {
	if err := checkFormatData(cueSchema, key, d); err != nil {
		return err
	}
	if d.Len() == 0 {
		c.DeleteFormatData(key)
		return nil
	}
	// Copies of a Cue share the map, so each change builds a new map. A stored Data never changes.
	m := make(map[string]*Data, len(c.formatData)+1)
	maps.Copy(m, c.formatData)
	m[key] = d.Clone()
	c.formatData = m
	return nil
}

// DeleteFormatData removes the cue format data of key.
func (c *Cue) DeleteFormatData(key string) {
	if _, ok := c.formatData[key]; !ok {
		return
	}
	if len(c.formatData) == 1 {
		c.formatData = nil
		return
	}
	m := maps.Clone(c.formatData)
	delete(m, key)
	c.formatData = m
}

// FormatDataKeys returns the keys of the cue format data, sorted.
func (c Cue) FormatDataKeys() []string {
	return slices.Sorted(maps.Keys(c.formatData))
}

// AlignmentUnchanged reports whether c.Alignment equals the numpad alignment that the cue format data of key
// holds in its field "alignment", 2 when the field is missing or not an int64.
// A writer of key writes a stored exact position only while AlignmentUnchanged holds.
func (c Cue) AlignmentUnchanged(key string) bool {
	n, ok := c.formatData[key].Int("alignment")
	if !ok {
		n = int64(BottomCenter.Numpad())
	}
	return n == int64(c.Alignment.Numpad())
}
