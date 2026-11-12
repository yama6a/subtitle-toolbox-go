package cavena890

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/parsing"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
)

var blockEndBytes = make([]byte, blockEnd) //nolint:gochecknoglobals // 4 zero bytes

// Read reads a Cavena 890 file from its raw bytes.
//
// Adapted from Subtitle Edit, src/libse/SubtitleFormats/Cavena890.cs
// (https://github.com/SubtitleEdit/subtitleedit/blob/a0dfbfe65f0517ef85b1e69f1606bca93f68ac3c/src/libse/SubtitleFormats/Cavena890.cs), MIT.
func Read(r io.Reader, common model.CommonReadOptions, _ ReadOptions) (*model.Subtitle, []model.ParseWarning, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("read %s: %w", model.Cavena890, err)
	}
	if len(content) < headerSize {
		return nil, nil, &model.ParseError{Format: model.Cavena890, Msg: fmt.Sprintf("A Cavena 890 file starts with a header of %d bytes.", headerSize)}
	}
	st := parsing.NewState(model.Cavena890, common)
	header, body := content[:headerSize], content[headerSize:]
	v := variantOf(header)
	shortEnd := len(body)%blockSize == blockSize-blockEnd
	if shortEnd {
		body = append(bytes.Clone(body), blockEndBytes...)
	} else if rest := len(body) % blockSize; rest != 0 {
		complete := len(body) / blockSize
		err := st.Fail(&model.ParseError{Format: model.Cavena890, Msg: fmt.Sprintf("The blocks of a Cavena 890 file have %d bytes each.", blockSize)},
			model.NoLine, complete, []string{hex.EncodeToString(body[complete*blockSize:])})
		if err != nil {
			return nil, nil, err
		}
		body = body[:complete*blockSize]
	}

	var cues []model.Cue
	for _, blocks := range cueBlocks(body) {
		cues = append(cues, cue(blocks, v))
	}
	s := &model.Subtitle{}
	parsing.AddParsed(s, cues, nil)
	_ = s.SetFormatData(formatDataKey, model.NewData( // the data has the shape that the writer reads
		model.Field{Key: "variant", Value: string(v)},
		model.Field{Key: "header", Value: hex.EncodeToString(header)},
		model.Field{Key: "shortEnd", Value: shortEnd},
	))
	return s, st.Warnings(), nil
}

// cueBlocks groups the blocks of one cue. A block with the same 2 number bytes as the block before it adds 2 lines
// to that cue.
func cueBlocks(body []byte) [][][]byte {
	var cues [][][]byte
	for i := 0; i < len(body); i += blockSize {
		block := body[i : i+blockSize]
		last := len(cues) - 1
		if last >= 0 && bytes.Equal(cues[last][len(cues[last])-1][numberAt:numberAt+2], block[numberAt:numberAt+2]) {
			cues[last] = append(cues[last], block)
		} else {
			cues = append(cues, [][]byte{block})
		}
	}
	return cues
}

func cue(blocks [][]byte, v variant) model.Cue {
	first := blocks[0]
	c := model.Cue{
		Start:    frames(first[startAt:]),
		End:      frames(first[endAt:]),
		Position: model.Position{Alignment: alignmentOf(first[alignmentAt])},
	}
	hexes := make([]any, len(blocks))
	for i, block := range blocks {
		hexes[i] = hex.EncodeToString(block)
		for _, at := range [...]int{line1At, line2At} {
			if line := v.decode(block[at : at+textSize]); strings.Trim(line, markupcore.PHPTrimSet) != "" {
				c.AddLine(line)
			}
		}
	}
	_ = c.SetFormatData(formatDataKey, model.NewData( // the data has the shape that the writer reads
		model.Field{Key: "text", Value: c.Text()},
		model.Field{Key: "blocks", Value: hexes},
	))
	return c
}

// frames returns the time of the frame number in the 3 big-endian bytes of b, at 25 fps.
func frames(b []byte) time.Duration {
	return model.FrameRate25.Duration(int64(b[0])<<16 | int64(b[1])<<8 | int64(b[2]))
}
