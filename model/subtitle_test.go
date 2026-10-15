package model_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/model"
)

func TestEntryNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		in        model.Cue
		wantStart time.Duration
		wantEnd   time.Duration
		wantLines []string
	}{
		{
			name:      "half a millisecond rounds away from zero",
			in:        model.Cue{Start: sec(1.0005), End: sec(-1.0005)},
			wantStart: sec(1.001),
			wantEnd:   sec(-1.001),
		},
		{
			name:      "below half a millisecond rounds down",
			in:        model.Cue{Start: sec(1.2) + 499*time.Microsecond, End: sec(3.4)},
			wantStart: sec(1.2),
			wantEnd:   sec(3.4),
		},
		{
			name:      "CR LF inside a line becomes LF",
			in:        model.Cue{Lines: []string{"line1\r\nline2"}},
			wantLines: []string{"line1\nline2"},
		},
		{
			name:      "double CR before LF, blank lines and tabs",
			in:        model.Cue{Lines: []string{"line3  \r\r\n\n\r\n\t\tline4"}},
			wantLines: []string{"line3 \n line4"},
		},
		{
			name:      "lone CR becomes LF",
			in:        model.Cue{Lines: []string{"a\rb\r\rc"}},
			wantLines: []string{"a\nb\nc"},
		},
		{
			name:      "tabs and double spaces collapse",
			in:        model.Cue{Lines: []string{"this is a       line", "with a \t\t   linebreak"}},
			wantLines: []string{"this is a line", "with a linebreak"},
		},
		{
			name:      "trim like PHP trim",
			in:        model.Cue{Lines: []string{"\x00\x0b\t a \r\n"}},
			wantLines: []string{"a"},
		},
		{
			name:      "no-break space stays",
			in:        model.Cue{Lines: []string{"\u00a0a\u00a0\u00a0b\u00a0"}},
			wantLines: []string{"\u00a0a\u00a0\u00a0b\u00a0"},
		},
		{
			name:      "inner LF keeps one line",
			in:        model.Cue{Lines: []string{"a\nb"}},
			wantLines: []string{"a\nb"},
		},
		{
			name:      "empty lines are dropped",
			in:        model.Cue{Lines: []string{"", "a", " \t ", "\r\n", "b"}},
			wantLines: []string{"a", "b"},
		},
		{
			name: "no line left gives nil",
			in:   model.Cue{Lines: []string{"", " "}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := tt.in.Clone()
			want := tt.in.Clone()
			want.Start, want.End, want.Lines = tt.wantStart, tt.wantEnd, tt.wantLines

			for name, add := range map[string]func(*model.Subtitle){
				"AddCue":  func(s *model.Subtitle) { s.AddCue(in) },
				"AddCues": func(s *model.Subtitle) { s.AddCues(in) },
				"SetCue": func(s *model.Subtitle) {
					s.AddCue(model.Cue{})
					require.NoError(t, s.SetCue(0, in))
				},
				"ReplaceCues": func(s *model.Subtitle) { s.ReplaceCues([]model.Cue{in}, nil) },
			} {
				s := &model.Subtitle{}
				add(s)
				got, ok := s.Cue(0)
				require.True(t, ok, name)
				assert.Equal(t, want, got, name)
			}
			assert.Equal(t, tt.in, in, "the caller's cue stays as it was")
		})
	}
}

func TestAddCueSortsByStart(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		times [][2]float64
		want  []string
	}{
		{"later cue first", [][2]float64{{1, 2}, {5, 6}, {3, 4}}, []string{"text0", "text2", "text1"}},
		{"starts less than 1 s apart", [][2]float64{{1.5, 2}, {1.2, 1.4}}, []string{"text1", "text0"}},
		{"equal starts keep the insertion order", [][2]float64{{3, 4}, {1, 2}, {3, 5}, {1, 3}}, []string{"text1", "text3", "text0", "text2"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			one := &model.Subtitle{}
			for i, tm := range tt.times {
				one.AddCue(model.Cue{Start: sec(tm[0]), End: sec(tm[1]), Lines: []string{fmt.Sprintf("text%d", i)}})
			}
			assert.Equal(t, tt.want, texts(one))
			assert.Equal(t, tt.want, texts(fromTimes(tt.times...)))
		})
	}
}

