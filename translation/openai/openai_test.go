package openai_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yama6a/subtitle-toolbox-go/v4/format/srt"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation/openai"
)

const (
	key     = "sk-proj-4f1c9e2a7b3d"
	baseURL = "http://localhost:11434/v1/"
)

var texts = []string{
	"The train to Basel leaves from platform 4 at <x1>10:15</x1>.",
	"<x1>Tickets &amp; seat\nreservations</x1> are sold here.",
	"- Is this seat free?\n- Yes, it is.",
	"<x1>The next stop is Zurich main station.</x1>",
}

var german = []string{
	"Der Zug nach Basel fährt um <x1>10:15</x1> von Gleis 4 ab.",
	"<x1>Fahrkarten &amp; Platz-\nreservierungen</x1> gibt es hier.",
	"- Ist dieser Platz frei?\n- Ja, ist er.",
	"<x1>Der nächste Halt ist Zürich Hauptbahnhof.</x1>",
}

func chat(content string) []byte {
	b, err := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content}}}})
	if err != nil {
		panic(err)
	}
	return b
}

func ok(body []byte) translationtest.Answer { return translationtest.Answer{Status: 200, Body: body} }

func jsonArray(t *testing.T, v ...string) string {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func newEngine(t *testing.T, client translation.HTTPClient, opts openai.Options) *openai.Engine {
	t.Helper()

	opts.BaseURL = baseURL
	opts.Model = "llama3"
	opts.HTTPClient = client
	e, err := openai.New(opts)
	require.NoError(t, err)
	return e
}

func userTexts(t *testing.T, r translationtest.Request) []string {
	t.Helper()

	var body struct {
		Messages []struct {
			Content string `json:"content"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(r.Body, &body))
	var got []string
	require.NoError(t, json.Unmarshal([]byte(body.Messages[1].Content), &got))
	return got
}

func TestValidate(t *testing.T) {
	t.Parallel()

	const apiKeyMsg = "The API key must not be empty, have a control character, or a space at the start or end. Leave it empty for a service without a key."
	good := openai.Options{BaseURL: "https://api.openai.com/v1", Model: "gpt-4o-mini"}
	tests := []struct {
		name   string
		mutate func(*openai.Options)
		option string
		msg    string
	}{
		{"valid", func(*openai.Options) {}, "", ""},
		{"valid with key and prompt", func(o *openai.Options) { o.APIKey = key; o.Prompt = "x" }, "", ""},
		{
			"base URL without a scheme", func(o *openai.Options) { o.BaseURL = "localhost:11434/v1" }, "openai.Options.BaseURL",
			`The base URL must start with http:// or https://, got "localhost:11434/v1".`,
		},
		{
			"base URL without a host", func(o *openai.Options) { o.BaseURL = "http:///v1" }, "openai.Options.BaseURL",
			`The base URL must start with http:// or https://, got "http:///v1".`,
		},
		{
			"empty model", func(o *openai.Options) { o.Model = " \t" }, "openai.Options.Model",
			"The model must not be empty. Pass a model name such as gpt-4o-mini or llama3.",
		},
		{"blank key", func(o *openai.Options) { o.APIKey = " " }, "openai.Options.APIKey", apiKeyMsg},
		{"key with a space at the end", func(o *openai.Options) { o.APIKey = "k " }, "openai.Options.APIKey", apiKeyMsg},
		{"key with a control character", func(o *openai.Options) { o.APIKey = "k\x7f" }, "openai.Options.APIKey", apiKeyMsg},
		{
			"blank prompt", func(o *openai.Options) { o.Prompt = "\n" }, "openai.Options.Prompt",
			"The prompt must not be empty. Leave it empty for the built-in prompt.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			o := good
			tt.mutate(&o)

			err := o.Validate()
			_, newErr := openai.New(o)

			if tt.option == "" {
				require.NoError(t, err)
				require.NoError(t, newErr)
				return
			}
			var ia *model.InvalidArgumentError
			require.ErrorAs(t, err, &ia)
			require.ErrorIs(t, newErr, model.ErrInvalidArgument)
			assert.Equal(t, tt.option, ia.Option)
			if tt.msg != "" {
				assert.Equal(t, tt.msg, ia.Msg)
			}
		})
	}
}

func TestOptionsPrintHideTheKey(t *testing.T) {
	t.Parallel()

	o := openai.Options{BaseURL: baseURL, Model: "llama3", APIKey: key}

	for _, verb := range []string{"%v", "%+v", "%#v", "%s"} {
		got := fmt.Sprintf(verb, o)
		assert.NotContains(t, got, key, verb)
		assert.Contains(t, got, "***", verb)
	}
	assert.NotContains(t, fmt.Sprintf("%v", openai.Options{Model: "m"}), "***")
}

