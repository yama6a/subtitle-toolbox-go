package timecode

// PTSMilliseconds returns ticks of the 90 kHz clock of MPEG time stamps as whole milliseconds, rounded half up.
// A negative count gives 0. It does not overflow for any int64.
func PTSMilliseconds(ticks int64) int64 {
	if ticks <= 0 {
		return 0
	}
	return ticks/90 + (ticks%90+45)/90
}
