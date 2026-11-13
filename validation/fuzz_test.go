package validation_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/validation"
)

// ruleOrder is the order of the violations of one cue.
var ruleOrder = []validation.Rule{
	validation.RuleRequireCues, validation.RuleNoNegativeDuration, validation.RuleNoEmptyCues,
	validation.RuleMaxCharactersPerLine, validation.RuleMaxLinesPerCue, validation.RuleMaxCharactersPerSecond,
	validation.RuleMinDuration, validation.RuleMaxDuration, validation.RuleNoDoubleSpaces,
	validation.RuleNoLeadingOrTrailingSpaces, validation.RuleNoUnbalancedTags, validation.RuleDialogueDashStyle,
	validation.RuleMaxSpeakersPerCue, validation.RuleMaxWordsPerMinute, validation.RuleMinDurationPerWord,
	validation.RuleAllowedCharacters, validation.RuleNoAllCapsLines, validation.RuleNoOverlap, validation.RuleMinGap,
}

func FuzzCheck(f *testing.F) {
	f.Add("-Where are you?\u00a0 <i>Home\n- Wait.\n<v Anna>CAPS</v>", int64(1000), int64(2000), int64(1500))
	f.Add("<ruby>a<rt>b<rt>c\n[BELL]\n\u202b- x", int64(0), int64(0), int64(0))
	f.Add("CAF\xc9 \xc9T\xc9\n a  b ", int64(5000), int64(-1), int64(7))
	f.Add("", int64(0), int64(1), int64(1))

	rules := validation.Rules{
		MaxCharactersPerSecond: 20, MaxCharactersPerLine: 42, MaxLinesPerCue: 2, MinDuration: 5 * time.Second / 6, MaxDuration: 7 * time.Second,
		MinGap: 80 * time.Millisecond, NoOverlap: true, NoEmptyCues: true, NoDoubleSpaces: true, NoLeadingOrTrailingSpaces: true,
		NoUnbalancedTags: true, DialogueDashStyle: style(model.DialogueDashHyphenSpace), MaxSpeakersPerCue: 2, MaxWordsPerMinute: 180,
		MinDurationPerWord: 300 * time.Millisecond, AllowedCharacters: "[A-Za-z]", NoAllCapsLines: true, RequireCues: true,
		NoNegativeDuration: true,
	}
	f.Fuzz(func(t *testing.T, text string, a, b, c int64) {
		const span = int64(1_000_000_000)
		ms := func(v int64) time.Duration { return time.Duration(v%span) * time.Millisecond }
		s := &model.Subtitle{}
		for i, line := range strings.Split(text, "\n") {
			start := ms(a + int64(i)*b)
			s.AddCue(model.Cue{Start: start, End: start + ms(c+int64(i)*a), Lines: []string{line}})
		}

		got, err := validation.Check(s, rules)

		require.NoError(t, err)
		last, lastCue := -1, validation.NoCue
		for _, v := range got {
			assert.False(t, math.IsNaN(v.Value), "%v", v)
			if v.Cue != lastCue {
				last, lastCue = -1, v.Cue
			}
			at := indexOf(v.Rule)
			assert.GreaterOrEqual(t, at, last, "%v", v)
			last = at
		}
	})
}

func indexOf(r validation.Rule) int {
	for i, o := range ruleOrder {
		if o == r {
			return i
		}
	}
	return -1
}
