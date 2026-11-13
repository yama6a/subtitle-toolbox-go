package google

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v3/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v3/model"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/internal/httppost"
)

const key = "AIzaSyD-example+key/42"

var texts = []string{
	"The train to Basel leaves from platform 4 at <x1>10:15</x1>.",
	"<x1>Tickets &amp; seat\nreservations</x1> are sold here.",
	"- Is this seat free?\n- Yes, it is.",
	"<x1>The next stop is Zurich main station.</x1>",
}

func answer(status int, body string) *translationtest.Client {
	return &translationtest.Client{Answers: []translationtest.Answer{{Status: status, Body: []byte(body)}}}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		opts    Options
		option  string
		message string
	}{
		{
			"empty",
			Options{APIKey: " \t\n\r\x00\v"},
			"google.Options.APIKey",
			"The API key must not be empty. Pass the key of your Google Cloud project.",
		},
		{
			"space at the end",
			Options{APIKey: "key "},
			"google.Options.APIKey",
			"The API key must not have a control character, or a space at the start or end.",
		},
		{
			"control character",
			Options{APIKey: "ke\x7fy"},
			"google.Options.APIKey",
			"The API key must not have a control character, or a space at the start or end.",
		},
		{
			"base URL",
			Options{APIKey: "key", BaseURL: "ftp://host"},
			"google.Options.BaseURL",
			`The base URL must start with http:// or https://, got "ftp://host".`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.opts.Validate()
			var ia *model.InvalidArgumentError
			require.ErrorAs(t, err, &ia)
			assert.Equal(t, tt.option, ia.Option)
			assert.Equal(t, tt.message, ia.Msg)

			_, err = New(tt.opts)
			require.ErrorIs(t, err, model.ErrInvalidArgument)
		})
	}

	o := Options{APIKey: key, BaseURL: "https://proxy.example"}
	for _, s := range []string{fmt.Sprintf("%v %+v %#v", o, o, o), fmt.Sprint(&o)} {
		assert.NotContains(t, s, key)
		assert.Contains(t, s, "***")
	}
	assert.NoError(t, Options{APIKey: key}.Validate())
}

func TestTranslateRequest(t *testing.T) {
	t.Parallel()

	recorded := testfiles.Read(t, "translation/google_fr.json")
	client := &translationtest.Client{Answers: []translationtest.Answer{{Status: 200, Body: recorded}}}
	e, err := New(Options{APIKey: key, HTTPClient: client})
	require.NoError(t, err)

	got, err := e.Translate(context.Background(), texts, "en", "fr")

	require.NoError(t, err)
	assert.Equal(t, []string{
		"Le train pour Bâle part du quai 4 à <x1>10:15</x1>.",
		"<x1>Les billets &amp; les réservations\nde places</x1> sont vendus ici.",
		"- Cette place est-elle libre ?\n- Oui, c&#39;est libre.",
		"<x1>Le prochain arrêt est la gare centrale de Zurich.</x1>",
	}, got)
	reqs := client.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "https://translation.googleapis.com/language/translate/v2", reqs[0].URL)
	assert.Equal(t, key, reqs[0].Key)
	assert.Equal(t, "application/json", reqs[0].Header.Get("Content-Type"))
	want, err := httppost.JSON(texts)
	require.NoError(t, err)
	assert.Equal(t, `{"q":`+string(want)+`,"target":"fr","format":"html","source":"en"}`, string(reqs[0].Body))
}

func TestTranslateDetectsSourceAndUsesBaseURL(t *testing.T) {
	t.Parallel()

	client := answer(200, `{"data": {"translations": [{"translatedText": "l&#39;a &amp; &quot;b&quot; &lt;x1&gt; &#60; &eacute;", "detectedSourceLanguage": "de"}]}}`)
	e, err := New(Options{APIKey: "key", BaseURL: "http://127.0.0.1:8080/", HTTPClient: client})
	require.NoError(t, err)

	got, err := e.Translate(context.Background(), []string{"Lauf!"}, "", "fr")

	require.NoError(t, err)
	assert.Equal(t, []string{"l&#39;a &amp; &quot;b&quot; &lt;x1&gt; &#60; &eacute;"}, got)
	reqs := client.Requests()
	assert.Equal(t, "http://127.0.0.1:8080/language/translate/v2", reqs[0].URL)
	assert.Equal(t, `{"q":["Lauf!"],"target":"fr","format":"html"}`, string(reqs[0].Body)) //nolint:testifylint // the key order is part of the contract
}

