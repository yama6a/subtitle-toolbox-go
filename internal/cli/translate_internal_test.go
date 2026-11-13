package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	stbox "github.com/yama6a/subtitle-toolbox-go/v3"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/testfiles"
	"github.com/yama6a/subtitle-toolbox-go/v3/internal/translationtest"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/deepl"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/google"
	"github.com/yama6a/subtitle-toolbox-go/v3/translation/openai"
)

// translateDir returns a directory with own_station.srt, and latin1.srt as latin1.srt and second.srt.
func translateDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	copyFile(t, testfiles.Path(t, "translation/own_station.srt"), filepath.Join(dir, "own_station.srt"))
	for _, name := range []string{"latin1.srt", "second.srt"} {
		copyFile(t, testfiles.Path(t, "cli/latin1.srt"), filepath.Join(dir, name))
	}
	return dir
}

// translateCall runs stbox translate against base, the address that every engine uses.
func translateCall(t *testing.T, dir, base, stdin string, env map[string]string, words ...string) result {
	t.Helper()

	return call{
		dir: dir, stdin: stdin, env: env, commands: []command{&translateCommand{baseURL: base}},
	}.run(t, append([]string{"translate"}, words...)...)
}

// translatedByLibrary translates own_station.srt with engine as the library does and writes it in format f.
func translatedByLibrary(t *testing.T, engine translation.Engine, source, target string, f stbox.Format) string {
	t.Helper()

	s, _, err := stbox.Load(testfiles.Path(t, "translation/own_station.srt"), stbox.ReadOptions{})
	require.NoError(t, err)
	_, err = translation.Translate(t.Context(), s, engine, source, target, translation.Options{})
	require.NoError(t, err)
	var buf bytes.Buffer
	_, err = stbox.Write(&buf, s, f, stbox.WriteOptions{})
	require.NoError(t, err)
	return buf.String()
}

// Adapted from ApplicationTranslateTest.
func TestTranslateWritesWhatTheLibraryWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		env    map[string]string
		words  []string
		engine func(url string) translation.Engine
		source string
		target string
		format stbox.Format
		stdout string
		path   string
		key    string
		body   []string
	}{
		{
			name:  "deepl with the key of the environment",
			env:   map[string]string{"DEEPL_API_KEY": "env-key", "GOOGLE_TRANSLATE_API_KEY": "google-key"},
			words: []string{"own_station.srt", "--engine", "DeepL", "--source-language", "en", "--target-language", "de", "--to", "vtt"},
			engine: func(url string) translation.Engine {
				e, _ := deepl.New(deepl.Options{APIKey: "x", BaseURL: url})
				return e
			},
			source: "en", target: "de", format: stbox.WebVTT,
			path: "/v2/translate", key: "env-key", body: []string{`"target_lang":"DE"`, `"source_lang":"EN"`},
		},
		{
			name:  "google with --api-key",
			env:   map[string]string{"GOOGLE_TRANSLATE_API_KEY": "env-key"},
			words: []string{"own_station.srt", "--engine", "google", "--api-key", "cli-key", "--target-language", "fr", "-o", "out.srt"},
			engine: func(url string) translation.Engine {
				e, _ := google.New(google.Options{APIKey: "x", BaseURL: url})
				return e
			},
			target: "fr", format: stbox.SubRip, stdout: "own_station.srt -> out.srt\n",
			path: "/language/translate/v2", key: "cli-key", body: []string{`"target":"fr"`, `"format":"html"`},
		},
		{
			name: "openai without a key",
			env: map[string]string{
				"OPENAI_MODEL": "llama3", "OPENAI_BASE_URL": "https://llm.example.com/v1", "DEEPL_API_KEY": "deepl-key",
			},
			words: []string{"own_station.srt", "--engine", "openai", "--source-language", "en", "--target-language", "de"},
			engine: func(url string) translation.Engine {
				e, _ := openai.New(openai.Options{BaseURL: url + "/v1", Model: "x"})
				return e
			},
			source: "en", target: "de", format: stbox.SubRip,
			path: "/v1/chat/completions", key: "", body: []string{`"model":"llama3"`, `You translate subtitle text from en to de.`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := translationtest.NewServer(t)
			base := srv.URL
			if tt.path == "/v1/chat/completions" {
				base += "/v1"
			}
			dir := translateDir(t)
			got := translateCall(t, dir, base, "", tt.env, tt.words...)
			requests := srv.Requests()
			require.Len(t, requests, 1)
			assert.Equal(t, tt.path, requests[0].Path)
			assert.Equal(t, tt.key, requests[0].Key)
			for _, part := range tt.body {
				assert.Contains(t, strings.ReplaceAll(string(requests[0].Body), `": "`, `":"`), part)
			}

			want := translatedByLibrary(t, tt.engine(srv.URL), tt.source, tt.target, tt.format)
			if tt.stdout == "" {
				assert.Equal(t, result{Stdout: want}, got)
				return
			}
			assert.Equal(t, result{Stdout: tt.stdout}, got)
			assert.Equal(t, want, readFile(t, dir, "out.srt"))
		})
	}
}

