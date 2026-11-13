# Translation

`translation.Translate` translates the cue text of a subtitle with a machine translation engine. It keeps the times and the tags. It translates a sentence as a whole, also when the sentence runs over 2 cues. Example: `testdata/translation/own_station.srt` has the cues `The train to Basel leaves` and `from platform 4 at <b>10:15</b>.`. The engine gets one text, `The train to Basel leaves from platform 4 at <x1>10:15</x1>.`. The runner cuts the translation back into 2 cues in proportion to their characters, at a space, and puts `<b>` back in place of `<x1>`.

<!-- example: translation/ExampleTranslate -->
```go
s := &model.Subtitle{}
for i, text := range []string{"The train to Basel leaves", "from platform 4 at <b>10:15</b>."} {
	c := model.Cue{Start: time.Duration(i+1) * time.Second, End: time.Duration(i+2) * time.Second}
	c.SetText(text)
	s.AddCue(c)
}

report, err := translation.Translate(context.Background(), s, stationEngine{}, "en", "de", translation.Options{})
if err != nil {
	fmt.Println(err)
	return
}

for _, c := range s.Cues() {
	fmt.Println(c.Lines)
}
language, _ := s.Metadata(model.MetadataLanguage)
fmt.Println(language, len(report.Warnings))
// Output:
// [Der Zug nach Basel faehrt]
// [um <b>10:15</b> von Gleis 4 ab.]
// de 0
```

## The engine

