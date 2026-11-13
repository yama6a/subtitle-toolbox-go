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

// searchSteps are the offsets that ToYCrCb tries around the rounded code. A step of 2 is needed for colours such as
// 0xFFFF40 in BT.601.
var searchSteps = [...]int{0, -1, 1, -2, 2} //nolint:gochecknoglobals // fixed search order of the code search

// ToYCrCb returns the limited range code y, cr, cb that ToRGB turns back into the colour rgb (0xRRGGBB), or into the
// nearest colour when no code does. The search tries the codes within 2 steps of the rounded code, luma steps
// outermost, and the first code with the smallest squared error wins.
//
// Each product is wrapped in float64() for the same reason as in ToRGB.
func ToYCrCb(rgb uint32, m Matrix) (y, cr, cb byte) {
	target := [3]int{int(rgb >> 16 & 0xFF), int(rgb >> 8 & 0xFF), int(rgb & 0xFF)}
	r, g, b := float64(target[0]), float64(target[1]), float64(target[2])
	luma := float64(m.Kr*r) + float64(float64(1-m.Kr-m.Kb)*g) + float64(m.Kb*b)
	centerY := int(math.Round(lumaMin + float64(luma*lumaRange/255)))
	centerCr := int(math.Round(chromaZero + float64((r-luma)/float64(2*(1-m.Kr))*chromaRange/255)))
	centerCb := int(math.Round(chromaZero + float64((b-luma)/float64(2*(1-m.Kb))*chromaRange/255)))

	var best [3]byte
	bestError := math.MaxInt
	for _, ys := range searchSteps {
		for _, crs := range searchSteps {
			for _, cbs := range searchSteps {
				code := [3]byte{
					byte(max(lumaMin, min(lumaMin+lumaRange, centerY+ys))),
					byte(max(lumaMin, min(chromaZero+chromaRange/2, centerCr+crs))),
					byte(max(lumaMin, min(chromaZero+chromaRange/2, centerCb+cbs))),
				}
				red, green, blue := ToRGB(code[0], code[1], code[2], m)
				e := sq(int(red)-target[0]) + sq(int(green)-target[1]) + sq(int(blue)-target[2])
				if e < bestError {
					best, bestError = code, e
				}
			}
		}
	}
	return best[0], best[1], best[2]
}

func sq(v int) int { return v * v }