func TestTranslateSends(t *testing.T) {
	t.Parallel()

	prompt := "You translate subtitle text from en to de. The user sends a JSON array of strings. " +
		"Translate each string on its own. Keep the tags such as <x1>, </x1> and <x2/> around the words that " +
		"they mark, and keep the entities &lt;, &gt; and &amp;. Keep the line breaks. Answer only with a JSON " +
		"array of strings that has as many strings as the input, in the same order, and no other text."
	tests := []struct {
		name           string
		opts           openai.Options
		source, target string
		wantAuth       string
		wantPrompt     string
	}{
		{"no key, built-in prompt", openai.Options{}, "en", "de", "", prompt},
		{"key as a bearer token", openai.Options{APIKey: key}, "en", "de", "Bearer " + key, prompt},
		{
			"own prompt and an empty source",
			openai.Options{Prompt: "Translate {source} into {target} for children."},
			"", "de-CH", "",
			"Translate the language of the text into de-CH for children.",
		},
		{"replacements run once", openai.Options{Prompt: "{source}/{target}"}, "{target}", "x", "", "{target}/x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &translationtest.Client{Answers: []translationtest.Answer{ok(chat(`["Hallo"]`))}}

			_, err := newEngine(t, client, tt.opts).Translate(t.Context(), []string{"Hello <x1>there</x1>/"}, tt.source, tt.target)

			require.NoError(t, err)
			reqs := client.Requests()
			require.Len(t, reqs, 1)
			assert.Equal(t, "http://localhost:11434/v1/chat/completions", reqs[0].URL)
			assert.Equal(t, "application/json", reqs[0].Header.Get("Content-Type"))
			assert.Equal(t, tt.wantAuth, reqs[0].Header.Get("Authorization"))
			want, err := json.Marshal(tt.wantPrompt)
			require.NoError(t, err)
			wantBody := `{"model":"llama3","messages":[{"role":"system","content":` + string(want) +
				`},{"role":"user","content":"[\"Hello <x1>there</x1>/\"]"}]}`
			assert.JSONEq(t, wantBody, string(reqs[0].Body))
			assert.Contains(t, string(reqs[0].Body), `<x1>there</x1>/`)
			assert.True(t, strings.HasPrefix(string(reqs[0].Body), `{"model":"llama3","messages":[{"role":"system"`))
		})
	}
}

func TestTranslateNoTextsSendsNoRequest(t *testing.T) {
	t.Parallel()

	client := &translationtest.Client{Answers: []translationtest.Answer{ok(chat(`[]`))}}

	got, err := newEngine(t, client, openai.Options{}).Translate(t.Context(), nil, "en", "de")

	require.NoError(t, err)
	assert.Equal(t, []string{}, got)
	assert.Empty(t, client.Requests())
}

func TestTranslateAnswers(t *testing.T) {
	t.Parallel()

	single := func(i int) translationtest.Answer { return ok(chat(jsonArray(t, german[i]))) }
	singles := []translationtest.Answer{single(0), single(1), single(2), single(3)}
	reasoning := func(field string) []byte {
		return []byte(strings.Replace(string(testfiles.Read(t, "translation/openai_reasoning_de.json")), `"reasoning"`, `"`+field+`"`, 1))
	}
	both, err := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": `["Hallo", "Tschüss", "Ja", "Nein"]`, "reasoning": `["Hi", "Ciao", "Ja", "Nein"]`,
	}}}})
	require.NoError(t, err)
	sureAndArray, err := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{
		"content": "Sure!", "reasoning": jsonArray(t, texts...),
	}}}})
	require.NoError(t, err)
	empty, err := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "", "reasoning": ""}}}})
	require.NoError(t, err)

	tests := []struct {
		name         string
		answers      []translationtest.Answer
		want         []string
		wantRequests int
		wantErr      string
	}{
		{"an array of 4", []translationtest.Answer{ok(testfiles.Read(t, "translation/openai_de.json"))}, german, 1, ""},
		{"too few strings", append([]translationtest.Answer{ok(chat(`["Der Zug", "Fahrkarten"]`))}, singles...), german, 5, ""},
		{"too many strings", append([]translationtest.Answer{ok(chat(`["a", "b", "c", "d", "e"]`))}, singles...), german, 5, ""},
		{"no JSON", append([]translationtest.Answer{ok(chat("Sure! Here is the translation."))}, singles...), german, 5, ""},
		{"cut-off array", append([]translationtest.Answer{ok(chat(`["Der Zug", "Fahrkarten", "Ist dieser", "Der nächste`))}, singles...), german, 5, ""},
		{"numbers", append([]translationtest.Answer{ok(chat(`[1, 2, 3, 4]`))}, singles...), german, 5, ""},
		{"an object", append([]translationtest.Answer{ok(chat(`{"text": "Der Zug"}`))}, singles...), german, 5, ""},
		{
			"think block and code block",
			[]translationtest.Answer{
				ok(chat("<think>The user wants [German].</think>\n```json\n[\"Hallo\", \"Tschüss\", \"Ja\", \"Nein\"]\n```")),
			},
			[]string{"Hallo", "Tschüss", "Ja", "Nein"},
			1, "",
		},
		{"array in reasoning", []translationtest.Answer{ok(reasoning("reasoning"))}, german, 1, ""},
		{"array in reasoning_content", []translationtest.Answer{ok(reasoning("reasoning_content"))}, german, 1, ""},
		{"a later field is not read after an unusable first field", append([]translationtest.Answer{ok(sureAndArray)}, singles...), german, 5, ""},
		{"a later choice is not read", []translationtest.Answer{ok([]byte(`{"choices":[{"message":{"content":` + string(mustJSON(t, jsonArray(t, german...))) + `}}, 5]}`))}, german, 1, ""},
		{"content before reasoning", []translationtest.Answer{ok(both)}, []string{"Hallo", "Tschüss", "Ja", "Nein"}, 1, ""},
		{
			"empty content and reasoning",
			[]translationtest.Answer{ok(empty)},
			nil, 2,
			`The model "llama3" did not answer with a JSON array that holds 1 string for 1 text.`,
		},
		{
			"a fallback answer without an array",
			[]translationtest.Answer{ok(chat("Hallo"))},
			nil, 2,
			`The model "llama3" did not answer with a JSON array that holds 1 string for 1 text.`,
		},
		{
			"no JSON body",
			[]translationtest.Answer{ok([]byte("<html>Welcome</html>"))},
			nil, 2,
			`The model "llama3" did not answer with a JSON array that holds 1 string for 1 text.`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &translationtest.Client{Answers: tt.answers}

			got, err := newEngine(t, client, openai.Options{}).Translate(t.Context(), texts, "en", "de")

			reqs := client.Requests()
			assert.Len(t, reqs, tt.wantRequests)
			if tt.wantErr != "" {
				var se *translation.ServiceError
				require.ErrorAs(t, err, &se)
				assert.Equal(t, "translation: "+tt.wantErr, se.Error())
				assert.Zero(t, se.Status)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
			if tt.wantRequests == 5 {
				assert.Equal(t, texts, userTexts(t, reqs[0]))
				for i := range texts {
					assert.Equal(t, texts[i:i+1], userTexts(t, reqs[i+1]))
				}
			}
		})
	}
}