A `translation.Engine` has one method. `Translate(ctx, texts, source, target)` returns one translation for each text, in the same order. The texts hold placeholders and entities, which the translations must keep, see [Placeholders](#placeholders). An empty `source` lets the service detect the language. A caller can plug in any translation service by implementing the interface, see [Write an engine](#write-an-engine).

The engines of the library live in `translation/<engine>`. Package `translation` itself has no engine.

## Translate

`Translate(ctx, s, engine, source, target, opts)` does these steps.

1. It checks `opts`, the engine and the target. A nil engine, and a target that is empty after the PHP trim, return an `*model.InvalidArgumentError`.
2. It copies the cues with `s.Cues()`.
3. It picks the cues that need translation, and joins them into sentences, see [What the runner sends](#what-the-runner-sends).
4. It replaces the tags of each text with placeholders.
5. It cuts the texts into batches and calls the engine once for each batch, see [Batches](#batches).
6. It fills the cues from the translations.
7. It stores the cues in `s` and sets the `language` metadata to `target`, also for a subtitle without cues. The times, the order and the comments stay.

`s` changes only when every engine call succeeds. The language that the engine gets is `target` as the caller wrote it. The metadata gets the same string.

`Translate` returns a `Report` with the warnings, see [Warnings](#warnings).

## Options

`translation.Options` is comparable. The zero value joins sentences of up to 3 cues and sends up to 5000 characters in one engine call.

| Field | Default (zero value) | Meaning |
|:--- |:--- |:--- |
| `NoJoinSentences` | joining on | send each cue as its own text |
| `MaxCuesPerSentence` | 0 means 3 | the most cues in one text |
| `MaxCharactersPerRequest` | 0 means 5000 | the most grapheme clusters of the encoded texts in one engine call. A longer text goes alone |

`Validate` returns an error that matches `model.ErrInvalidArgument` for a limit below 0, for example `The maximum cues per sentence must be at least 1, got -1.`.

## What the runner sends

The visible text of a cue is its lines joined with a space, with the tags removed, the entities decoded and the white space trimmed.

- **Needs translation**: a cue needs translation when its visible text holds a character that is no number, no punctuation, no symbol and no white space. A cue with text that is no valid UTF-8 needs translation. Cues such as `1984`, `...`, a music note and an image cue without lines are never sent. A cue that is not sent keeps its lines.
- **Ends a sentence**: a cue ends a sentence when its visible text ends with `.`, `?`, `!`, U+2026, U+3002, U+FF0E, U+FF1F or U+FF01. Any of `"`, `'`, U+201D, U+2019, U+00BB, `)` and `]` may follow. Visible text that is no valid UTF-8 ends a sentence, so the runner never cuts it.
- **Groups**: a group starts at each cue that needs translation. With joining on, the group takes the next cue while it has fewer cues than `MaxCuesPerSentence`, the last cue does not end a sentence, and the next cue needs translation.
- **Request text**: a group of 1 cue sends its lines joined with LF. A larger group sends the lines of each cue joined with a space, and the cues joined with a space.

Example: the cues `one`, `two`, `three`, `four`, `five.` send `one two three` and `four five.`. With `MaxCuesPerSentence` 2 they send `one two`, `three four` and `five.`.

## Batches

The runner fills a batch with whole texts while the sum of their grapheme counts stays at most `MaxCharactersPerRequest`. Placeholders and entities count with all their characters. A text that is longer alone goes in a batch of its own. One engine call serves one batch. A subtitle without text to translate makes no call.

Example: with a limit of 32, the texts `First sentence.`, `Second sentence.`, `Third one is split over cues.` and a text of 59 characters make 3 calls.

## Placeholders

A placeholder is a numbered stand-in for a tag. The texts are then valid XML content, which the services read with their tag handling.

- **Tags**: the tags are the matches of `</?[a-zA-Z][^<>]*>`.
- **Pairs**: the name of a tag is lower case. A tag that ends in `/>` is single. A closing tag pairs with the last open tag of its name. A pair becomes `<xN>` and `</xN>`. Every other tag becomes `<xN/>`, such as a `<v Name>` without `</v>` and a stray closing tag. N counts from 1 in text order of the opening or single tag.
- **Text**: the text between tags has its entities decoded once and then escaped with `&lt;`, `&gt;` and `&amp;` only.

| Text | Encoded | Restored after an unchanged answer |
|:--- |:--- |:--- |
| `<i>Run, <b>now</b>!</i>` | `<x1>Run, <x2>now</x2>!</x1>` | the same text |
| `<v Fred>Hi <br/>there.` | `<x1/>Hi <x2/>there.` | the same text |
| `<b>Tom &amp; Jerry</b> sagen a &lt; b` | `<x1>Tom &amp; Jerry</x1> sagen a &lt; b` | the same text |

**Kept placeholders**: a translation keeps the placeholders when 3 rules hold. The placeholders of the form `</?x\d+/?>` are the same multiset as the expected ones. The count of the loose form `<\s*/?\s*x\s*\d+\s*/?\s*>` equals the count of the exact form. For each pair, the first `<xN>` comes before the first `</xN>`. The table shows the translations of `<x1>Lauf!</x1>`.

| Translation | Kept |
|:--- |:--- |
| `<x1>Run!</x1>` | yes |
| `<x1>Run! </ x1>` | no, a loose placeholder |
| `Run!</x1><x1>` | no, swapped |
| `<x1>Run!</x1><x2/>` | no, an extra placeholder |
| `<x1>Run!</x1></x1>` | no, a doubled placeholder |

When a translation does not keep the placeholders, the runner removes every loose placeholder, uses no tags for the cues of that text, and adds a warning for each cue. The rest is plain text, so the answer `<x<x1/>1>Hallo` gives the line `&lt;x1&gt;Hallo`.

**Restore**: the runner decodes the entities of the answer once, escapes the text again, and writes the tags back. A pair with only white space between its placeholders disappears, and the white space stays. So `<x1> </x1>` becomes a space. A bare `&`, `<` or `>` in the answer becomes an entity.

Example: the answer `<x1>Tom & Jerry</x1> &#39;say&#39; a < b &amp; c&nbsp;d` for the cue `<b>Tom &amp; Jerry</b> sagen a &lt; b` gives the line `<b>Tom &amp; Jerry</b> 'say' a &lt; b &amp; c` + U+00A0 + `d`.

## The sentence splitter

A group of more than 1 cue gets one translation. The splitter cuts it into one piece per cue.

- **Weights**: the weight of a cue is the grapheme count of its visible text, at least 1.
- **Cut candidates**: a candidate is a character offset above 0 where the character is white space and not the last character. A candidate is also an offset where the character and the one before it both belong to the scripts Han, Hiragana, Katakana, Thai, Lao, Khmer or Myanmar. The runner reads these scripts as PCRE2 does, with their Script_Extensions, so U+3002 and U+30FC count.
- **Cuts**: for each cut, the runner takes the candidate nearest to the share of the characters that the weights give. With fewer candidates than cuts, the last pieces stay empty.
- **Pieces**: a white space character at the start of a piece is dropped. A tag that is open at the end of a piece closes there and opens again at the start of the next piece.

Example: the cues `<i>The train` and `leaves now.</i>` get the translation `<x1>` + 7 Chinese characters ending in U+3002 + `</x1>`. The first cue gets `<i>` + the first 3 characters + `</i>`, and the second cue gets `<i>` + the last 4 characters + `</i>`.

Example: the cues `Yes,` and `of course.` get the translation `Ja`. The first cue gets `Ja`. The second cue stays empty and gets a warning.

Each piece reopens the tags that are open at its start. The output is at most `MaxCuesPerSentence` times the encoded text, and no other bound applies.

## Warnings

`Report.Warnings` lists each `Warning` with the 0-based cue index and a message, sorted by cue index. It is nil when there is no warning. `stbox translate` prints the index plus 1.

| Message | When |
|:--- |:--- |
| `The engine dropped or changed a placeholder tag. The cue has no tags.` | the translation of the text does not keep the placeholders. One warning for each cue of the text |
| `The translation of the sentence is too short to fill this cue. The cue has no text.` | a piece of a sentence over several cues is empty. The cue keeps no line |

## Errors

| Case | Error |
|:--- |:--- |
| a limit below 0 | `*model.InvalidArgumentError` with `Option` `translation.Options.MaxCuesPerSentence` or `translation.Options.MaxCharactersPerRequest` |
| nil engine, empty target | `*model.InvalidArgumentError` with an empty `Option`: `The translation engine is missing.` and `The target language is missing.` |
| the engine returns a wrong number of strings | `*translation.ServiceError` with `Status` 0: `The translation engine must return one string per text, got 1 values for 2 texts.` |
| the engine returns an error | that error, unchanged |
| `ctx` ended | an error that wraps `ctx.Err()`, with the prefix `translation: `. It is no `*ServiceError` |
| no response from the service | `*ServiceError` with `Status` 0 and the `*url.Error` as cause: `The request to 127.0.0.1 failed: REASON.` |
| the request URL is not valid | `*ServiceError` with `Status` 0: `The request URL is not valid: REASON.` The URL stays out of the message, because it can hold a password |
| the answer is too large | `*ServiceError` with the status: `The answer from HOST has more than N bytes.` |

Every `*ServiceError` matches `translation.ErrService` with `errors.Is`. A `ctx` error is no `*ServiceError`, so a caller can tell an interrupt from a service failure. `Report` is empty on every error, and `s` does not change.

## Words and speakers

The runner sets `Words` to nil on each cue that it translated. A source-language word time has no meaning for a target-language word, so karaoke output and resegmenting by words do not work on a translation. A cue that the runner does not send keeps its words. `Speaker` is a name. The runner does not send it, and it stays. The tags of a cue with 2 voices are in the lines, so they become placeholders.

Example: a cue with `Speaker` `Fred`, the line `Hi there.` and 2 words sends `Hi there.`. The result keeps `Fred`, has the line `HI THERE.` and no words.

## HTTP

The engines send JSON requests through `translation.HTTPClient`, an interface with the method `Do`. `*http.Client` implements it. A test points an engine at an `httptest.Server`.

- **Default client**: `translation.DefaultHTTPClient()` returns a new client with a connect timeout of 10 s and a total timeout of 120 s. It uses the proxy of the environment. It follows no redirect, so a 3xx answer reaches the engine as it is and no key goes to another host.
- **Retries**: after HTTP 429, 500, 502, 503 and 504 the request goes out again, up to 3 times, so up to 4 requests. The fourth answer returns as it is, and the engine turns its status into an error. No other status is retried. A request without a response is not retried.
- **Waits**: before retry 1, 2 and 3 the engine waits for the `Retry-After` header when it is valid, else 1, 2 and 4 s. `Retry-After` holds digits for seconds, or an HTTP date. A date counts from the `Date` header of the same answer, else from the clock. The wait is at most 60 s, so one batch can wait up to 3 minutes. Any other value gives the fixed wait.
- **Size bound**: an answer may hold 1 MiB plus 16 bytes for each byte of the request, because it holds the texts of the request. A larger answer returns a `*ServiceError` and is not retried.
- **Key**: a message of an engine never holds its key.

## Write an engine

This engine posts the texts as JSON and reads `{"translations": [...]}`. It takes the client from its field, and uses `DefaultHTTPClient` when the field is nil.

```text
type webEngine struct {
	url    string
	client translation.HTTPClient
}

func (e webEngine) Translate(ctx context.Context, texts []string, source, target string) ([]string, error) {
	body, err := json.Marshal(map[string]any{"texts": texts, "source": source, "target": target})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := e.client
	if client == nil {
		client = translation.DefaultHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &translation.ServiceError{Status: resp.StatusCode, Msg: "The service answered " + resp.Status + "."}
	}
	var answer struct{ Translations []string }
	if err := json.NewDecoder(resp.Body).Decode(&answer); err != nil {
		return nil, err
	}
	return answer.Translations, nil
}
```

The service must return the placeholders of each text, such as `<x1>` and `<x2/>`, and the entities `&lt;`, `&gt;` and `&amp;`. A service with an XML tag handling does this. A service that rewrites a placeholder makes the runner drop the tags of that text and add a warning.

## Google Cloud Translation

The package `translation/google` is a client of Cloud Translation Basic (v2) with an API key. It uses `net/http` only.

<!-- example: translation/google/ExampleNew -->
```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, `{"data": {"translations": [{"translatedText": "Le train part &agrave; <x1>10:15</x1>."}]}}`)
}))
defer server.Close()
engine, err := google.New(google.Options{APIKey: "my-key", BaseURL: server.URL})
if err != nil {
	fmt.Println(err)
	return
}

s := &model.Subtitle{}
c := model.Cue{Start: time.Second, End: 2 * time.Second}
c.SetText("The train leaves at <b>10:15</b>.")
s.AddCue(c)
if _, err := translation.Translate(context.Background(), s, engine, "en", "fr", translation.Options{}); err != nil {
	fmt.Println(err)
	return
}

fmt.Println(s.Cues()[0].Lines)
// Output: [Le train part à <b>10:15</b>.]
```

| Option | Default | Meaning |
|:--- |:--- |:--- |
| `APIKey` | required | an API key of a Google Cloud project with the Cloud Translation API |
| `BaseURL` | `https://translation.googleapis.com` | the scheme and host of the service, for example a proxy |
| `HTTPClient` | `translation.DefaultHTTPClient()` | the client that sends the requests |

`New` rejects an empty key, a key with a control character or a space at the start or end, and a base URL that does not start with `http://` or `https://`. Each error matches `model.ErrInvalidArgument`. `fmt` prints the key as `***`.

- **Request**: `POST {base}/language/translate/v2` with the header `X-goog-api-key` and a JSON body `{"q": [...], "target": "fr", "format": "html", "source": "en"}`. The key never goes into the URL. An empty source leaves `source` out, so Google detects the language.
- **Chunks**: the engine sends at most `google.MaxTextsPerRequest` (128) texts per request. 300 texts give requests of 128, 128 and 44 texts.
- **Answer**: the engine returns each `translatedText` as received. The runner decodes entities such as `&#39;`. A wrong number of translations, or a translation that is not a string, gives a `*translation.ServiceError`.
- **Errors**: every `*translation.ServiceError` has the HTTP status and a message without the key.

| Status | Message |
|:--- |:--- |
| 403 | `Google Cloud Translation refused the request (HTTP 403). Check the key and that the API is enabled.` and the `error.message` of the answer |
| 429, after 4 attempts | `Google Cloud Translation got too many requests (HTTP 429). Wait and try again.` |
| other | `Google Cloud Translation answered with HTTP N.` and the `error.message` of the answer |

## Differences from PHP

- An engine that returns a wrong number of strings gives a `*ServiceError`. PHP throws `InvalidArgumentException`.
- The engines read `Retry-After`. PHP always waits 1, 2 and 4 s.
- An empty target language fails before any request. PHP sends it to the service.
- An answer larger than 1 MiB plus 16 bytes for each request byte fails. PHP reads any size.
- An answer such as `<x<x1/>1>Hallo` gives plain text and a warning. PHP throws a `TypeError`.
- The runner sets `Words` to nil on a translated cue. PHP puts the source word timestamps back in the text.
- Go needs no ext-curl, because it uses `net/http`.
- A character of a script without spaces that is new after Unicode 15.0, such as U+2EBF0, is a cut candidate in Go only.

See [php-differences.md](php-differences.md).