func TestAddCuesGoAfterStoredCuesWithTheSameStart(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2}, [2]float64{3, 4})
	s.AddCues(model.Cue{Start: sec(3), Lines: []string{"added"}}, model.Cue{Start: sec(1), Lines: []string{"added first"}})

	assert.Equal(t, []string{"text0", "added first", "text1", "added"}, texts(s))
}

func TestAddCuesKeepsCommentsWithTheirCues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stored   [][2]float64
		comment  int
		added    []model.Cue
		wantText []string
		want     []model.Comment
	}{
		{
			name:     "sort moves the cue of the comment",
			stored:   [][2]float64{{8, 9}},
			comment:  0,
			added:    []model.Cue{{Start: sec(5), Lines: []string{"middle"}}, {Start: sec(1), Lines: []string{"early"}}},
			wantText: []string{"early", "middle", "text0"},
			want:     []model.Comment{{Text: "c", BeforeCue: 2}},
		},
		{
			name:     "comment after the last cue stays last when a cue sorts first",
			stored:   [][2]float64{{1, 2}, {3, 4}, {5, 6}},
			comment:  3,
			added:    []model.Cue{{Start: sec(0), End: sec(0.5), Lines: []string{"zero"}}},
			wantText: []string{"zero", "text0", "text1", "text2"},
			want:     []model.Comment{{Text: "c", BeforeCue: 4}},
		},
		{
			name:     "comment after the last cue stays last when a cue sorts last",
			stored:   [][2]float64{{1, 2}, {3, 4}},
			comment:  2,
			added:    []model.Cue{{Start: sec(5), End: sec(6), Lines: []string{"five"}}},
			wantText: []string{"text0", "text1", "five"},
			want:     []model.Comment{{Text: "c", BeforeCue: 3}},
		},
		{
			name:     "comment above the cue count goes after the last cue",
			stored:   [][2]float64{{1, 2}},
			comment:  7,
			added:    []model.Cue{{Start: sec(5), Lines: []string{"five"}}},
			wantText: []string{"text0", "five"},
			want:     []model.Comment{{Text: "c", BeforeCue: 2}},
		},
		{
			name:     "comment before a cue stays before it on append",
			stored:   [][2]float64{{1, 2}, {3, 4}},
			comment:  1,
			added:    []model.Cue{{Start: sec(5), Lines: []string{"five"}}},
			wantText: []string{"text0", "text1", "five"},
			want:     []model.Comment{{Text: "c", BeforeCue: 1}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes(tt.stored...)
			require.NoError(t, s.AddComment("c", tt.comment))
			s.AddCues(tt.added...)

			assert.Equal(t, tt.wantText, texts(s))
			assert.Equal(t, tt.want, s.Comments())
		})
	}
}

func TestSetCueGivesTheOrderOfAStableSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		index int
		start float64
		want  []string
	}{
		{"C moves to the start of B", 2, 2, []string{"A", "B", "C"}},
		{"A moves to the start of B", 0, 2, []string{"A", "B", "C"}},
		{"A moves after C", 0, 4, []string{"B", "C", "A"}},
		{"C moves before A", 2, 0.5, []string{"C", "A", "B"}},
		{"C moves to the start of A", 2, 1, []string{"A", "C", "B"}},
		{"B stays", 1, 2.5, []string{"A", "B", "C"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &model.Subtitle{}
			s.AddCues(
				model.Cue{Start: sec(1), Lines: []string{"A"}},
				model.Cue{Start: sec(2), Lines: []string{"B"}},
				model.Cue{Start: sec(3), Lines: []string{"C"}},
			)
			c, ok := s.Cue(tt.index)
			require.True(t, ok)
			c.Start = sec(tt.start)
			require.NoError(t, s.SetCue(tt.index, c))

			assert.Equal(t, tt.want, texts(s))
		})
	}
}

