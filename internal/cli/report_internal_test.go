package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportCommandKeepsTheJSONText(t *testing.T) {
	t.Parallel()

	entries := []any{map[string]string{"text": "<b>a & b</b> \xff"}}
	r := &runner{c: &console{}}
	var out stringWriter
	r.c.stdout = &out
	o := &reportOutput{json: true, entries: entries}
	require.NoError(t, o.finish(r))
	assert.Equal(t, "[\n    {\n        \"text\": \"<b>a & b</b> \ufffd\"\n    }\n]\n", out.s) //nolint:testifylint // the test checks the bytes of the encoding
}

type stringWriter struct{ s string }

func (w *stringWriter) Write(p []byte) (int, error) {
	w.s += string(p)
	return len(p), nil
}
