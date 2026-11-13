package deepl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/format/webvtt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation"
)

const key = "0f6c2a8e-6c1d-4b5e-9f3a-2d7e8b1c4a90"

var stationTexts = []string{
	"The train to Basel leaves from platform 4 at <x1>10:15</x1>.",
	"<x1>Tickets &amp; seat\nreservations</x1> are sold here.",
	"- Is this seat free?\n- Yes, it is.",
	"<x1>The next stop is Zurich main station.</x1>",
}

func answer(tb testing.TB, status int, file string) translationtest.Answer {
	tb.Helper()
	return translationtest.Answer{Status: status, Body: testfiles.Read(tb, "translation/"+file)}
}

func newEngine(tb testing.TB, opts Options, answers ...translationtest.Answer) (*Engine, *translationtest.Client, *[]time.Duration) {
	tb.Helper()
	client := &translationtest.Client{Answers: answers}
	opts.HTTPClient = client
	e, err := New(opts)
	require.NoError(tb, err)
	var waits []time.Duration
	e.wait = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	return e, client, &waits
}

func body(tb testing.TB, r translationtest.Request) map[string]any {
	tb.Helper()
	var m map[string]any
	require.NoError(tb, json.Unmarshal(r.Body, &m))
	return m
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts Options
		want string
	}{
		{"ok", Options{APIKey: key}, ""},
		{"empty", Options{APIKey: ""}, "deepl.Options.APIKey: The API key must not be empty. Pass the key of your DeepL account."},
		{"blank", Options{APIKey: " \t\n\r\x00\v"}, "deepl.Options.APIKey: The API key must not be empty. Pass the key of your DeepL account."},
		{"space at the end", Options{APIKey: key + " "}, "deepl.Options.APIKey: The API key must not have a control character, or a space at the start or end."},
		{"control character", Options{APIKey: "ab\x7fcd"}, "deepl.Options.APIKey: The API key must not have a control character, or a space at the start or end."},
		{"base URL", Options{APIKey: key, BaseURL: "ftp://host"}, `deepl.Options.BaseURL: The base URL must start with http:// or https://, got "ftp://host".`},
		{"base URL without host", Options{APIKey: key, BaseURL: "https:///x"}, `deepl.Options.BaseURL: The base URL must start with http:// or https://, got "https:///x".`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			if tt.want == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, model.ErrInvalidArgument)
			assert.Contains(t, err.Error(), tt.want)
			_, err = New(tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}

	o := Options{APIKey: key}
	assert.NotContains(t, fmt.Sprintf("%v %+v %#v", o, o, o), key)
}

func TestTranslateRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		opts         Options
		source, dest string
		wantURL      string
		wantBody     map[string]any
	}{
		{
			"paid host, source and target",
			Options{APIKey: key},
			"en", "de", "https://api.deepl.com/v2/translate",
			map[string]any{"text": []any{"Hello"}, "target_lang": "DE", "tag_handling": "xml", "split_sentences": "nonewlines", "source_lang": "EN"},
		},
		{
			"free key",
			Options{APIKey: "abc:fx"},
			"en", "de", "https://api-free.deepl.com/v2/translate",
			map[string]any{"text": []any{"Hello"}, "target_lang": "DE", "tag_handling": "xml", "split_sentences": "nonewlines", "source_lang": "EN"},
		},
		{
			"base URL wins over the free key",
			Options{APIKey: "abc:fx", BaseURL: "http://127.0.0.1:8080/"},
			"en", "de", "http://127.0.0.1:8080/v2/translate",
			map[string]any{"text": []any{"Hello"}, "target_lang": "DE", "tag_handling": "xml", "split_sentences": "nonewlines", "source_lang": "EN"},
		},
		{
			"no source, region in the target",
			Options{APIKey: key},
			"", "en-US", "https://api.deepl.com/v2/translate",
			map[string]any{"text": []any{"Hello"}, "target_lang": "EN-US", "tag_handling": "xml", "split_sentences": "nonewlines"},
		},
		{
			"region in the source",
			Options{APIKey: key},
			"en-US", "de", "https://api.deepl.com/v2/translate",
			map[string]any{"text": []any{"Hello"}, "target_lang": "DE", "tag_handling": "xml", "split_sentences": "nonewlines", "source_lang": "EN"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, client, _ := newEngine(t, tt.opts, translationtest.Answer{Status: 200, Body: []byte(`{"translations": [{"text": "Hallo"}]}`)})
			got, err := e.Translate(t.Context(), []string{"Hello"}, tt.source, tt.dest)
			require.NoError(t, err)
			assert.Equal(t, []string{"Hallo"}, got)
			reqs := client.Requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, tt.wantURL, reqs[0].URL)
			assert.Equal(t, tt.wantBody, body(t, reqs[0]))
			assert.Equal(t, "DeepL-Auth-Key "+tt.opts.APIKey, reqs[0].Header.Get("Authorization"))
			assert.Equal(t, "application/json", reqs[0].Header.Get("Content-Type"))
		})
	}

	t.Run("body key order", func(t *testing.T) {
		t.Parallel()

		e, client, _ := newEngine(t, Options{APIKey: key}, translationtest.Answer{Status: 200, Body: []byte(`{"translations": [{"text": "a"}]}`)})
		_, err := e.Translate(t.Context(), []string{"<x1>a</x1> / b"}, "en", "de")
		require.NoError(t, err)
		want := `{"text":["<x1>a</x1> / b"],"target_lang":"DE","tag_handling":"xml","split_sentences":"nonewlines","source_lang":"EN"}`
		assert.True(t, bytes.Equal([]byte(want), client.Requests()[0].Body), "body %s", client.Requests()[0].Body)
	})
}