func TestSetCueMovesTheCommentsWithTheCue(t *testing.T) {
	t.Parallel()

	s := harbourTour(t)
	c, ok := s.Cue(3)
	require.True(t, ok)
	c.Start, c.End = sec(19), sec(20)
	require.NoError(t, s.SetCue(3, c))

	assert.Equal(t, []time.Duration{sec(1), sec(4), sec(6.5), sec(15), sec(18), sec(19)}, starts(s))
	assert.Equal(t, []model.Comment{
		{Text: "Recorded on the north pier", BeforeCue: 0},
		{Text: "Segment two starts here", BeforeCue: 1},
		{Text: "The boat turns left here", BeforeCue: 3},
		{Text: "The guide speaks from here", BeforeCue: 5},
		{Text: "End of recording", BeforeCue: 6},
	}, s.Comments())

	s.AddCue(model.Cue{Start: sec(25), End: sec(26), Lines: []string{"Thank you for joining the tour."}})

	assert.Equal(t, []time.Duration{sec(1), sec(4), sec(6.5), sec(15), sec(18), sec(19), sec(25)}, starts(s))
	assert.Equal(t, model.Comment{Text: "The guide speaks from here", BeforeCue: 5}, s.Comments()[3])
	assert.Equal(t, model.Comment{Text: "End of recording", BeforeCue: 7}, s.Comments()[4])
}

func TestSetCueChangesTheIndexes(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2}, [2]float64{3, 4}, [2]float64{5, 6})
	require.NoError(t, s.SetCue(0, model.Cue{Start: sec(10), End: sec(11), Lines: []string{"text0"}}))

	i, ok := s.CueIndexAt(sec(10.5))
	assert.True(t, ok)
	assert.Equal(t, 2, i)
	assert.Equal(t, []string{"text1", "text2", "text0"}, texts(s))

	require.NoError(t, s.RemoveCue(1))
	assert.Equal(t, []string{"text1", "text0"}, texts(s))
}

func TestCueIndexOutOfRange(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2})
	for _, i := range []int{-1, 1, 123} {
		_, ok := s.Cue(i)
		assert.False(t, ok, i)
		require.ErrorIs(t, s.SetCue(i, model.Cue{}), model.ErrCueNotFound, i)
		require.ErrorIs(t, s.RemoveCue(i), model.ErrCueNotFound, i)
	}
	assert.Equal(t, []string{"text0"}, texts(s))
}

func TestRemoveCue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		comments []model.Comment
		remove   int
		want     []model.Comment
	}{
		{
			name:     "comment before the removed cue moves to the next cue",
			comments: []model.Comment{{Text: "before second", BeforeCue: 1}, {Text: "before third", BeforeCue: 2}},
			remove:   1,
			want:     []model.Comment{{Text: "before second", BeforeCue: 1}, {Text: "before third", BeforeCue: 1}},
		},
		{
			name:     "comment before the removed last cue moves to the end",
			comments: []model.Comment{{Text: "before third", BeforeCue: 2}},
			remove:   2,
			want:     []model.Comment{{Text: "before third", BeforeCue: 2}},
		},
		{
			name:     "comment above the count stops at the new count",
			comments: []model.Comment{{Text: "far", BeforeCue: 9}},
			remove:   0,
			want:     []model.Comment{{Text: "far", BeforeCue: 2}},
		},
		{
			name:     "comment before an earlier cue stays",
			comments: []model.Comment{{Text: "first", BeforeCue: 0}},
			remove:   1,
			want:     []model.Comment{{Text: "first", BeforeCue: 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := fromTimes([2]float64{1, 2}, [2]float64{3, 4}, [2]float64{5, 6})
			for _, c := range tt.comments {
				require.NoError(t, s.AddComment(c.Text, c.BeforeCue))
			}
			require.NoError(t, s.RemoveCue(tt.remove))

			assert.Equal(t, 2, s.Len())
			assert.Equal(t, tt.want, s.Comments())
		})
	}
}

