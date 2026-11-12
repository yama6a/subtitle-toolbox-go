package mkv

// SetInflateCap sets the number of bytes that Extract may inflate for one track.
func (f *File) SetInflateCap(n int64) { f.inflateCap = n }
