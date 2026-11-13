package jsondata_test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/jsondata"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/orderedjson"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

func TestToData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  any
	}{
		{"int", `12`, int64(12)},
		{"minus zero", `-0`, int64(0)},
		{"float with fraction", `12.0`, float64(12)},
		{"exponent", `1e3`, float64(1000)},
		{"beyond int64", `9223372036854775808`, 9.223372036854776e18},
		{"beyond float64", `1e400`, math.Inf(1)},
		{"negative beyond float64", `-1e400`, math.Inf(-1)},
		{"null", `null`, nil},
		{"bool", `true`, true},
		{"string", `"a"`, "a"},
		{"list", `[1, "a"]`, []any{int64(1), "a"}},
		{"object in order", `{"b": 1, "a": 2.5}`, model.NewData(model.Field{Key: "b", Value: int64(1)}, model.Field{Key: "a", Value: 2.5})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v, err := orderedjson.Decode([]byte(tt.input))
			require.NoError(t, err)
			assert.Equal(t, tt.want, jsondata.ToData(v))
		})
	}
}

func TestFromData(t *testing.T) {
	t.Parallel()

	data := model.NewData(model.Field{Key: "z", Value: 0.0}, model.Field{Key: "a", Value: []any{int64(-3), nil, true, "x"}})
	v, err := jsondata.FromData(data)
	require.NoError(t, err)
	out, err := orderedjson.Encode(v, false)
	require.NoError(t, err)
	assert.Equal(t, `{"z":0.0,"a":[-3,null,true,"x"]}`, string(out)) //nolint:testifylint // JSONEq does not compare number text

	_, err = jsondata.FromData(int(1))
	require.Error(t, err)
	_, err = jsondata.FromData([]any{struct{}{}})
	require.Error(t, err)
}

// knownDataExceptions lists the files that do not come back equal through format data.
var knownDataExceptions = map[string]bool{
	"podcast/real/spec_word_segments.json": true,
	"whisper/own_out_of_range_number.json": true,
}

func TestFixtureRoundTrip(t *testing.T) {
	t.Parallel()

	root := testfiles.Root(t)
	var files []string
	for _, folder := range []string{"json/real", "whisper", "aws-transcribe", "deepgram", "assemblyai", "google-speech", "podcast", "youtube", "chapters/podcast"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(folder)), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if ext := filepath.Ext(p); !d.IsDir() && (ext == ".json" || ext == ".json3") && filepath.Base(filepath.Dir(p)) != "expected" {
				rel, err := filepath.Rel(root, p)
				files = append(files, filepath.ToSlash(rel))
				return err
			}
			return nil
		})
		require.NoError(t, err)
	}
	require.NotEmpty(t, files)

	for _, rel := range files {
		t.Run(rel, func(t *testing.T) {
			t.Parallel()

			first, err := orderedjson.Decode(testfiles.Read(t, rel))
			require.NoError(t, err)
			compact, err := orderedjson.Encode(first, false)
			require.NoError(t, err)
			again, err := orderedjson.Decode(compact)
			require.NoError(t, err)
			assert.True(t, orderedjson.Equal(first, again))

			back, err := jsondata.FromData(jsondata.ToData(first))
			require.NoError(t, err)
			assert.Equal(t, !knownDataExceptions[rel], orderedjson.Equal(first, back))
		})
	}
}
