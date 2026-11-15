package model

// The named frame rates of the broadcast formats. Treat them as read-only.
//
//nolint:gochecknoglobals // named frame rate, read-only by convention
var (
	FrameRate23976  = FrameRate{Num: 24000, Den: 1001}
	FrameRate24     = FrameRate{Num: 24, Den: 1}
	FrameRate25     = FrameRate{Num: 25, Den: 1}
	FrameRate2997   = FrameRate{Num: 30000, Den: 1001}
	FrameRate2997DF = FrameRate{Num: 30000, Den: 1001, DropFrame: true}
	FrameRate30     = FrameRate{Num: 30, Den: 1}
	FrameRate50     = FrameRate{Num: 50, Den: 1}
	FrameRate5994   = FrameRate{Num: 60000, Den: 1001}
	FrameRate5994DF = FrameRate{Num: 60000, Den: 1001, DropFrame: true}
	FrameRate60     = FrameRate{Num: 60, Den: 1}
)
