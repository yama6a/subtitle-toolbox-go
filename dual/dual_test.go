package dual_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/dual"
	"github.com/yama6a/subtitle-toolbox-go/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestFromPairExamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		primary   []row
		secondary []row
		opts      dual.Options
		want      []got
	}{
		{
			"stack with a style",
			[]row{{1, 4, "Where are you going?"}},
			[]row{{1.2, 3.9, "Wohin gehst du?"}},
			dual.Options{SecondaryStyle: "i"},
			[]got{bottom(1, 4, "Where are you going?\n<i>Wohin gehst du?</i>")},
		},
		{
			"stack without a style",
			[]row{{1, 4, "Where are you going?"}},
			[]row{{1.2, 3.9, "Wohin gehst du?"}},
			dual.Options{Mode: dual.ModeStack},
			[]got{bottom(1, 4, "Where are you going?\nWohin gehst du?")},
		},
		{
			"stack takes the first primary cue on equal overlap",
			[]row{{0, 2, "first"}, {2, 4, "second"}},
			[]row{{1.5, 2.5, "between"}},
			dual.Options{},
			[]got{bottom(0, 2.5, "first\nbetween"), bottom(2, 4, "second")},
		},
		{
			"stack takes the first primary cue when the float overlaps differ in the last bit",
			[]row{{0, 1, "first"}, {1.2, 2.2, "second"}},
			[]row{{0, 3, "between"}},
			dual.Options{},
			[]got{bottom(0, 3, "first\nbetween"), bottom(1.2, 2.2, "second")},
		},
		{
			"stack takes the primary cue with the larger overlap",
			[]row{{0, 2, "first"}, {2, 4, "second"}},
			[]row{{1.5, 3.5, "between"}},
			dual.Options{},
			[]got{bottom(0, 2, "first"), bottom(1.5, 4, "second\nbetween")},
		},
		{
			"stack keeps touching cues apart",
			[]row{{0, 2, "first"}},
			[]row{{2, 3, "after"}},
			dual.Options{},
			[]got{bottom(0, 2, "first"), bottom(2, 3, "after")},
		},
		{
			"stack joins several secondary cues in start order",
			[]row{{0, 10, "first"}},
			[]row{{1, 2, "a"}, {3, 11, "b"}, {-1, 4, "c"}},
			dual.Options{SecondaryStyle: "b"},
			[]got{bottom(-1, 11, "first\n<b>c</b>\n<b>a</b>\n<b>b</b>")},
		},
		{
			"stack finds a partner behind a long primary cue",
			[]row{{0, 100, "long"}, {10, 11, "short"}, {12, 13, "other"}},
			[]row{{14, 15, "late"}},
			dual.Options{},
			[]got{bottom(0, 100, "long\nlate"), bottom(10, 11, "short"), bottom(12, 13, "other")},
		},
		{
			"stack keeps the alignment of an unpaired secondary cue",
			[]row{{0, 1, "first"}},
			[]row{{5, 6, "later"}},
			dual.Options{SecondaryStyle: `font color="#ffff00"`, SecondaryAlignment: alignment(model.TopLeft)},
			[]got{bottom(0, 1, "first"), bottom(5, 6, `<font color="#ffff00">later</font>`)},
		},
		{
			"top-bottom with an alignment",
			[]row{{0, 2, "first"}},
			[]row{{0, 2, "erste"}},
			dual.Options{Mode: dual.ModeTopBottom, SecondaryAlignment: alignment(model.TopRight)},
			[]got{bottom(0, 2, "first"), at(model.TopRight, 0, 2, "erste")},
		},
		{
			"top-bottom at the bottom",
			[]row{{0, 2, "first"}},
			[]row{{0, 2, "erste"}},
			dual.Options{Mode: dual.ModeTopBottom, SecondaryAlignment: alignment(model.BottomCenter)},
			[]got{bottom(0, 2, "first"), bottom(0, 2, "erste")},
		},
		{
			"top-bottom snaps to the primary times",
			[]row{{1, 4, "first"}},
			[]row{{1.25, 3.7, "erste"}},
			dual.Options{Mode: dual.ModeTopBottom},
			[]got{bottom(1, 4, "first"), at(model.TopCenter, 1, 3.7, "erste")},
		},
		{
			"top-bottom with a tolerance of 0",
			[]row{{1, 4, "first"}},
			[]row{{1.25, 3.7, "erste"}},
			dual.Options{Mode: dual.ModeTopBottom, TimeTolerance: tolerance(0)},
			[]got{bottom(1, 4, "first"), at(model.TopCenter, 1.25, 3.7, "erste")},
		},
		{
			"top-bottom snaps to an equal time with a tolerance of 0",
			[]row{{1, 4, "first"}},
			[]row{{1, 3.7, "erste"}},
			dual.Options{Mode: dual.ModeTopBottom, TimeTolerance: tolerance(0)},
			[]got{bottom(1, 4, "first"), at(model.TopCenter, 1, 3.7, "erste")},
		},
		{
			"top-bottom keeps both times when the snap would hide the cue",
			[]row{{1, 4, "first"}},
			[]row{{3.9, 4.1, "kurz"}},
			dual.Options{Mode: dual.ModeTopBottom},
			[]got{bottom(1, 4, "first"), at(model.TopCenter, 3.9, 4.1, "kurz")},
		},
		{
			"top-bottom snaps to the earlier time on a tie",
			[]row{{1, 2, "first"}, {2.5, 4, "second"}},
			[]row{{2.25, 3, "mitte"}},
			dual.Options{Mode: dual.ModeTopBottom},
			[]got{bottom(1, 2, "first"), at(model.TopCenter, 2, 3, "mitte"), bottom(2.5, 4, "second")},
		},
		{
			"top-bottom snaps to the closer time",
			[]row{{1, 2, "first"}, {2.1, 4, "second"}},
			[]row{{2.06, 3, "mitte"}},
			dual.Options{Mode: dual.ModeTopBottom},
			[]got{bottom(1, 2, "first"), bottom(2.1, 4, "second"), at(model.TopCenter, 2.1, 3, "mitte")},
		},
		{
			"top-bottom with a tolerance that includes the distance",
			[]row{{1, 4, "first"}},
			[]row{{1.25, 3.5, "erste"}},
			dual.Options{Mode: dual.ModeTopBottom, TimeTolerance: tolerance(250 * time.Millisecond)},
			[]got{bottom(1, 4, "first"), at(model.TopCenter, 1, 3.5, "erste")},
		},
		{
			"top-bottom puts the primary cue first at an equal start",
			[]row{{0, 2, "first"}},
			[]row{{0, 2, "erste"}, {0, 2, "zweite"}},
			dual.Options{Mode: dual.ModeTopBottom},
			[]got{bottom(0, 2, "first"), at(model.TopCenter, 0, 2, "erste"), at(model.TopCenter, 0, 2, "zweite")},
		},
		{
			"an empty primary",
			nil,
			[]row{{0, 2, "erste"}},
			dual.Options{},
			[]got{bottom(0, 2, "erste")},
		},
		{
			"an empty secondary",
			[]row{{0, 2, "first"}},
			nil,
			dual.Options{Mode: dual.ModeTopBottom},
			[]got{bottom(0, 2, "first")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := dual.FromPair(subtitle(tt.primary...), subtitle(tt.secondary...), tt.opts)

			require.NoError(t, err)
			assert.Equal(t, tt.want, describe(result))
		})
	}
}