func TestTranslateChunks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		count int
		want  []int
	}{{0, nil}, {128, []int{128}}, {300, []int{128, 128, 44}}}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.count), func(t *testing.T) {
			t.Parallel()

			in := make([]string, tt.count)
			for i := range in {
				in[i] = "text " + strconv.Itoa(i)
			}
			answers := []translationtest.Answer{{Status: 200}}
			if tt.count > 0 {
				answers = nil
				for chunk := range slices.Chunk(in, MaxTextsPerRequest) {
					answers = append(answers, translationtest.Answer{Status: 200, Body: translated(chunk)})
				}
			}
			client := &translationtest.Client{Answers: answers}
			e, err := New(Options{APIKey: key, HTTPClient: client})
			require.NoError(t, err)

			got, err := e.Translate(context.Background(), in, "en", "fr")

			require.NoError(t, err)
			assert.NotNil(t, got)
			require.Len(t, got, tt.count)
			reqs := client.Requests()
			sizes := make([]int, 0, len(reqs))
			for _, r := range reqs {
				sizes = append(sizes, len(sentTexts(t, r.Body)))
			}
			assert.Equal(t, tt.want, nilIfEmpty(sizes))
			if tt.count > 0 {
				assert.Equal(t, strings.ToUpper(in[tt.count-1]), got[tt.count-1])
			}
		})
	}
}

func TestTranslateErrors(t *testing.T) {
	t.Parallel()

	recordedError := string(testfiles.Read(t, "translation/google_error.json"))
	tests := []struct {
		name    string
		status  int
		body    string
		message string
	}{
		{
			"403", 403, `{"error": {"code": 403, "message": "Cloud Translation API has not been used in project 1."}}`,
			"translation: Google Cloud Translation refused the request (HTTP 403). Check the key and that the API is enabled. Cloud Translation API has not been used in project 1.",
		},
		{
			"429 after the retries", 429, `{"error": {"message": "Quota exceeded"}}`,
			"translation: Google Cloud Translation got too many requests (HTTP 429). Wait and try again.",
		},
		{
			"400 recorded", 400, recordedError,
			"translation: Google Cloud Translation answered with HTTP 400. API key not valid. Please pass a valid API key.",
		},
		{
			"400 key in the message", 400, `{"error": {"message": "Bad key ` + key + `"}}`,
			"translation: Google Cloud Translation answered with HTTP 400. Bad key ***",
		},
		{"418 without a message", 418, `{"error": "x"}`, "translation: Google Cloud Translation answered with HTTP 418."},
		{
			"not JSON", 200, "<html>",
			"translation: Google Cloud Translation returned 0 translations for 1 texts, or an answer that is not the JSON of translate v2.",
		},
		{
			"too few", 200, `{"data": {"translations": []}}`,
			"translation: Google Cloud Translation returned 0 translations for 1 texts, or an answer that is not the JSON of translate v2.",
		},
		{
			"not a string", 200, `{"data": {"translations": [{"translatedText": 5}]}}`,
			"translation: Google Cloud Translation returned 1 translations for 1 texts, or an answer that is not the JSON of translate v2.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			e, err := New(Options{APIKey: key, HTTPClient: answer(tt.status, tt.body)})
			require.NoError(t, err)
			e.wait = noWait

			_, err = e.Translate(context.Background(), []string{"Hello"}, "en", "fr")

			var se *translation.ServiceError
			require.ErrorAs(t, err, &se)
			require.ErrorIs(t, err, translation.ErrService)
			assert.Equal(t, tt.status, se.Status)
			assert.Equal(t, tt.message, err.Error())
			assert.NotContains(t, err.Error(), key)
		})
	}
}

