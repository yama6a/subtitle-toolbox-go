package orderedjson_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/internal/orderedjson"
)

func FuzzDecode(f *testing.F) {
	for _, seed := range []string{
		`{"a": 1, "b": [0.0, 1e400, "\ud83d\ude00"]}`, `[]`, `{}`, `"\ud800"`, "\"\xe9\x9b\"", `{"a": 1, "a": 2}`, `[[[[]]]]`, ` 1 `, `-0`, `{"a":`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := orderedjson.Decode(data)
		if err != nil {
			var serr *orderedjson.SyntaxError
			require.ErrorAs(t, err, &serr)
			return
		}
		out, err := orderedjson.Encode(v, false)
		require.NoError(t, err)
		require.True(t, json.Valid(out))
		again, err := orderedjson.Decode(out)
		require.NoError(t, err)
		require.True(t, orderedjson.Equal(v, again))
		pretty, err := orderedjson.Encode(v, true)
		require.NoError(t, err)
		require.True(t, json.Valid(pretty))
	})
}