func TestFromPairChangesNeitherInput(t *testing.T) {
	t.Parallel()

	for _, opts := range []dual.Options{
		{SecondaryStyle: "i"},
		{Mode: dual.ModeTopBottom, SecondaryAlignment: alignment(model.TopLeft)},
	} {
		primary, secondary := load(t, "station_en.srt"), load(t, "station_de.srt")
		primaryBefore, secondaryBefore := primary.Clone(), secondary.Clone()

		_, err := dual.FromPair(primary, secondary, opts)

		require.NoError(t, err)
		assert.Equal(t, primaryBefore, primary)
		assert.Equal(t, secondaryBefore, secondary)
	}
}

func TestFromPairReturnsANewSubtitle(t *testing.T) {
	t.Parallel()

	primary := subtitle(row{0, 2, "first"})
	result, err := dual.FromPair(primary, subtitle(), dual.Options{})
	require.NoError(t, err)

	result.AddCue(model.Cue{Start: time.Hour, End: 2 * time.Hour, Lines: []string{"added"}})

	assert.Equal(t, 1, primary.Len())
}

func TestFromPairMatchesTheExpectedFiles(t *testing.T) {
	t.Parallel()

	stack := dual.Options{SecondaryStyle: "i"}
	topBottom := dual.Options{Mode: dual.ModeTopBottom, SecondaryStyle: `font color="#ffff00"`}
	tests := []struct {
		name   string
		opts   dual.Options
		prefix string
	}{
		{"stack", stack, "station_stack"},
		{"top-bottom", topBottom, "station_top_bottom"},
	}
	for _, tt := range tests {
		for _, f := range formats() {
			t.Run(tt.name+" "+f.name, func(t *testing.T) {
				t.Parallel()

				result, err := dual.FromPair(load(t, "station_en.srt"), load(t, "station_de.srt"), tt.opts)
				require.NoError(t, err)
				var buf bytes.Buffer
				require.NoError(t, f.write(&buf, result))

				written, err := f.read(&buf)
				require.NoError(t, err)
				expected, err := f.read(bytes.NewReader(bytes.TrimPrefix(testfiles.Read(t, "dual/"+tt.prefix+"."+f.ext), []byte("\xef\xbb\xbf"))))
				require.NoError(t, err)
				assert.Equal(t, cuesOf(expected), cuesOf(written))
			})
		}
	}
}

