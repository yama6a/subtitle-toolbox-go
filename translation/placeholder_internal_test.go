package translation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlaceholders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		encoded  string
		tags     []tagPair
		restored string
	}{
		{
			name:    "nested pair",
			text:    "<i>Run, <b>now</b>!</i>",
			encoded: "<x1>Run, <x2>now</x2>!</x1>",
			tags:    []tagPair{{"<i>", "</i>"}, {"<b>", "</b>"}},
		},
		{
			name:    "voice without end and a break",
			text:    "<v Fred>Hi <br/>there.",
			encoded: "<x1/>Hi <x2/>there.",
			tags:    []tagPair{{open: "<v Fred>"}, {open: "<br/>"}},
		},
		{
			name:    "entities",
			text:    "<b>Tom &amp; Jerry</b> sagen a &lt; b",
			encoded: "<x1>Tom &amp; Jerry</x1> sagen a &lt; b",
			tags:    []tagPair{{"<b>", "</b>"}},
		},
		{
			name:     "entity is decoded once and escaped",
			text:     "a &#39;b&#39; < c &amp;lt;",
			encoded:  "a 'b' &lt; c &amp;lt;",
			restored: "a 'b' &lt; c &amp;lt;",
		},
		{
			name:    "crossing pairs",
			text:    "<b><i>x</b></i>",
			encoded: "<x1><x2>x</x1></x2>",
			tags:    []tagPair{{"<b>", "</b>"}, {"<i>", "</i>"}},
		},
		{
			name:    "stray closing tag and upper case name",
			text:    "</i><B>x</b>",
			encoded: "<x1/><x2>x</x2>",
			tags:    []tagPair{{open: "</i>"}, {"<B>", "</b>"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded, tags := encode(tt.text)
			assert.Equal(t, tt.encoded, encoded)
			assert.Equal(t, tt.tags, tags)
			assert.True(t, keepsPlaceholders(encoded, tags))
			want := tt.restored
			if want == "" {
				want = tt.text
			}
			assert.Equal(t, want, restore(tokenize(encoded), tags))
		})
	}
}

func TestKeepsPlaceholders(t *testing.T) {
	t.Parallel()

	tags := []tagPair{{"<b>", "</b>"}}
	tests := []struct {
		name        string
		translation string
		tags        []tagPair
		want        bool
		removed     string
	}{
		{name: "kept", translation: "<x1>Run!</x1>", tags: tags, want: true},
		{name: "loose closing tag", translation: "<x1>Run! </ x1>", tags: tags, removed: "Run! "},
		{name: "swapped", translation: "Run!</x1><x1>", tags: tags, removed: "Run!"},
		{name: "extra placeholder", translation: "<x1>Run!</x1><x2/>", tags: tags, removed: "Run!"},
		{name: "doubled placeholder", translation: "<x1>Run!</x1></x1>", tags: tags, removed: "Run!"},
		{name: "no placeholder", translation: "Lauf", tags: tags, removed: "Lauf"},
		{name: "closing first", translation: "</x1>Lauf<x1>", tags: tags, removed: "Lauf"},
		{name: "loose closing", translation: "<x1>Lauf< /x1>", tags: tags, removed: "Lauf"},
		{name: "leading zero", translation: "<x01>Run!</x01>", tags: tags, removed: "Run!"},
		{name: "no tags", translation: "Lauf", want: true, removed: "Lauf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, keepsPlaceholders(tt.translation, tt.tags))
			if !tt.want {
				assert.Equal(t, tt.removed, removePlaceholders(tt.translation))
			}
		})
	}
}

func TestRestore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		translation string
		tags        []tagPair
		want        string
	}{
		{
			name:        "answer entities",
			translation: "<x1>Tom & Jerry</x1> &#39;say&#39; a < b &amp; c&nbsp;d",
			tags:        []tagPair{{"<b>", "</b>"}},
			want:        "<b>Tom &amp; Jerry</b> 'say' a &lt; b &amp; c d",
		},
		{
			name:        "pair of a space",
			translation: "a<x1> </x1>b",
			tags:        []tagPair{{"<b>", "</b>"}},
			want:        "a b",
		},
		{
			name:        "nested pairs of a space",
			translation: "<x1><x2> </x2></x1>",
			tags:        []tagPair{{"<i>", "</i>"}, {"<b>", "</b>"}},
			want:        " ",
		},
		{
			name:        "outer pair keeps its text",
			translation: "<x1>a <x2> </x2></x1>",
			tags:        []tagPair{{"<i>", "</i>"}, {"<b>", "</b>"}},
			want:        "<i>a  </i>",
		},
		{
			name:        "empty pair",
			translation: "<x1></x1>x",
			tags:        []tagPair{{"<i>", "</i>"}},
			want:        "x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, restore(tokenize(tt.translation), tt.tags))
		})
	}
}