func TestRemoveCuesWhere(t *testing.T) {
	t.Parallel()

	t.Run("comments move to the next kept cue", func(t *testing.T) {
		t.Parallel()

		s := harbourTour(t)
		s.RemoveCuesWhere(func(c model.Cue) bool { return containsText(c, "Guide") })

		assert.Equal(t, 4, s.Len())
		assert.Equal(t, []model.Comment{
			{Text: "Recorded on the north pier", BeforeCue: 0},
			{Text: "Segment two starts here", BeforeCue: 1},
			{Text: "The guide speaks from here", BeforeCue: 3},
			{Text: "The boat turns left here", BeforeCue: 3},
			{Text: "End of recording", BeforeCue: 4},
		}, s.Comments())
	})

	t.Run("by duration", func(t *testing.T) {
		t.Parallel()

		s := weatherSigns()
		s.RemoveCuesWhere(func(c model.Cue) bool { return c.End-c.Start < sec(3.5) })

		assert.Equal(t, []string{"SUNNY\n25 C", "m 0 0 l 200 0 200 60 0 60"}, texts(s))
	})

	t.Run("comment before a removed last cue goes after the last cue", func(t *testing.T) {
		t.Parallel()

		s := fromTimes([2]float64{1, 2}, [2]float64{3, 4}, [2]float64{5, 6})
		require.NoError(t, s.AddComment("c", 1))
		s.RemoveCuesWhere(func(c model.Cue) bool { return c.Start > sec(2) })

		assert.Equal(t, []model.Comment{{Text: "c", BeforeCue: 1}}, s.Comments())
	})

	t.Run("the predicate cannot change the cues", func(t *testing.T) {
		t.Parallel()

		s := fromTimes([2]float64{1, 2})
		s.RemoveCuesWhere(func(c model.Cue) bool {
			c.Lines[0] = "changed"
			return false
		})
		assert.Equal(t, []string{"text0"}, texts(s))
	})
}

func TestCuesWhere(t *testing.T) {
	t.Parallel()

	s := harbourTour(t)

	assert.Equal(t, []int{0, 1}, s.CuesWhere(func(c model.Cue) bool { return containsText(c, "harbour") }))
	assert.Equal(t, []int{3, 4}, s.CuesWhere(func(c model.Cue) bool { return containsText(c, "Guide") }))
	assert.Empty(t, s.CuesWhere(func(model.Cue) bool { return false }))
	assert.Equal(t, 6, s.Len())
}

func TestComments(t *testing.T) {
	t.Parallel()

	t.Run("empty by default", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, (&model.Subtitle{}).Comments())
	})

	t.Run("AddComment keeps the comments in cue order", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		require.NoError(t, s.AddComment("after last cue", 2))
		require.NoError(t, s.AddComment("first before cue 0", 0))
		require.NoError(t, s.AddComment("second before cue 0", 0))
		require.NoError(t, s.AddComment("before cue 1", 1))

		assert.Equal(t, []model.Comment{
			{Text: "first before cue 0", BeforeCue: 0},
			{Text: "second before cue 0", BeforeCue: 0},
			{Text: "before cue 1", BeforeCue: 1},
			{Text: "after last cue", BeforeCue: 2},
		}, s.Comments())
	})

	t.Run("negative index", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		err := s.AddComment("text", -1)
		require.ErrorIs(t, err, model.ErrInvalidArgument)
		var argErr *model.InvalidArgumentError
		require.ErrorAs(t, err, &argErr)
		assert.Equal(t, model.InvalidArgumentError{Msg: "The cue index of a comment must not be negative, got -1."}, *argErr)
		assert.Empty(t, s.Comments())
	})

	t.Run("Comments returns a copy", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		require.NoError(t, s.AddComment("text", 0))
		s.Comments()[0].Text = "changed"
		assert.Equal(t, []model.Comment{{Text: "text"}}, s.Comments())
	})
}