// cuesOf returns the fields that the expected files fix.
func cuesOf(s *model.Subtitle) []model.Cue {
	cues := s.Cues()
	for i := range cues {
		cues[i] = model.Cue{Start: cues[i].Start, End: cues[i].End, Lines: cues[i].Lines, Alignment: cues[i].Alignment}
	}
	return cues
}

func TestFromPairKeepsMetadataCommentsAndFormatDataOfThePrimary(t *testing.T) {
	t.Parallel()

	primary := subtitle(row{0, 2, "first"}, row{3, 5, "second"})
	primary.SetMetadata(model.MetadataLanguage, "en")
	primary.SetMetadata(model.MetadataTitle, "Station")
	require.NoError(t, primary.SetFormatData("vtt", model.NewData(model.Field{Key: "header", Value: "Kind: captions"})))
	require.NoError(t, primary.AddComment("before second", 1))
	require.NoError(t, primary.AddComment("at the end", 2))
	secondary := subtitle(row{0.5, 1, "vorher"}, row{2.2, 2.8, "dazwischen"})
	secondary.SetMetadata(model.MetadataLanguage, "de")
	secondary.SetMetadata(model.MetadataAuthor, "Somebody")
	require.NoError(t, secondary.SetFormatData("ass", model.NewData(model.Field{Key: "sectionOrder", Value: []any{"Events"}})))
	require.NoError(t, secondary.AddComment("secondary comment", 0))

	result, err := dual.FromPair(primary, secondary, dual.Options{})

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"language": "en+de", "title": "Station"}, result.AllMetadata())
	assert.Equal(t, []string{"vtt"}, result.FormatDataKeys())
	assert.Equal(t, []model.Comment{{Text: "before second", BeforeCue: 2}, {Text: "at the end", BeforeCue: 3}}, result.Comments())
	assert.Equal(t,
		[]got{bottom(0, 2, "first\nvorher"), bottom(2.2, 2.8, "dazwischen"), bottom(3, 5, "second")},
		describe(result))
}

func TestFromPairLanguage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		primary, secondary map[string]string
		want               map[string]string
	}{
		{"both", map[string]string{"language": "en"}, map[string]string{"language": "de"}, map[string]string{"language": "en+de"}},
		{"only the primary", map[string]string{"language": "en"}, nil, map[string]string{"language": "en"}},
		{"only the secondary", nil, map[string]string{"language": "de"}, map[string]string{}},
		{"neither", nil, nil, map[string]string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			primary, secondary := subtitle(row{0, 2, "first"}), subtitle(row{0, 2, "erste"})
			for k, v := range tt.primary {
				primary.SetMetadata(k, v)
			}
			for k, v := range tt.secondary {
				secondary.SetMetadata(k, v)
			}

			result, err := dual.FromPair(primary, secondary, dual.Options{})

			require.NoError(t, err)
			assert.Equal(t, tt.want, result.AllMetadata())
		})
	}
}