func TestTranslateChunks(t *testing.T) {
	t.Parallel()

	texts := make([]string, 120)
	for i := range texts {
		texts[i] = fmt.Sprintf("text %d", i+1)
	}
	reply := func(from, to int) translationtest.Answer {
		entries := make([]string, 0, to-from)
		for _, text := range texts[from:to] {
			entries = append(entries, fmt.Sprintf(`{"text": %q}`, text))
		}
		return translationtest.Answer{Status: 200, Body: []byte(`{"translations": [` + strings.Join(entries, ",") + `]}`)}
	}
	e, client, _ := newEngine(t, Options{APIKey: key}, reply(0, 50), reply(50, 100), reply(100, 120))

	got, err := e.Translate(t.Context(), texts, "en", "de")
	require.NoError(t, err)
	assert.Equal(t, texts, got)
	reqs := client.Requests()
	sizes := make([]int, 0, len(reqs))
	for _, r := range reqs {
		sizes = append(sizes, len(body(t, r)["text"].([]any)))
	}
	assert.Equal(t, []int{50, 50, 20}, sizes)

	got, err = e.Translate(t.Context(), nil, "en", "de")
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Len(t, client.Requests(), 3)
}

func TestTranslateErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		answer translationtest.Answer
		texts  int
		status int
		want   string
	}{
		{"403", translationtest.Answer{Status: 403, Body: []byte(`{"message": "Forbidden"}`)}, 1, 403, "DeepL rejected the API key (HTTP 403). Check the key and its plan."},
		{"429 after the retries", translationtest.Answer{Status: 429, Body: []byte(`{"message": "Too many requests"}`)}, 1, 429, "DeepL got too many requests (HTTP 429). Wait and try again."},
		{"456", translationtest.Answer{Status: 456, Body: []byte(`{"message": "Quota exceeded"}`)}, 1, 456, "The DeepL character quota is used up (HTTP 456)."},
		{"400 recorded", answer(t, 400, "deepl_error.json"), 1, 400, "DeepL answered with HTTP 400. Value for 'target_lang' not supported."},
		{"500 key in the body", translationtest.Answer{Status: 500, Body: []byte(`{"message": "Unknown key ` + key + `"}`)}, 1, 500, "DeepL answered with HTTP 500. Unknown key ***"},
		{"418 without a message", translationtest.Answer{Status: 418, Body: []byte(`{"message": 7}`)}, 1, 418, "DeepL answered with HTTP 418."},
		{
			"too few translations",
			translationtest.Answer{Status: 200, Body: []byte(`{"translations": [{"text": "Hallo"}]}`)},
			2, 200,
			"DeepL returned 1 translations for 2 texts, or an answer that is not the JSON of /v2/translate.",
		},
		{
			"text is not a string",
			translationtest.Answer{Status: 200, Body: []byte(`{"translations": [{"text": 5}]}`)},
			1, 200,
			"DeepL returned 1 translations for 1 texts, or an answer that is not the JSON of /v2/translate.",
		},
		{
			"no translations",
			translationtest.Answer{Status: 200, Body: []byte(`<html>`)},
			1, 200,
			"DeepL returned 0 translations for 1 texts, or an answer that is not the JSON of /v2/translate.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, _, _ := newEngine(t, Options{APIKey: key}, tt.answer)
			_, err := e.Translate(t.Context(), make([]string, tt.texts), "en", "de")
			var se *translation.ServiceError
			require.ErrorAs(t, err, &se)
			require.ErrorIs(t, err, translation.ErrService)
			assert.Equal(t, tt.status, se.Status)
			assert.Equal(t, tt.want, se.Msg)
			assert.NotContains(t, err.Error(), key)
		})
	}
}

