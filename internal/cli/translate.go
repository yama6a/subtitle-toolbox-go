package cli

import (
	"cmp"
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation/deepl"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation/google"
	"github.com/yama6a/subtitle-toolbox-go/v4/translation/openai"
)

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

// keyVariables maps each engine to the variable of its API key.
//
//nolint:gochecknoglobals // fixed lookup table
var keyVariables = map[string]string{
	"deepl":  "DEEPL_API_KEY",
	"google": "GOOGLE_TRANSLATE_API_KEY",
	"openai": "OPENAI_API_KEY",
}

var baseURLPattern = regexp.MustCompile(`(?i)^https?://[^/]`)

// translateCommand translates the cue text of each input and writes it as convert does.
type translateCommand struct {
	out    writeOutput
	engine translation.Engine
	source string
	target string
	// baseURL points every engine at a test server. Only in-process tests set it, so release binaries have no
	// switch that changes where the key goes.
	baseURL string
}

var _ fileCommand = &translateCommand{}

func (*translateCommand) name() string { return "translate" }

func (*translateCommand) summary() string {
	return "Translate the cue text with DeepL, Google Cloud Translation or an OpenAI-compatible service."
}

func (*translateCommand) usage() []string {
	return []string{"<input>... --engine deepl|google|openai --target-language CODE [--source-language CODE] [--api-key KEY] [options]"}
}

func (*translateCommand) details() string {
	return "The API key comes from --api-key, else from DEEPL_API_KEY for deepl, GOOGLE_TRANSLATE_API_KEY for google " +
		"or OPENAI_API_KEY for openai. openai needs no key for a local service such as Ollama. " +
		"openai reads the model from OPENAI_MODEL and the base URL from OPENAI_BASE_URL, default " + defaultOpenAIBaseURL + ". " +
		"Without -o or --output-dir, the output goes to standard output."
}

func (*translateCommand) options() []option {
	return slices.Concat([]option{
		valueOption("engine", "deepl|google|openai", "Translation service. Required."),
		valueOption("api-key", "KEY", "API key of the engine. Default: the environment variable of the engine."),
		valueOption("source-language", "CODE", "Language of the input, for example de. Default: the engine detects it."),
		valueOption("target-language", "CODE", "Language of the output, for example en-US for DeepL or fr for Google. Required."),
	}, outputOptions("Output format. Default: the input format."), inputOptions(true, writeFPSHelp))
}

func (t *translateCommand) run(ctx context.Context, a *args, c *console) error {
	return runFiles(ctx, a, c, t)
}

// prepare runs every check before any read, so each failure is a usage error.
func (t *translateCommand) prepare(a *args, r *runner) error {
	if err := r.prepareRead(a); err != nil {
		return err
	}
	if err := t.out.prepare(a, r); err != nil {
		return err
	}
	name, ok, err := a.Choice("engine", "deepl", "google", "openai")
	if err != nil {
		return err
	}
	if !ok {
		return usage("Pass --engine deepl, google or openai.")
	}
	key, err := apiKey(a, r.c.getenv, name)
	if err != nil {
		return err
	}
	t.source, _ = a.Value("source-language")
	if t.target, ok = a.Value("target-language"); !ok || strings.Trim(t.target, markupcore.PHPTrimSet) == "" {
		return usage("Pass --target-language, for example --target-language fr.")
	}
	return t.build(name, key, r.c.getenv)
}

// build reads the settings of the engine from the environment and builds it.
func (t *translateCommand) build(name, key string, getenv func(string) string) error {
	var model, base string
	if name == "openai" {
		var err error
		if model, base, err = openAISettings(getenv); err != nil {
			return err
		}
	}
	engine, err := newEngine(name, key, model, cmp.Or(t.baseURL, base))
	if err != nil {
		return usage(message(err))
	}
	t.engine = engine
	return nil
}

// apiKey returns the key of --api-key, else of the variable of the engine, and checks it. openai runs without a key.
func apiKey(a *args, getenv func(string) string, engine string) (string, error) {
	variable := keyVariables[engine]
	key, given := a.Value("api-key")
	if !given {
		key = getenv(variable)
	}
	if key == "" && !given {
		if engine == "openai" {
			return "", nil
		}
		return "", usage("Pass --api-key or set the environment variable " + variable + ".")
	}
	if strings.Trim(key, markupcore.PHPTrimSet) == "" {
		return "", usage("The API key is empty. Pass --api-key or set " + variable + ".")
	}
	if strings.HasPrefix(key, " ") || strings.HasSuffix(key, " ") || strings.ContainsFunc(key, isControl) {
		return "", usage("The API key has a control character, or a space at the start or end. Pass the key without them.")
	}
	return key, nil
}

func isControl(r rune) bool { return r < 0x20 || r == 0x7f }

// openAISettings reads OPENAI_MODEL and OPENAI_BASE_URL. The base URL is "" when the variable is unset.
func openAISettings(getenv func(string) string) (model, base string, err error) {
	model = getenv("OPENAI_MODEL")
	switch {
	case model == "":
		return "", "", usage("Set the environment variable OPENAI_MODEL, for example OPENAI_MODEL=gpt-4o-mini.")
	case strings.Trim(model, markupcore.PHPTrimSet) == "":
		return "", "", usage("The environment variable OPENAI_MODEL is empty. Set it to a model name, for example gpt-4o-mini.")
	}
	base = getenv("OPENAI_BASE_URL")
	if base != "" && !baseURLPattern.MatchString(base) {
		return "", "", usage("The environment variable OPENAI_BASE_URL must start with http:// or https://, got " + quote(base) + ".")
	}
	return model, base, nil
}

// newEngine builds the engine of name from the checked key, model and base URL. An empty base URL keeps the
// default of the engine.
//
//nolint:ireturn // each engine is a different type, and the caller needs only the interface
func newEngine(name, key, model, baseURL string) (translation.Engine, error) {
	switch name {
	case "deepl":
		e, err := deepl.New(deepl.Options{APIKey: key, BaseURL: baseURL})
		if err != nil {
			return nil, err
		}
		return e, nil
	case "google":
		e, err := google.New(google.Options{APIKey: key, BaseURL: baseURL})
		if err != nil {
			return nil, err
		}
		return e, nil
	default:
		e, err := openai.New(openai.Options{BaseURL: cmp.Or(baseURL, defaultOpenAIBaseURL), Model: model, APIKey: key})
		if err != nil {
			return nil, err
		}
		return e, nil
	}
}

func (t *translateCommand) checkInputs(r *runner, inputs []string) error {
	return t.out.checkInputs(r, inputs, inputs, nil)
}

func (*translateCommand) loadSideFiles(context.Context, *runner) error { return nil }

func (t *translateCommand) process(ctx context.Context, r *runner, in *input) error {
	rep, err := translation.Translate(ctx, in.sub, t.engine, t.source, t.target, translation.Options{})
	if err != nil {
		return err
	}
	for _, w := range rep.Warnings {
		r.c.err(in.label() + ": cue " + strconv.Itoa(w.Cue+1) + ": " + w.Message + "\n")
	}
	return t.out.write(r, in, nil)
}

func (*translateCommand) finish(*runner) error { return nil }