func TestFromPairCommentPositions(t *testing.T) {
	t.Parallel()

	t.Run("an empty primary keeps its comment after the last cue", func(t *testing.T) {
		t.Parallel()

		primary := &model.Subtitle{}
		require.NoError(t, primary.AddComment("only a note", 0))

		result, err := dual.FromPair(primary, subtitle(row{0, 2, "erste"}), dual.Options{})

		require.NoError(t, err)
		assert.Equal(t, []model.Comment{{Text: "only a note", BeforeCue: 1}}, result.Comments())
		assert.Equal(t, 0, primary.Len())
	})
	t.Run("a comment after the last primary cue stays after the last cue", func(t *testing.T) {
		t.Parallel()

		primary := subtitle(row{0, 2, "first"})
		require.NoError(t, primary.AddComment("end", 1))

		result, err := dual.FromPair(primary, subtitle(row{5, 6, "later"}), dual.Options{})

		require.NoError(t, err)
		assert.Equal(t, []model.Comment{{Text: "end", BeforeCue: 2}}, result.Comments())
	})
	t.Run("a comment stays before its cue when a secondary cue comes first", func(t *testing.T) {
		t.Parallel()

		primary := subtitle(row{5, 6, "first"})
		require.NoError(t, primary.AddComment("note", 0))

		result, err := dual.FromPair(primary, subtitle(row{1, 2, "vorher"}), dual.Options{})

		require.NoError(t, err)
		assert.Equal(t, []model.Comment{{Text: "note", BeforeCue: 1}}, result.Comments())
	})
}

func TestFromPairKeepsThePrimaryCueData(t *testing.T) {
	t.Parallel()

	var primaryCue model.Cue
	primaryCue.Start, primaryCue.End = 0, 2*time.Second
	primaryCue.Lines = []string{"first"}
	primaryCue.ID, primaryCue.Alignment, primaryCue.Forced = "7", model.TopLeft, true
	require.NoError(t, primaryCue.SetFormatData("ass", model.NewData(model.Field{Key: "text", Value: "raw"})))
	var primary model.Subtitle
	primary.AddCue(primaryCue)

	result, err := dual.FromPair(&primary, subtitle(row{1, 3, "erste"}), dual.Options{})

	require.NoError(t, err)
	got, ok := result.Cue(0)
	require.True(t, ok)
	assert.Equal(t, "7", got.ID)
	assert.Equal(t, model.TopLeft, got.Alignment)
	assert.True(t, got.Forced)
	assert.Equal(t, []string{"ass"}, got.FormatDataKeys())
	assert.Equal(t, 3*time.Second, got.End)
}

func TestFromPairSecondaryCuesLoseIDAndFormatData(t *testing.T) {
	t.Parallel()

	var cue model.Cue
	cue.Start, cue.End, cue.Lines, cue.ID = 5*time.Second, 6*time.Second, []string{"sp\u00e4ter"}, "7"
	require.NoError(t, cue.SetFormatData("ass", model.NewData(model.Field{Key: "text", Value: "raw"})))
	var secondary model.Subtitle
	secondary.AddCue(cue)

	for _, mode := range []dual.Mode{dual.ModeStack, dual.ModeTopBottom} {
		result, err := dual.FromPair(subtitle(row{0, 2, "first"}), &secondary, dual.Options{Mode: mode})

		require.NoError(t, err)
		got, ok := result.Cue(1)
		require.True(t, ok)
		assert.Empty(t, got.ID, mode)
		assert.Empty(t, got.FormatDataKeys(), mode)
	}
}