func TestTranslateRetries(t *testing.T) {
	t.Parallel()

	e, client, waits := newEngine(t, Options{APIKey: key},
		translationtest.Answer{Status: 429, Body: []byte(`{"message": "busy"}`)},
		translationtest.Answer{Status: 503},
		answer(t, 200, "deepl_de.json"))

	got, err := e.Translate(t.Context(), stationTexts, "en", "de")
	require.NoError(t, err)
	assert.Equal(t, []string{
		"Der Zug nach Basel fährt um <x1>10:15</x1> von Gleis 4 ab.",
		"<x1>Fahrkarten &amp; Platz-\nreservierungen</x1> gibt es hier.",
		"- Ist dieser Platz frei?\n- Ja, ist er.",
		"<x1>Der nächste Halt ist Zürich Hauptbahnhof.</x1>",
	}, got)
	reqs := client.Requests()
	require.Len(t, reqs, 3)
	assert.Equal(t, reqs[0].Body, reqs[2].Body)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, *waits)
}

func readSRT(t *testing.T, file string) *model.Subtitle {
	t.Helper()
	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, "translation/"+file)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	return s
}

type cueView struct {
	Start, End time.Duration
	Lines      []string
}

func view(s *model.Subtitle) []cueView {
	out := make([]cueView, 0, len(s.Cues()))
	for _, c := range s.Cues() {
		out = append(out, cueView{c.Start, c.End, c.Lines})
	}
	return out
}

func TestTranslateWithTheRunner(t *testing.T) {
	t.Parallel()

	station, _, err := webvtt.Read(bytes.NewReader(testfiles.Read(t, "translation/own_station_de_deepl.vtt")), model.CommonReadOptions{}, webvtt.ReadOptions{})
	require.NoError(t, err)

	tests := []struct {
		name   string
		input  string
		answer translationtest.Answer
		want   []cueView
		sent   string
	}{
		{"own_station.srt", "own_station.srt", answer(t, 200, "deepl_de.json"), view(station), ""},
		{
			"own_entities.srt", "own_entities.srt", answer(t, 200, "deepl_entities_de.json"), view(readSRT(t, "own_entities_de_deepl.srt")),
			"Write &amp;lt;i&amp;gt;Run&amp;lt;/i&amp;gt; for italics.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, client, _ := newEngine(t, Options{APIKey: key}, tt.answer)
			s := readSRT(t, tt.input)
			before := view(s)

			report, err := translation.Translate(t.Context(), s, e, "en", "de", translation.Options{})
			require.NoError(t, err)
			assert.Empty(t, report.Warnings)
			require.Len(t, client.Requests(), 1)
			if tt.sent != "" {
				assert.Equal(t, tt.sent, body(t, client.Requests()[0])["text"].([]any)[0])
			}
			got := view(s)
			assert.Equal(t, tt.want, got)
			for i := range got {
				assert.Equal(t, before[i].Start, got[i].Start)
				assert.Equal(t, before[i].End, got[i].End)
			}
		})
	}
}

func TestTranslateWithTheServer(t *testing.T) {
	t.Parallel()

	srv := translationtest.NewServer(t)
	e, err := New(Options{APIKey: "fine", BaseURL: srv.URL})
	require.NoError(t, err)
	got, err := e.Translate(t.Context(), stationTexts, "en", "de")
	require.NoError(t, err)
	assert.Len(t, got, 4)
}

func TestTranslateNoNetwork(t *testing.T) {
	t.Parallel()

	e, err := New(Options{APIKey: key, BaseURL: translationtest.ClosedURL(t)})
	require.NoError(t, err)
	_, err = e.Translate(t.Context(), []string{"Hello"}, "en", "de")
	require.ErrorIs(t, err, translation.ErrService)
	assert.NotContains(t, err.Error(), key)
}