func TestReplaceCues(t *testing.T) {
	t.Parallel()

	abc := func(t *testing.T) *model.Subtitle {
		t.Helper()

		s := &model.Subtitle{}
		s.AddCues(
			model.Cue{Start: sec(1), Lines: []string{"A"}},
			model.Cue{Start: sec(2), Lines: []string{"B"}},
			model.Cue{Start: sec(3), Lines: []string{"C"}},
		)
		require.NoError(t, s.AddComment("before B", 1))
		require.NoError(t, s.AddComment("before C", 2))
		require.NoError(t, s.AddComment("at the end", 3))
		return s
	}
	replacement := []model.Cue{
		{Start: sec(0.5), Lines: []string{"C2"}},
		{Start: sec(1), Lines: []string{"A2"}},
		{Start: sec(2), Lines: []string{"B2"}},
		{Start: sec(1), Lines: []string{"A3"}},
	}

	tests := []struct {
		name      string
		subtitle  func(t *testing.T) *model.Subtitle
		cues      []model.Cue
		anchor    func(old int) int
		wantTexts []string
		want      []model.Comment
	}{
		{
			name: "nil anchor keeps the index in the new cues",
			subtitle: func(t *testing.T) *model.Subtitle {
				t.Helper()

				s := &model.Subtitle{}
				require.NoError(t, s.AddComment("before late", 0))
				require.NoError(t, s.AddComment("before early too", 2))
				require.NoError(t, s.AddComment("at the end", 3))
				return s
			},
			cues: []model.Cue{
				{Start: sec(5), End: sec(6), Lines: []string{"late"}},
				{Start: sec(1), End: sec(2), Lines: []string{"early"}},
				{Start: sec(1), End: sec(3), Lines: []string{"early too"}},
			},
			wantTexts: []string{"early", "early too", "late"},
			want: []model.Comment{
				{Text: "before early too", BeforeCue: 1},
				{Text: "before late", BeforeCue: 2},
				{Text: "at the end", BeforeCue: 3},
			},
		},
		{
			name:      "anchor maps the old cues to the new cues",
			subtitle:  abc,
			cues:      replacement,
			anchor:    func(old int) int { return []int{1, 2, 0}[old] },
			wantTexts: []string{"C2", "A2", "A3", "B2"},
			want: []model.Comment{
				{Text: "before C", BeforeCue: 0},
				{Text: "before B", BeforeCue: 3},
				{Text: "at the end", BeforeCue: 4},
			},
		},
		{
			name:      "anchor out of range puts the comment after the last cue",
			subtitle:  abc,
			cues:      replacement,
			anchor:    func(old int) int { return []int{0, -1, 4}[old] },
			wantTexts: []string{"C2", "A2", "A3", "B2"},
			want: []model.Comment{
				{Text: "before B", BeforeCue: 4},
				{Text: "before C", BeforeCue: 4},
				{Text: "at the end", BeforeCue: 4},
			},
		},
		{
			name:      "no cues left",
			subtitle:  abc,
			anchor:    func(old int) int { return old },
			wantTexts: []string{},
			want: []model.Comment{
				{Text: "before B", BeforeCue: 0},
				{Text: "before C", BeforeCue: 0},
				{Text: "at the end", BeforeCue: 0},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := tt.subtitle(t)
			s.ReplaceCues(tt.cues, tt.anchor)

			assert.Equal(t, tt.wantTexts, texts(s))
			assert.Equal(t, tt.want, s.Comments())
		})
	}
}

func TestMetadata(t *testing.T) {
	t.Parallel()

	t.Run("empty by default", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		assert.Equal(t, map[string]string{}, s.AllMetadata())
		_, ok := s.Metadata(model.MetadataTitle)
		assert.False(t, ok)
	})

	t.Run("set, overwrite and delete", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		s.SetMetadata(model.MetadataTitle, "Yesterday")
		s.SetMetadata(model.MetadataArtist, "The Beatles")
		s.SetMetadata(model.MetadataAuthor, "Jane Doe")
		s.SetMetadata(model.MetadataLanguage, "en")
		s.SetMetadata(model.MetadataLanguage, "de")
		s.SetMetadata(model.MetadataAlbum, "Help!")
		s.SetMetadata("custom", "")
		s.DeleteMetadata(model.MetadataAuthor)
		s.DeleteMetadata("missing")

		v, ok := s.Metadata("title")
		assert.True(t, ok)
		assert.Equal(t, "Yesterday", v)
		v, ok = s.Metadata("custom")
		assert.True(t, ok)
		assert.Empty(t, v)
		_, ok = s.Metadata("author")
		assert.False(t, ok)
		assert.Equal(t, map[string]string{
			"title": "Yesterday", "artist": "The Beatles", "language": "de", "album": "Help!", "custom": "",
		}, s.AllMetadata())
	})

	t.Run("AllMetadata returns a copy", func(t *testing.T) {
		t.Parallel()

		s := &model.Subtitle{}
		s.SetMetadata("title", "a")
		s.AllMetadata()["title"] = "changed"
		s.AllMetadata()["new"] = "x"
		assert.Equal(t, map[string]string{"title": "a"}, s.AllMetadata())
	})
}

