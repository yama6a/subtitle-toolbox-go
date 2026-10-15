package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

// formatDataHolder is the format data API that Subtitle and Cue share.
type formatDataHolder interface {
	FormatData(key string) (*model.Data, bool)
	SetFormatData(key string, d *model.Data) error
	DeleteFormatData(key string)
	FormatDataKeys() []string
}

func holders() []struct {
	name  string
	build func() formatDataHolder
} {
	return []struct {
		name  string
		build func() formatDataHolder
	}{
		{"subtitle", func() formatDataHolder { return &model.Subtitle{} }},
		{"cue", func() formatDataHolder { return &model.Cue{} }},
	}
}

func TestFormatDataIsEmptyByDefault(t *testing.T) {
	t.Parallel()

	for _, h := range holders() {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()

			d, ok := h.build().FormatData("ass")
			assert.False(t, ok)
			assert.Nil(t, d)
			assert.Empty(t, h.build().FormatDataKeys())
		})
	}
}

// Ports testSetAndFindFormatDataPerFormat and testSettingFormatDataReplacesPreviousData
// of tests/SubtitleTest.php and tests/SubtitleCueTest.php of subtitle-toolbox-php.
func TestSetFormatDataPerFormat(t *testing.T) {
	t.Parallel()

	for _, h := range holders() {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()

			o := h.build()
			require.NoError(t, o.SetFormatData("vtt", data("region", "top")))
			require.NoError(t, o.SetFormatData("ass", data("style", "Default", "marginV", int64(10))))

			got, ok := o.FormatData("ass")
			assert.True(t, ok)
			assert.Equal(t, data("style", "Default", "marginV", int64(10)), got)
			got, ok = o.FormatData("vtt")
			assert.True(t, ok)
			assert.Equal(t, data("region", "top"), got)
			_, ok = o.FormatData("srt")
			assert.False(t, ok)
			assert.Equal(t, []string{"ass", "vtt"}, o.FormatDataKeys())

			require.NoError(t, o.SetFormatData("ass", data("style", "Sign")))
			got, _ = o.FormatData("ass")
			assert.Equal(t, data("style", "Sign"), got)

			require.NoError(t, o.SetFormatData("ass", data()))
			_, ok = o.FormatData("ass")
			assert.False(t, ok)
			assert.Equal(t, []string{"vtt"}, o.FormatDataKeys())

			require.NoError(t, o.SetFormatData("vtt", nil))
			assert.Empty(t, o.FormatDataKeys())
		})
	}
}

func TestDeleteFormatData(t *testing.T) {
	t.Parallel()

	for _, h := range holders() {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()

			o := h.build()
			require.NoError(t, o.SetFormatData("vtt", data("region", "top")))
			require.NoError(t, o.SetFormatData("ass", data("text", "x")))
			o.DeleteFormatData("vtt")
			o.DeleteFormatData("missing")
			assert.Equal(t, []string{"ass"}, o.FormatDataKeys())
		})
	}
}

// Ports testSetFormatDataRejectsAFieldThatAFormatterReadsWithTheWrongType of tests/SubtitleTest.php
// and testSetFormatDataRejectsAFieldThatAFormatterReadsWithTheWrongShape of tests/SubtitleCueTest.php of subtitle-toolbox-php.
func TestSetFormatDataKeepsTheOldDataOnError(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	require.NoError(t, s.SetFormatData("scc", data("dropFrame", true, "note", int64(5))))
	got, _ := s.FormatData("scc")
	assert.Equal(t, data("dropFrame", true, "note", int64(5)), got)

	err := s.SetFormatData("scc", data("dropFrame", "yes"))
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	require.EqualError(t, err, "stbox: invalid argument: The field formatData.scc.dropFrame must be a boolean.")
	got, _ = s.FormatData("scc")
	assert.Equal(t, data("dropFrame", true, "note", int64(5)), got)

	var c model.Cue
	require.NoError(t, c.SetFormatData("srt", data("coordinates", data("x1", int64(1), "x2", int64(2), "y1", int64(3), "y2", int64(4)))))
	err = c.SetFormatData("srt", data("coordinates", data("x1", int64(1))))
	require.ErrorIs(t, err, model.ErrInvalidArgument)
	require.EqualError(t, err, "stbox: invalid argument: The field formatData.srt.coordinates.x2 is missing.")
	gotCue, _ := c.FormatData("srt")
	coordinates, ok := gotCue.Object("coordinates")
	require.True(t, ok)
	assert.Equal(t, []string{"x1", "x2", "y1", "y2"}, coordinates.Keys())
}

