package diff

import (
	"strconv"
	"strings"
	"time"

	"github.com/yama6a/subtitle-toolbox-go/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Text writes the differences as a text report. Each difference is a block with the kind, the cue numbers from 1,
// the old cue after "-" and the new cue after "+". Blocks are separated by an empty line. No differences give "".
func Text(differences []Difference) string {
	var b strings.Builder
	for n, d := range differences {
		if n > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(string(d.Kind) + ": ")
		var numbers []string
		if d.OldIndex >= 0 {
			numbers = append(numbers, "old cue "+strconv.Itoa(d.OldIndex+1))
		}
		if d.NewIndex >= 0 {
			numbers = append(numbers, "new cue "+strconv.Itoa(d.NewIndex+1))
		}
		b.WriteString(strings.Join(numbers, ", ") + "\n")
		writeCue(&b, "-", d.Old)
		writeCue(&b, "+", d.New)
	}
	return b.String()
}

func writeCue(b *strings.Builder, marker string, c *model.Cue) {
	if c == nil {
		return
	}
	b.WriteString(marker + " " + formatTime(c.Start) + " --> " + formatTime(c.End))
	if c.Forced {
		b.WriteString(" forced")
	}
	b.WriteByte('\n')
	for _, line := range c.Lines {
		b.WriteString("  " + line + "\n")
	}
}

func formatTime(d time.Duration) string {
	if d < 0 {
		return "-" + markupcore.CoreTimestamp(-d)
	}
	return markupcore.CoreTimestamp(d)
}