// Adapted from ApplicationTranslateTest: the service cases.
func TestTranslateServiceFailures(t *testing.T) {
	t.Parallel()

	station, err := os.ReadFile(testfiles.Path(t, "translation/own_station.srt"))
	require.NoError(t, err)
	tests := []struct {
		name     string
		env      map[string]string
		words    []string
		openAI   bool
		stderr   string
		contains []string
		noKey    string
	}{
		{
			name:   "an openai HTTP error",
			env:    map[string]string{"OPENAI_MODEL": "gpt-9", "OPENAI_API_KEY": "bad-request"},
			words:  []string{"-", "--engine", "openai", "--target-language", "de"},
			openAI: true,
			contains: []string{
				"stdin: The OpenAI-compatible service answered with HTTP 400.", "gpt-9",
			},
		},
		{
			name:   "deepl rejects the key",
			words:  []string{"-", "--engine", "deepl", "--api-key", "forbidden", "--target-language", "de"},
			stderr: "stdin: DeepL rejected the API key (HTTP 403). Check the key and its plan.\n",
			noKey:  "forbidden",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := translationtest.NewServer(t)
			base := srv.URL
			if tt.openAI {
				base += "/v1"
			}
			got := translateCall(t, t.TempDir(), base, string(station), tt.env, tt.words...)
			assert.Equal(t, exitFile, got.Code)
			assert.Empty(t, got.Stdout)
			if tt.stderr != "" {
				assert.Equal(t, tt.stderr, got.Stderr)
			}
			for _, part := range tt.contains {
				assert.Contains(t, got.Stderr, part)
			}
			if tt.noKey != "" {
				assert.NotContains(t, got.Stderr, tt.noKey)
			}
			require.Len(t, srv.Requests(), 1)
		})
	}
}

func TestTranslateKeepGoingAndClosedPort(t *testing.T) {
	t.Parallel()

	srv := translationtest.NewServer(t)
	dir := translateDir(t)
	got := translateCall(t, dir, srv.URL, "", map[string]string{"DEEPL_API_KEY": "forbidden"},
		"latin1.srt", "second.srt", "--engine", "deepl", "--target-language", "de", "--encoding", "Windows-1252",
		"--output-dir", "out", "--keep-going")
	failure := "DeepL rejected the API key (HTTP 403). Check the key and its plan.\n"
	assert.Equal(t, result{
		Code: exitFile, Stdout: "2 files: 0 succeeded, 2 failed.\n",
		Stderr: "latin1.srt: " + failure + "second.srt: " + failure,
	}, got)
	assert.Len(t, srv.Requests(), 2)
	assert.NoDirExists(t, filepath.Join(dir, "out"))

	got = translateCall(t, dir, translationtest.ClosedURL(t), "", map[string]string{"DEEPL_API_KEY": "secret-key"},
		"latin1.srt", "--engine", "deepl", "--target-language", "de", "--encoding", "Windows-1252")
	assert.Equal(t, exitFile, got.Code)
	assert.Contains(t, got.Stderr, "latin1.srt: The request to 127.0.0.1 failed: ")
	assert.NotContains(t, got.Stderr, "secret-key")
}

