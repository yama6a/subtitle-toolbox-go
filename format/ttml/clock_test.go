package ttml_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// Ports testTheParserReadsTheTimeExpression and testReadsALooseExpression of tests/Parsers/TtmlClockTest.php of
// subtitle-toolbox-php.
func TestClock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, parameters, expression string
		want                         float64
	}{
		{"clock time", "", "01:02:03.5", 3723.5},
		{"clock time with comma", "", "00:00:01,500", 1.5},
		{"clock time with frames", "", "00:00:01:15", 1.5},
		{"offset in hours", "", "1.5h", 5400},
		{"offset in minutes", "", "2m", 120},
		{"offset in seconds", "", "3.5s", 3.5},
		{"offset in milliseconds", "", "250ms", 0.25},
		{"offset in frames", `ttp:frameRate="25"`, "50f", 2},
		{"offset in ticks", `ttp:tickRate="10000000"`, "15000000t", 1.5},
		{"ticks at the frame rate", `ttp:frameRate="25"`, "50t", 2},
		{"SMPTE drop NTSC", `ttp:timeBase="smpte" ttp:frameRate="30" ttp:frameRateMultiplier="1000 1001" ttp:dropMode="dropNTSC"`, "01:10:00:00", 4199.996},
		{"SMPTE drop PAL", `ttp:timeBase="smpte" ttp:frameRate="25" ttp:dropMode="dropPAL"`, "01:10:00:00", 4194.88},
		{"sub-frames", `ttp:frameRate="30" ttp:subFrameRate="2"`, "00:00:01:10.1", 1.35},
		{"frame rate multiplier", `ttp:frameRate="30" ttp:frameRateMultiplier="1000 1001"`, "00:00:00:30", 1.001},
		{"last field as hundredths", "", "00:00:01:75", 1.75},
		{"one-digit fields", "", "1:2:3", 3723},
		{"minutes and seconds", "", "01:05.5", 65.5},
		{"milliseconds as frames", "", "00:00:01:500", 1.5},
		{"bare milliseconds", "", "1500", 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _ := mustRead(t, `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:ttp="http://www.w3.org/ns/ttml#parameter" `+tt.parameters+`>`+
				`<body><div><p begin="`+tt.expression+`" dur="1s">Text</p></div></body></tt>`, true)
			c, _ := s.Cue(0)
			assert.Equal(t, sec(tt.want), c.Start)
		})
	}
}

func TestClockSkipsALooseExpressionThatIsNoTime(t *testing.T) {
	t.Parallel()

	s, warnings := mustRead(t, bare+`<body><div><p begin="soon" dur="1s">Text</p></div></body></tt>`, true)
	assert.Equal(t, 0, s.Len())
	assert.Equal(t, []warningRow{{1, 0, model.Skipped, `The time expression "soon" is not valid.`}}, warningRows(warnings))
}
