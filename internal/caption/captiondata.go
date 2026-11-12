package caption

import (
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea608"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/cea708"
	"github.com/yama6a/subtitle-toolbox-go/v2/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v2/model"
)

// CaptionDataCues returns the cues of a CEA-708 service. When service is 0, or the service gives no cue,
// it returns the cues of the CEA-608 field 1 pairs, read with opts.
// The CEA-708 cues hold only the alignment of the window anchor, and no offset.
//
//nolint:revive // the name comes from the issue and the PHP class
func CaptionDataCues(frames []cea708.Frame, service int, opts CEA608Options, lastCueDuration time.Duration) []model.Cue {
	if service != 0 {
		if cues := cea708Cues(cea708.NewDecoder(service).Decode(frames), lastCueDuration); len(cues) > 0 {
			return cues
		}
	}
	return CEA608Cues(codeLines(frames), opts, lastCueDuration)
}

func cea708Cues(states []cea708.State, lastCueDuration time.Duration) []model.Cue {
	var cues []model.Cue
	for i, s := range states {
		if len(s.Lines) == 0 {
			continue
		}
		end := parsing.EndAfter(s.Time, lastCueDuration)
		if i+1 < len(states) {
			end = states[i+1].Time
		}
		if end <= s.Time {
			continue
		}
		cue := model.Cue{Start: s.Time, End: end, Lines: s.Lines}
		cue.Position.Alignment = alignmentOf(s.AnchorPoint)
		cues = append(cues, cue)
	}
	return cues
}

// codeLines joins the CEA-608 field 1 byte pairs of consecutive frames into code lines, as an SCC file holds
// them. A null pair ends a line, and so does a gap of more than one frame at 29.97 fps.
func codeLines(frames []cea708.Frame) []cea608.CodeLine {
	var lines []cea608.CodeLine
	open := false
	for _, f := range frames {
		frame := model.FrameRate2997.Frames(f.Time)
		for _, t := range f.Triplets {
			if t.Type != cea708.Field1 {
				continue
			}
			word := uint16(t.Data[0])<<8 | uint16(t.Data[1])
			if word&0x7F7F == 0 {
				open = false
				continue
			}
			if !open || frame > lines[len(lines)-1].Start+int64(len(lines[len(lines)-1].Words))+1 {
				lines = append(lines, cea608.CodeLine{Start: frame})
				open = true
			}
			last := &lines[len(lines)-1]
			last.Words = append(last.Words, word)
		}
	}
	return lines
}

// CEA708Captions turns cues into encoder captions: the anchor point comes from the alignment.
func CEA708Captions(cues []model.Cue) []cea708.Caption {
	captions := make([]cea708.Caption, len(cues))
	for i, c := range cues {
		a := c.Position.Alignment
		captions[i] = cea708.Caption{Start: c.Start, End: c.End, Lines: c.Lines, AnchorPoint: 3*(2-a.Row()) + a.Column() - 1}
	}
	return captions
}

// alignmentOf returns the alignment of the anchor point 0 to 8.
func alignmentOf(point int) model.Alignment {
	a, _ := model.AlignmentAt(2-point/3, point%3+1)
	return a
}