func TestTranslateWithTheRunner(t *testing.T) {
	t.Parallel()

	server := translationtest.NewServer(t)
	e, err := New(Options{APIKey: key, BaseURL: server.URL})
	require.NoError(t, err)
	s := readSRT(t, "translation/own_station.srt")

	report, err := translation.Translate(context.Background(), s, e, "", "fr", translation.Options{})

	require.NoError(t, err)
	assert.Empty(t, report.Warnings)
	want := readSRT(t, "translation/own_station_fr_google.srt")
	require.Len(t, s.Cues(), len(want.Cues()))
	for i, c := range s.Cues() {
		assert.Equal(t, want.Cues()[i].Start, c.Start)
		assert.Equal(t, want.Cues()[i].End, c.End)
		assert.Equal(t, want.Cues()[i].Lines, c.Lines)
	}
	reqs := server.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, key, reqs[0].Key)
	assert.NotContains(t, string(reqs[0].Body), `"source"`)
}

func TestTranslateDecodesEntitiesOnce(t *testing.T) {
	t.Parallel()

	sent := []string{"Write &amp;lt;i&amp;gt;Run&amp;lt;/i&amp;gt; for italics.", "Tom &amp;amp; Jerry is a cat and mouse act."}
	client := &translationtest.Client{Answers: []translationtest.Answer{{Status: 200, Body: translatedAsIs(sent)}}}
	e, err := New(Options{APIKey: key, HTTPClient: client})
	require.NoError(t, err)
	s := readSRT(t, "translation/own_entities.srt")
	before := lines(s)

	_, err = translation.Translate(context.Background(), s, e, "en", "fr", translation.Options{})

	require.NoError(t, err)
	assert.Equal(t, before, lines(s))
	assert.Equal(t, sent, sentTexts(t, client.Requests()[0].Body))
}

func TestTranslateRetries(t *testing.T) {
	t.Parallel()

	client := &translationtest.Client{Answers: []translationtest.Answer{
		{Status: 429}, {Status: 503}, {Status: 200, Body: testfiles.Read(t, "translation/google_fr.json")},
	}}
	e, err := New(Options{APIKey: key, HTTPClient: client})
	require.NoError(t, err)
	var waits []time.Duration
	e.wait = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}

	got, err := e.Translate(context.Background(), texts, "en", "fr")

	require.NoError(t, err)
	assert.Len(t, got, 4)
	reqs := client.Requests()
	require.Len(t, reqs, 3)
	assert.Equal(t, reqs[0].Body, reqs[2].Body)
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, waits)
}

func nilIfEmpty(s []int) []int {
	if len(s) == 0 {
		return nil
	}
	return s
}

func noWait(context.Context, time.Duration) error { return nil }

type translationItem struct {
	TranslatedText string `json:"translatedText"`
}

func answerBody(t []string) []byte {
	var out struct {
		Data struct {
			Translations []translationItem `json:"translations"`
		} `json:"data"`
	}
	for _, s := range t {
		out.Data.Translations = append(out.Data.Translations, translationItem{s})
	}
	b, err := json.Marshal(out)
	if err != nil {
		panic(err)
	}
	return b
}

// translated returns an answer that holds each text in upper case.
func translated(texts []string) []byte {
	up := make([]string, len(texts))
	for i, s := range texts {
		up[i] = strings.ToUpper(s)
	}
	return answerBody(up)
}

// translatedAsIs returns an answer that holds each text unchanged.
func translatedAsIs(texts []string) []byte { return answerBody(texts) }

func sentTexts(t *testing.T, body []byte) []string {
	t.Helper()

	var in struct {
		Q []string `json:"q"`
	}
	require.NoError(t, json.Unmarshal(body, &in))
	return in.Q
}

func readSRT(t *testing.T, name string) *model.Subtitle {
	t.Helper()

	s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, name)), model.CommonReadOptions{}, srt.ReadOptions{})
	require.NoError(t, err)
	return s
}

func lines(s *model.Subtitle) [][]string {
	out := make([][]string, 0, len(s.Cues()))
	for _, c := range s.Cues() {
		out = append(out, c.Lines)
	}
	return out
}