func TestTranslatePrintsWarnings(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"translations": [{"text": "Hallo"}]}`)
	}))
	t.Cleanup(srv.Close)
	got := translateCall(t, t.TempDir(), srv.URL, "1\n00:00:01,000 --> 00:00:02,000\n<i>Hello</i>\n",
		map[string]string{"DEEPL_API_KEY": "k"}, "-", "--engine", "deepl", "--target-language", "de")
	assert.Equal(t, result{
		Stdout: "\ufeff1\n00:00:01,000 --> 00:00:02,000\nHallo\n",
		Stderr: "stdin: cue 1: The engine dropped or changed a placeholder tag. The cue has no tags.\n",
	}, got)
}

// Adapted from ApplicationTranslateTest::usageErrors.
func TestTranslateUsageErrors(t *testing.T) {
	t.Parallel()

	engine := func(name string, more ...string) []string {
		return append([]string{"own_station.srt", "--engine", name, "--target-language", "fr"}, more...)
	}
	const noKey = "Pass --api-key or set the environment variable DEEPL_API_KEY."
	const badKey = "The API key has a control character, or a space at the start or end. Pass the key without them."
	deepl := map[string]string{"DEEPL_API_KEY": "k"}
	tests := []struct {
		name  string
		env   map[string]string
		words []string
		msg   string
	}{
		{"no engine", nil, []string{"own_station.srt", "--target-language", "fr"}, "Pass --engine deepl, google or openai."},
		{"unknown engine", nil, engine("bing"), `The option --engine must be deepl, google or openai, got "bing".`},
		{"deepl without a key", nil, engine("deepl"), noKey},
		{"google without a key", nil, engine("google"), "Pass --api-key or set the environment variable GOOGLE_TRANSLATE_API_KEY."},
		{"the key of another engine", deepl, engine("google"), "Pass --api-key or set the environment variable GOOGLE_TRANSLATE_API_KEY."},
		{"an empty --api-key", deepl, engine("deepl", "--api-key", ""), "The API key is empty. Pass --api-key or set DEEPL_API_KEY."},
		{
			"a key of spaces",
			map[string]string{"DEEPL_API_KEY": " "},
			engine("deepl"),
			"The API key is empty. Pass --api-key or set DEEPL_API_KEY.",
		},
		{"a key with a CR", nil, engine("deepl", "--api-key", "key\r"), badKey},
		{"a key with a space", nil, engine("deepl", "--api-key", " key"), badKey},
		{
			"an empty --api-key for openai",
			map[string]string{"OPENAI_MODEL": "m"},
			engine("openai", "--api-key", ""),
			"The API key is empty. Pass --api-key or set OPENAI_API_KEY.",
		},
		{"no target", deepl, []string{"own_station.srt", "--engine", "deepl"}, "Pass --target-language, for example --target-language fr."},
		{
			"an empty target", deepl,
			[]string{"own_station.srt", "--engine", "deepl", "--target-language", " "},
			"Pass --target-language, for example --target-language fr.",
		},
		{"openai without a model", nil, engine("openai"), "Set the environment variable OPENAI_MODEL, for example OPENAI_MODEL=gpt-4o-mini."},
		{
			"openai with a model of spaces",
			map[string]string{"OPENAI_MODEL": " "},
			engine("openai"),
			"The environment variable OPENAI_MODEL is empty. Set it to a model name, for example gpt-4o-mini.",
		},
		{
			"openai with a bad base URL",
			map[string]string{"OPENAI_MODEL": "m", "OPENAI_BASE_URL": "localhost:11434/v1"},
			engine("openai"),
			`The environment variable OPENAI_BASE_URL must start with http:// or https://, got "localhost:11434/v1".`,
		},
		{
			"2 inputs without --output-dir", deepl,
			[]string{"a.srt", "b.srt", "--engine", "deepl", "--target-language", "fr"},
			"2 input files need --output-dir DIR. One input file goes to standard output or to -o FILE.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := translateCall(t, translateDir(t), translationtest.ClosedURL(t), "", tt.env, tt.words...)
			assert.Equal(t, exitUsage, got.Code)
			assert.Contains(t, got.Stderr, "Error: "+tt.msg+"\n")
			assert.Contains(t, got.Stderr, `Run "stbox help translate" for the usage.`)
			assert.Empty(t, got.Stdout)
		})
	}
}

func TestTranslateAcceptsTheSharedOptions(t *testing.T) {
	t.Parallel()

	srv := translationtest.NewServer(t)
	dir := translateDir(t)
	got := translateCall(t, dir, srv.URL, "", map[string]string{"DEEPL_API_KEY": "k"},
		"own_station.srt", "--engine", "deepl", "--target-language", "de", "--output-dir", "out", "--to", "srt",
		"--output-fps", "25", "--line-ending", "crlf", "--bom", "--skip-image-cues", "--scc-fit", "--from", "srt",
		"--lenient", "--input-fps", "25", "--fps", "25", "--scc-roll-up", "lines", "--keep-going")
	assert.Equal(t, result{Stdout: "own_station.srt -> out/own_station.srt\n"}, got)
	got = translateCall(t, dir, srv.URL, "", map[string]string{"DEEPL_API_KEY": "k"},
		"own_station.srt", "--engine", "deepl", "--target-language", "de", "--no-bom", "-o", "other.srt")
	assert.Equal(t, result{Stdout: "own_station.srt -> other.srt\n"}, got)
	got = translateCall(t, dir, srv.URL, "", map[string]string{"DEEPL_API_KEY": "k"},
		"own_station.srt", "--engine", "deepl", "--target-language", "de", "--track", "1")
	assert.Equal(t, exitFile, got.Code)
}

func TestTranslateHelpHoldsNoKey(t *testing.T) {
	t.Parallel()

	got := call{env: map[string]string{"DEEPL_API_KEY": "secret-env-key"}}.run(t, "translate", "--help")
	assert.Equal(t, 0, got.Code)
	assert.True(t, strings.HasPrefix(got.Stdout, "Usage: stbox translate <input>... --engine deepl|google|openai\n"+
		"                       --target-language CODE [--source-language CODE]\n"+
		"                       [--api-key KEY] [options]\n"))
	assert.NotContains(t, got.Stdout, "secret-env-key")
	assert.NotContains(t, strings.ToLower(got.Stdout), "test url")
	names := make([]string, 0, 7)
	for _, o := range (&translateCommand{}).options()[:7] {
		names = append(names, o.name)
	}
	assert.Equal(t, []string{"engine", "api-key", "source-language", "target-language", "to", "output", "output-dir"}, names)
}
