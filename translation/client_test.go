package translation_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/translation"
)

func TestDefaultHTTPClientFollowsNoRedirect(t *testing.T) {
	t.Parallel()

	var reached atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached.Add(1) }))
	t.Cleanup(second.Close)
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(first.Close)

	for _, method := range []string{http.MethodPost, http.MethodGet} {
		req, err := http.NewRequestWithContext(t.Context(), method, first.URL, nil)
		require.NoError(t, err)
		req.Header.Set("X-Goog-Api-Key", "secret-key")

		resp, err := translation.DefaultHTTPClient().Do(req)

		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		assert.Equal(t, http.StatusTemporaryRedirect, resp.StatusCode)
	}
	assert.Zero(t, reached.Load())
}