func TestTranslateWithServer(t *testing.T) {
	t.Parallel()

	srv := translationtest.NewServer(t)
	e, err := openai.New(openai.Options{BaseURL: srv.URL + "/v1", Model: "llama3", APIKey: "local-key"})
	require.NoError(t, err)

	got, err := e.Translate(t.Context(), texts, "en", "de")

	require.NoError(t, err)
	assert.Equal(t, german, got)
	reqs := srv.Requests()
	require.Len(t, reqs, 1)
	assert.Equal(t, "/v1/chat/completions", reqs[0].Path)
	assert.Equal(t, "local-key", reqs[0].Key)
}

func TestTranslateClosedPort(t *testing.T) {
	t.Parallel()

	e, err := openai.New(openai.Options{BaseURL: translationtest.ClosedURL(t) + "/v1", Model: "llama3", APIKey: key})
	require.NoError(t, err)

	_, err = e.Translate(t.Context(), []string{"Hello"}, "en", "de")

	var se *translation.ServiceError
	require.ErrorAs(t, err, &se)
	assert.Zero(t, se.Status)
	assert.NotContains(t, err.Error(), key)
}

func TestRunnerOnOwnStation(t *testing.T) {
	t.Parallel()

	dropped := append([]string(nil), german...)
	dropped[3] = "Der nächste Halt ist Zürich Hauptbahnhof."
	tests := []struct {
		name         string
		answer       translationtest.Answer
		wantGolden   bool
		wantWarnings []int
	}{
		{"recorded answer", ok(testfiles.Read(t, "translation/openai_de.json")), true, nil},
		{"dropped placeholders", ok(chat(jsonArray(t, dropped...))), false, []int{6, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, _, err := srt.Read(bytes.NewReader(testfiles.Read(t, "translation/own_station.srt")), model.CommonReadOptions{}, srt.ReadOptions{})
			require.NoError(t, err)
			client := &translationtest.Client{Answers: []translationtest.Answer{tt.answer}}

			report, err := translation.Translate(t.Context(), s, newEngine(t, client, openai.Options{}), "en", "de", translation.Options{})

			require.NoError(t, err)
			assert.Len(t, client.Requests(), 1)
			var warned []int
			for _, w := range report.Warnings {
				warned = append(warned, w.Cue)
			}
			assert.Equal(t, tt.wantWarnings, warned)
			if tt.wantGolden {
				var out bytes.Buffer
				_, err := srt.Write(&out, s, model.CommonWriteOptions{}, srt.WriteOptions{})
				require.NoError(t, err)
				assert.Equal(t, string(testfiles.Read(t, "translation/own_station_de_openai.srt")), out.String())
				return
			}
			cues := s.Cues()
			assert.Equal(t, []string{"Der nächste Halt ist"}, cues[6].Lines)
			assert.Equal(t, []string{"Zürich Hauptbahnhof."}, cues[7].Lines)
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()

	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