func TestFormatDataIsCopied(t *testing.T) {
	t.Parallel()

	for _, h := range holders() {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()

			o := h.build()
			in := data("dropFrame", true, "list", []any{"a"})
			require.NoError(t, o.SetFormatData("scc", in))
			in.Set("dropFrame", "after the check")

			out, _ := o.FormatData("scc")
			out.Set("dropFrame", "changed")
			list, _ := out.List("list")
			list[0] = "changed"

			got, _ := o.FormatData("scc")
			assert.Equal(t, data("dropFrame", true, "list", []any{"a"}), got)
		})
	}
}

func TestCueCopiesKeepTheirOwnFormatData(t *testing.T) {
	t.Parallel()

	var a model.Cue
	require.NoError(t, a.SetFormatData("vtt", data("line", "0")))
	b := a
	require.NoError(t, b.SetFormatData("vtt", data("line", "50%")))
	require.NoError(t, b.SetFormatData("srt", data("x", "1")))
	c := a.Clone()
	c.DeleteFormatData("vtt")

	got, _ := a.FormatData("vtt")
	assert.Equal(t, data("line", "0"), got)
	assert.Equal(t, []string{"vtt"}, a.FormatDataKeys())
	assert.Equal(t, []string{"srt", "vtt"}, b.FormatDataKeys())
	assert.Empty(t, c.FormatDataKeys())
}

func TestSubtitleKeepsCueFormatData(t *testing.T) {
	t.Parallel()

	var c model.Cue
	c.Lines = []string{"Hello"}
	require.NoError(t, c.SetFormatData("vtt", data("line", "0", "alignment", int64(8))))

	s := &model.Subtitle{}
	s.AddCue(c)
	require.NoError(t, c.SetFormatData("vtt", data("line", "100%")))

	stored, ok := s.Cue(0)
	require.True(t, ok)
	got, _ := stored.FormatData("vtt")
	assert.Equal(t, data("line", "0", "alignment", int64(8)), got)

	require.NoError(t, stored.SetFormatData("vtt", nil))
	again, _ := s.Cue(0)
	assert.Equal(t, []string{"vtt"}, again.FormatDataKeys())

	require.NoError(t, s.SetCue(0, stored))
	again, _ = s.Cue(0)
	assert.Empty(t, again.FormatDataKeys())
}

// Ports testCloneKeepsCuesMetadataCommentsAndFormatData of tests/SubtitleCloneTest.php of subtitle-toolbox-php.
func TestCloneKeepsFormatData(t *testing.T) {
	t.Parallel()

	s := &model.Subtitle{}
	var c model.Cue
	require.NoError(t, c.SetFormatData("vtt", data("line", "0")))
	s.AddCue(c)
	require.NoError(t, s.SetFormatData("srt", data("key", "value")))

	clone := s.Clone()
	require.NoError(t, s.SetFormatData("srt", data("key", "changed")))
	require.NoError(t, s.SetFormatData("vtt", data("header", "x")))

	got, ok := clone.FormatData("srt")
	assert.True(t, ok)
	assert.Equal(t, data("key", "value"), got)
	assert.Equal(t, []string{"srt"}, clone.FormatDataKeys())
	assert.Equal(t, s.Cues(), clone.Cues())
}
