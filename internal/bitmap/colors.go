package bitmap

import "math"

// Matrix holds Kr and Kb of a YCrCb matrix. Kg is 1 - Kr - Kb.
type Matrix struct{ Kr, Kb float64 }

var (
	// BT601 is the matrix of ITU-R BT.601, for video up to 576 lines.
	BT601 = Matrix{Kr: 0.299, Kb: 0.114} //nolint:gochecknoglobals // fixed coefficients of ITU-R BT.601
	// BT709 is the matrix of ITU-R BT.709, for video above 576 lines.
	BT709 = Matrix{Kr: 0.2126, Kb: 0.0722} //nolint:gochecknoglobals // fixed coefficients of ITU-R BT.709
)

// The limited range of 8-bit video: luma from 16 to 235, chroma from 16 to 240 around 128.
const (
	lumaMin     = 16
	lumaRange   = 235 - 16
	chromaZero  = 128
	chromaRange = 240 - 16
)

// MatrixFor returns the matrix of a video of screenHeight lines: BT.601 up to 576 lines, BT.709 above.
func MatrixFor(screenHeight int) Matrix {
	if screenHeight > 576 {
		return BT709
	}
	return BT601
}

// ToRGB turns the limited range code y, cr, cb into red, green and blue.
//
// Each product is wrapped in float64() so that no platform fuses a multiply and an add. A fused result can round to
// another byte.
func ToRGB(y, cr, cb byte, m Matrix) (r, g, b byte) {
	kg := 1 - m.Kr - m.Kb
	luma := float64((int(y)-lumaMin)*255) / lumaRange
	red := float64((int(cr)-chromaZero)*255) / chromaRange
	blue := float64((int(cb)-chromaZero)*255) / chromaRange
	return clamp(luma + float64(float64(2*(1-m.Kr))*red)),
		clamp(luma - float64(float64(2*m.Kb*(1-m.Kb))/kg*blue) - float64(float64(2*m.Kr*(1-m.Kr))/kg*red)),
		clamp(luma + float64(float64(2*(1-m.Kb))*blue))
}

func clamp(v float64) byte {
	return byte(max(0, min(255, math.Round(v))))
}