func TestCueAndCuesReturnDeepCopies(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2})
	c, ok := s.Cue(0)
	require.True(t, ok)
	c.Lines[0] = "changed"
	s.Cues()[0].Lines[0] = "changed"
	for _, c := range s.All() {
		c.Lines[0] = "changed"
	}

	assert.Equal(t, []string{"text0"}, texts(s))
}

func TestAllIteratesASnapshot(t *testing.T) {
	t.Parallel()

	s := fromTimes([2]float64{1, 2}, [2]float64{3, 4}, [2]float64{5, 6})
	var got []string
	var indexes []int
	for i, c := range s.All() {
		indexes = append(indexes, i)
		got = append(got, c.Text())
		s.AddCue(model.Cue{Start: 0, Lines: []string{"new"}})
		if i == 0 {
			require.NoError(t, s.RemoveCue(s.Len()-1))
		}
	}

	assert.Equal(t, []int{0, 1, 2}, indexes)
	assert.Equal(t, []string{"text0", "text1", "text2"}, got)
	assert.Equal(t, 5, s.Len())

	var first []int
	for i := range s.All() {
		first = append(first, i)
		break
	}
	assert.Equal(t, []int{0}, first)
}

func TestClone(t *testing.T) {
	t.Parallel()

	original := fromTimes([2]float64{1, 2}, [2]float64{3, 4})
	original.SetMetadata(model.MetadataTitle, "Clone test")
	require.NoError(t, original.AddComment("before second", 1))
	_, _ = original.CueIndexAt(sec(1))

	clone := original.Clone()
	assert.Equal(t, original.Cues(), clone.Cues())
	assert.Equal(t, original.Comments(), clone.Comments())
	assert.Equal(t, original.AllMetadata(), clone.AllMetadata())

	require.NoError(t, clone.SetCue(0, model.Cue{Start: sec(7), End: sec(8), Lines: []string{"changed"}}))
	clone.SetMetadata(model.MetadataTitle, "changed")
	require.NoError(t, clone.AddComment("new", 0))
	clone.RemoveCuesWhere(func(c model.Cue) bool { return c.Text() == "text1" })

	assert.Equal(t, []string{"text0", "text1"}, texts(original))
	assert.Equal(t, []model.Comment{{Text: "before second", BeforeCue: 1}}, original.Comments())
	assert.Equal(t, map[string]string{"title": "Clone test"}, original.AllMetadata())
	i, ok := original.CueIndexAt(sec(1))
	assert.True(t, ok)
	assert.Zero(t, i)
	_, ok = original.CueIndexAt(sec(7))
	assert.False(t, ok)

	empty := (&model.Subtitle{}).Clone()
	assert.Zero(t, empty.Len())
	assert.Equal(t, map[string]string{}, empty.AllMetadata())
}

func TestZeroValueIsAnEmptySubtitle(t *testing.T) {
	t.Parallel()

	var s model.Subtitle
	assert.Zero(t, s.Len())
	assert.Empty(t, s.Cues())
	assert.Empty(t, s.Comments())
	assert.Empty(t, s.CuesAt(sec(1)))
	_, ok := s.CueIndexAt(sec(1))
	assert.False(t, ok)
	got, err := s.CuesBetween(0, sec(10))
	require.NoError(t, err)
	assert.Empty(t, got)
	for range s.All() {
		t.Fatal("the zero value has no cues")
	}
}

func containsText(c model.Cue, sub string) bool {
	return strings.Contains(c.Text(), sub)
}
