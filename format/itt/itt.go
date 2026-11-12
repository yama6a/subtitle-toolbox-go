// Package itt reads and writes iTunes Timed Text (iTT), the TTML profile that Apple asks for in iTunes Store and
// Apple TV deliveries.
//
// An iTT file is a TTML file with SMPTE time codes, one <div> and a top and a bottom region. The reader is the
// TTML reader of package ttml, plus the itt format data with the timing parameters of the root element. The writer
// writes a fixed Apple head, a reduced tag set, SMPTE time codes at one of 5 frame rates and 4 root parameters.
package itt

import (
	"math/big"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/ttmlwrite"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

const (
	formatDataKey   = string(model.ITT)
	frameRateOption = "WriteOptions.ITT.FrameRate"
)

const (
	needsFrameRate = "iTT output needs the frame rate of the video."
	badFrameRate   = "The ITT formatter accepts the frame rates 23.976, 24, 25, 29.97 and 30, got "
)

// profileRate is one frame rate of the iTT profile.
type profileRate struct {
	rate                  model.FrameRate // the effective rate of the row
	frameRate, multiplier string          // ttp:frameRate and ttp:frameRateMultiplier
}

//nolint:gochecknoglobals // the 5 frame rates of the iTT profile
var profileRates = [...]profileRate{
	{model.FrameRate{Num: 2997, Den: 125}, "24", "999 1000"},
	{model.FrameRate24, "24", "1 1"},
	{model.FrameRate25, "25", "1 1"},
	{model.FrameRate{Num: 2997, Den: 100}, "30", "999 1000"},
	{model.FrameRate30, "30", "1 1"},
}

// ReadOptions holds the settings of the iTT reader. It has no fields yet.
type ReadOptions struct{}

// WriteReport holds what the writer reports. It has no fields yet.
type WriteReport struct{}

// WriteOptions holds the settings of the iTT writer.
type WriteOptions struct {
	// FrameRate is the frame rate of the output: 23.976, 24, 25, 29.97 or 30, each within 0.01 fps. The zero value
	// takes the frame rate that the reader stored in the itt format data. FrameRate2997DF writes 29.97 with
	// ttp:dropMode="nonDrop".
	FrameRate model.FrameRate
}

// Validate returns an error that matches model.ErrInvalidArgument for a frame rate that is not within 0.01 fps of
// 23.976, 24, 25, 29.97 or 30.
func (o WriteOptions) Validate() error {
	if o.FrameRate.IsZero() {
		return nil
	}
	if _, ok := o.row(); !ok {
		return &model.InvalidArgumentError{Option: frameRateOption, Msg: badFrameRate + o.FrameRate.String() + "."}
	}
	return nil
}

// row returns the index in profileRates of the first row that the frame rate of o is near.
func (o WriteOptions) row() (int, bool) {
	if o.FrameRate.Num <= 0 || o.FrameRate.Den <= 0 {
		return 0, false
	}
	return rowOf(o.FrameRate)
}

func rowOf(r model.FrameRate) (int, bool) {
	for i, p := range profileRates {
		if p.rate.Near(r) {
			return i, true
		}
	}
	return 0, false
}

// storedRate returns the effective rate of the stored ttp:frameRate and ttp:frameRateMultiplier as an exact
// fraction, or false when it does not fit a frame rate. The multiplier counts as the TTML reader counts it.
func storedRate(frameRate, multiplier string) (model.FrameRate, bool) {
	a, b := ttmlwrite.Multiplier(multiplier)
	fps, okFPS := fraction(ttmlwrite.PHPFloat(frameRate))
	num, okNum := fraction(a)
	den, okDen := fraction(b)
	if !okFPS || !okNum || !okDen {
		return model.FrameRate{}, false
	}
	product := fps.Mul(fps, num)
	product.Quo(product, den)
	if !product.Num().IsInt64() || !product.Denom().IsInt64() {
		return model.FrameRate{}, false
	}
	r := model.FrameRate{Num: product.Num().Int64(), Den: product.Denom().Int64()}
	return r, r.Validate() == nil
}

// fraction returns f as the exact fraction that model.FrameRateFromFloat finds.
func fraction(f float64) (*big.Rat, bool) {
	r, err := model.FrameRateFromFloat(f)
	if err != nil {
		return nil, false
	}
	return big.NewRat(r.Num, r.Den), true
}
