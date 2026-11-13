package translationtest_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/translationtest"
)

const deepL, google, openAI = "/v2/translate", "/language/translate/v2", "/v1/chat/completions"

func post(t *testing.T, url string, header http.Header, body string) (int, []byte) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	req.Header = header
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	answer, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, answer
}

func TestServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		header     http.Header
		key        string
		wantStatus int
		wantFile   string
		wantBody   string
	}{
		{"DeepL answer", deepL, http.Header{"Authorization": {"DeepL-Auth-Key secret"}}, "secret", 200, "deepl_de.json", ""},
		{"DeepL without a key", deepL, nil, "", 200, "deepl_de.json", ""},
		{"DeepL forbidden", deepL, http.Header{"Authorization": {"DeepL-Auth-Key forbidden"}}, "forbidden", 403, "", `{"message": "error"}`},
		{"DeepL busy", deepL, http.Header{"Authorization": {"DeepL-Auth-Key busy"}}, "busy", 429, "", `{"message": "error"}`},
		{"DeepL quota", deepL, http.Header{"Authorization": {"DeepL-Auth-Key quota"}}, "quota", 456, "", `{"message": "error"}`},
		{"DeepL bad request", deepL, http.Header{"Authorization": {"DeepL-Auth-Key bad-request"}}, "bad-request", 400, "deepl_error.json", ""},
		{"Google answer", google, http.Header{"X-Goog-Api-Key": {"secret"}}, "secret", 200, "google_fr.json", ""},
		{"Google bad request", google, http.Header{"X-Goog-Api-Key": {"bad-request"}}, "bad-request", 400, "google_error.json", ""},
		{"Google forbidden", google, http.Header{"X-Goog-Api-Key": {"forbidden"}}, "forbidden", 403, "", `{"message": "error"}`},
		{"OpenAI answer", openAI, http.Header{"Authorization": {"Bearer secret"}}, "secret", 200, "openai_de.json", ""},
		{"OpenAI without a key", openAI, nil, "", 200, "openai_de.json", ""},
		{"OpenAI bad request", openAI, http.Header{"Authorization": {"Bearer bad-request"}}, "bad-request", 400, "openai_error.json", ""},
		{"other path", "/v3/other", http.Header{"Authorization": {"DeepL-Auth-Key secret"}}, "", 404, "", `{"message": "not found"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := translationtest.NewServer(t)

			status, body := post(t, server.URL+tt.path, tt.header, `{"q":"<x1>"}`)

			assert.Equal(t, tt.wantStatus, status)
			want := []byte(tt.wantBody)
			if tt.wantFile != "" {
				want = testfiles.Read(t, "translation/"+tt.wantFile)
			}
			assert.Equal(t, want, body)
			requests := server.Requests()
			require.Len(t, requests, 1)
			assert.Equal(t, tt.path, requests[0].Path)
			assert.Equal(t, tt.key, requests[0].Key)
			assert.Equal(t, server.URL+tt.path, requests[0].URL)
			assert.JSONEq(t, `{"q":"<x1>"}`, string(requests[0].Body))
		})
	}
}

func TestClient(t *testing.T) {
	t.Parallel()

	client := &translationtest.Client{Answers: []translationtest.Answer{
		{Status: 429, Header: http.Header{"Retry-After": {"1"}}, Body: []byte("busy")},
		{Status: 200, Body: []byte("ok")},
	}}
	got := make([]string, 0, 3)
	for range 3 {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://translate.test"+google, bytes.NewReader([]byte("{}")))
		require.NoError(t, err)
		req.Header.Set("X-Goog-Api-Key", "secret")
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		got = append(got, resp.Status+" "+string(body))
	}

	assert.Equal(t, []string{"429 Too Many Requests busy", "200 OK ok", "200 OK ok"}, got)
	requests := client.Requests()
	require.Len(t, requests, 3)
	assert.Equal(t, "secret", requests[2].Key)
	assert.Equal(t, google, requests[2].Path)
	assert.Equal(t, "{}", string(requests[2].Body))
}