func TestFromPairForcedFlag(t *testing.T) {
	t.Parallel()

	forced := func(start, end time.Duration, text string) *model.Subtitle {
		var s model.Subtitle
		s.AddCue(model.Cue{Start: start, End: end, Lines: []string{text}, Forced: true})
		return &s
	}
	t.Run("a secondary cue of its own keeps the flag", func(t *testing.T) {
		t.Parallel()

		for _, mode := range []dual.Mode{dual.ModeStack, dual.ModeTopBottom} {
			result, err := dual.FromPair(subtitle(row{0, 1, "first"}), forced(5*time.Second, 6*time.Second, "sp\u00e4ter"), dual.Options{Mode: mode})

			require.NoError(t, err)
			c, _ := result.Cue(1)
			assert.True(t, c.Forced, mode)
		}
	})
	t.Run("top-bottom keeps the flag of a snapped cue", func(t *testing.T) {
		t.Parallel()

		result, err := dual.FromPair(subtitle(row{0, 2, "first"}), forced(0, 2*time.Second, "erste"), dual.Options{Mode: dual.ModeTopBottom})

		require.NoError(t, err)
		c, _ := result.Cue(1)
		assert.True(t, c.Forced)
	})
	t.Run("stack uses the flag of the primary cue", func(t *testing.T) {
		t.Parallel()

		result, err := dual.FromPair(subtitle(row{0, 2, "first"}), forced(0, 2*time.Second, "erste"), dual.Options{})

		require.NoError(t, err)
		c, _ := result.Cue(0)
		assert.False(t, c.Forced)
		result, err = dual.FromPair(forced(0, 2*time.Second, "first"), subtitle(row{0, 2, "erste"}), dual.Options{})
		require.NoError(t, err)
		c, _ = result.Cue(0)
		assert.True(t, c.Forced)
	})
}

func TestFromPairWritesTheSecondaryStyleAsATagPair(t *testing.T) {
	t.Parallel()

	tests := []struct{ style, want string }{
		{"i", "<i>erste</i>"},
		{" i ", "<i>erste</i>"},
		{"I", "<i>erste</i>"},
		{`font color="#ffff00"`, `<font color="#ffff00">erste</font>`},
		{`FONT Color="#FFFF00"`, `<font Color="#FFFF00">erste</font>`},
		{"v Anna", "<v Anna>erste</v>"},
		{"", "erste"},
	}
	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			t.Parallel()

			result, err := dual.FromPair(subtitle(row{0, 2, "first"}), subtitle(row{0, 2, "erste"}), dual.Options{SecondaryStyle: tt.style})

			require.NoError(t, err)
			c, _ := result.Cue(0)
			assert.Equal(t, []string{"first", tt.want}, c.Lines)
		})
	}
}

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts dual.Options
		msg  string
	}{
		{"a negative tolerance", dual.Options{TimeTolerance: tolerance(-100 * time.Millisecond)}, "The time tolerance -0.1 must not be negative."},
		{"an unknown tag", dual.Options{SecondaryStyle: "c.yellow"}, `The secondary style c.yellow must be a core markup tag such as i or font color="#ffff00".`},
		{"a tag that is no core tag", dual.Options{SecondaryStyle: "em"}, `The secondary style em must be a core markup tag such as i or font color="#ffff00".`},
		{"only white space", dual.Options{SecondaryStyle: "  "}, `The secondary style    must be a core markup tag such as i or font color="#ffff00".`},
		{"an unknown mode", dual.Options{Mode: "topBottom"}, `The mode "topBottom" is not stack or top-bottom.`},
		{"an alignment outside the grid", dual.Options{SecondaryAlignment: alignment(9)}, "The secondary alignment Alignment(9) is not one of the 9 alignment constants."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()

			require.ErrorIs(t, err, model.ErrInvalidArgument)
			var invalid *model.InvalidArgumentError
			require.ErrorAs(t, err, &invalid)
			assert.Equal(t, tt.msg, invalid.Msg)

			_, err = dual.FromPair(&model.Subtitle{}, &model.Subtitle{}, tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}
}

func TestOptionsValidateAccepts(t *testing.T) {
	t.Parallel()

	for _, opts := range []dual.Options{
		{},
		{Mode: dual.ModeStack},
		{Mode: dual.ModeTopBottom},
		{TimeTolerance: tolerance(0)},
		{SecondaryStyle: "b"},
		{SecondaryStyle: "u"},
		{SecondaryStyle: "s"},
		{SecondaryStyle: "v"},
		{SecondaryAlignment: alignment(model.BottomCenter)},
		{SecondaryAlignment: alignment(model.TopRight)},
	} {
		assert.NoError(t, opts.Validate(), opts)
	}
}
