package translation

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yama6a/subtitle-toolbox-go/v4/internal/grapheme"
	"github.com/yama6a/subtitle-toolbox-go/v4/internal/markupcore"
	"github.com/yama6a/subtitle-toolbox-go/v4/model"
)

const sentenceEnds = ".?!\u2026\u3002\uff0e\uff1f\uff01"

const sentenceEndClosers = "\"'\u201d\u2019\u00bb)]"

// mongolianVowelSeparator is U+180E. Unicode 6.3 removed it from White_Space, and PCRE2 10.44 still counts it as \s.
const mongolianVowelSeparator = '\u180e'

// request is the text of one cue or of a sentence over several cues.
type request struct {
	cues []int
	text string
	tags []tagPair
}

// Translate translates the cue text of s from source to target with engine and sets the language metadata to target.
// s changes only when every engine call succeeds.
func Translate(ctx context.Context, s *model.Subtitle, engine Engine, source, target string, opts Options) (Report, error) {
	if err := check(engine, target, opts); err != nil {
		return Report{}, err
	}

	cues := s.Cues()
	requests := buildRequests(cues, opts)
	var warnings []Warning
	for _, batch := range batches(requests, cmp.Or(opts.MaxCharactersPerRequest, DefaultMaxCharactersPerRequest)) {
		got, err := callEngine(ctx, engine, batch, source, target)
		if err != nil {
			return Report{}, err
		}
		for i, r := range batch {
			warnings = append(warnings, fill(cues, r, got[i])...)
		}
	}

	for _, r := range requests {
		for _, i := range r.cues {
			if err := s.SetCue(i, cues[i]); err != nil {
				return Report{}, err
			}
		}
	}
	s.SetMetadata(model.MetadataLanguage, target)
	slices.SortStableFunc(warnings, func(a, b Warning) int { return cmp.Compare(a.Cue, b.Cue) })
	return Report{Warnings: warnings}, nil
}

func check(engine Engine, target string, opts Options) error {
	if err := opts.Validate(); err != nil {
		return err
	}
	if engine == nil {
		return &model.InvalidArgumentError{Msg: "The translation engine is missing."}
	}
	if strings.Trim(target, markupcore.PHPTrimSet) == "" {
		return &model.InvalidArgumentError{Msg: "The target language is missing."}
	}
	return nil
}

// callEngine sends the texts of batch and returns one translation per request. A context that ended wins over the
// error of the engine, so a caller can tell an interrupt from a service failure.
func callEngine(ctx context.Context, engine Engine, batch []request, source, target string) ([]string, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("translation: %w", ctx.Err())
	}
	texts := make([]string, len(batch))
	for i, r := range batch {
		texts[i] = r.text
	}
	got, err := engine.Translate(ctx, texts, source, target)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("translation: %w", ctx.Err())
		}
		return nil, err
	}
	if len(got) != len(texts) {
		return nil, &ServiceError{Msg: fmt.Sprintf("The translation engine must return one string per text, got %d values for %d texts.", len(got), len(texts))}
	}
	return got, nil
}

func visibleText(c model.Cue) string {
	return markupcore.VisibleText(strings.Join(c.Lines, " "))
}

func needsTranslation(c model.Cue) bool {
	text := visibleText(c)
	if text == "" {
		return false
	}
	if !utf8.ValidString(text) {
		return true
	}
	for _, r := range text {
		if !unicode.IsNumber(r) && !unicode.IsPunct(r) && !unicode.IsSymbol(r) && !unicode.IsSpace(r) && r != mongolianVowelSeparator {
			return true
		}
	}
	return false
}

func endsSentence(c model.Cue) bool {
	text := visibleText(c)
	if !utf8.ValidString(text) {
		return true
	}
	text = strings.TrimRight(text, sentenceEndClosers)
	r, _ := utf8.DecodeLastRuneInString(text)
	return r != utf8.RuneError && strings.ContainsRune(sentenceEnds, r)
}

func buildRequests(cues []model.Cue, opts Options) []request {
	maxCues := cmp.Or(opts.MaxCuesPerSentence, DefaultMaxCuesPerSentence)
	if opts.NoJoinSentences {
		maxCues = 1
	}
	var requests []request
	for i := 0; i < len(cues); i++ {
		if !needsTranslation(cues[i]) {
			continue
		}
		group := []int{i}
		for len(group) < maxCues && !endsSentence(cues[i]) && i+1 < len(cues) && needsTranslation(cues[i+1]) {
			i++
			group = append(group, i)
		}
		text, tags := encode(requestText(cues, group))
		requests = append(requests, request{cues: group, text: text, tags: tags})
	}
	return requests
}

func requestText(cues []model.Cue, group []int) string {
	if len(group) == 1 {
		return cues[group[0]].Text()
	}
	parts := make([]string, len(group))
	for k, i := range group {
		parts[k] = strings.Join(cues[i].Lines, " ")
	}
	return strings.Join(parts, " ")
}

// batches fills each batch with whole requests up to maxCharacters. A longer request goes in a batch of its own.
func batches(requests []request, maxCharacters int) [][]request {
	var out [][]request
	start, characters := 0, 0
	for i, r := range requests {
		n := grapheme.Count(r.text)
		if i > start && characters+n > maxCharacters {
			out = append(out, requests[start:i])
			start, characters = i, 0
		}
		characters += n
	}
	if start < len(requests) {
		out = append(out, requests[start:])
	}
	return out
}

// fill writes the translation of r into the cues of r and returns the warnings.
func fill(cues []model.Cue, r request, translation string) []Warning {
	var warnings []Warning
	for _, i := range r.cues {
		cues[i].Words = nil
	}
	tags := r.tags
	var tokens []token
	if keepsPlaceholders(translation, tags) {
		tokens = tokenize(translation)
	} else {
		tokens = plainTokens(removePlaceholders(translation))
		tags = nil
		for _, i := range r.cues {
			warnings = append(warnings, Warning{Cue: i, Message: "The engine dropped or changed a placeholder tag. The cue has no tags."})
		}
	}
	if len(r.cues) == 1 {
		cues[r.cues[0]].SetText(restore(tokens, tags))
		return warnings
	}

	weights := make([]int, len(r.cues))
	for k, i := range r.cues {
		weights[k] = max(1, grapheme.Count(visibleText(cues[i])))
	}
	for k, piece := range split(tokens, weights) {
		i := r.cues[k]
		cues[i].Lines = nil
		cues[i].AddLine(strings.ReplaceAll(restore(piece, tags), "\n", " "))
		if len(cues[i].Lines) == 0 {
			warnings = append(warnings, Warning{Cue: i, Message: "The translation of the sentence is too short to fill this cue. The cue has no text."})
		}
	}
	return warnings
}
