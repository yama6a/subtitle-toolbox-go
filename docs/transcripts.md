# Transcripts

Package `format/transcript` reads and writes the JSON and HTML transcripts of speech-to-text tools and podcasts. This page holds the settings and the rules that every transcript format shares. The format list below names the fields of each format.

## Read options

`ReadOptions.Transcript` holds `transcript.ReadOptions`. The zero value reads no word times and no speakers. All fields are bools, so the struct is comparable and has no `Validate` rule.

| Field | Zero value | Meaning | Read by |
|:--- |:--- |:--- |:--- |
| `WordTimestamps` | false: cues have no `Words` | fill `Cue.Words` from the word times of the source | the formats with word times, and joined Podcasting 2.0 cues |
| `SpeakerVoices` | false: `Cue.Speaker` stays empty | set `Cue.Speaker` from the speaker of the source | the formats with speakers. The Podcasting 2.0 and HTML transcripts always set it |
| `KeepSegments` | false: join one-word segments | one cue per segment | the Podcasting 2.0 transcript only |

`WriteOptions.Transcript` holds `transcript.WriteOptions`. It has no fields yet. A format that needs a write setting adds a nested struct there.

The raw source words stay in the cue format data with both values of `WordTimestamps`. So a writer reuses them after a read without word times.

## Grouping words into cues

A source with timed words and no segments becomes cues by these rules. A new cue starts before a word when one of them holds.

| Rule | Condition | Example |
|---:|:--- |:--- |
| 1 | the speaker differs from the speaker of the word before. A missing speaker differs from every name, also from `""` | `Hi` by A, `there` by B give 2 cues |
| 2 | the word starts 1 s or more after the word before ends | `wait` ends at 0.3 s and `then` starts at 1.3 s give 2 cues. At 1.299 s they give 1 cue |
| 3 | the cue text would pass 84 characters | 41 `a`, 42 `b` and `c` give 2 cues, and `c` starts the second |
| 4 | the word before ends a sentence, and the word does not start with a lower-case letter | `One.` `Two` give 2 cues. `e.g.` `this` gives 1 cue |

- **Length**: the grapheme count of the words plus 1 for each joining space. A letter with combining marks counts 1.
- **Sentence end**: the word ends in `.`, `?`, `!`, U+3002, U+FF0E, U+FF1F or U+FF01, after any closing quotes or brackets.
- **Overlap**: a word that starts before the word before it ends never starts a cue.
- **Cue**: it starts with the first word and ends with the last word. Its text is the words joined with one space. Its speaker is the speaker of the first word.

## Word fill rules

`WordTimestamps` fills `Cue.Words` of each cue from the source words.

1. Each source word with a text and a valid start is searched in the plain text of the cue, from the end of the match before. The search is case-sensitive.
2. A word that the text does not hold gets no entry. Its text stays in the cue.
3. A word that starts before the entry before it gets no entry.
4. The end is the source end, else the start of the next entry, else the cue end. It is never before the start.
5. The confidence is the source value when it is from 0 to 1, else nil. The raw value stays in the format data.

With `SpeakerVoices`, `Cue.Speaker` is the speaker of the source, trimmed, when it is not empty.

## Reuse of source words

A writer rebuilds the word list of a cue from `Cue.Words`. It keeps each source word that the cue still holds, with its ids and fields, and builds the others.

**With words.** The writer splits the cue text at the words of `Cue.Words`. Each part is the text of one word and the untimed text after it. It matches each part, in order, to the first source word that has the same start, rounded to whole milliseconds, and the same text.

- **Same text**: equal, or the source text followed only by punctuation. So the OpenAI word `quiet` matches the part `quiet.`.
- **Run**: source words without a start join the part before them. A part `Pay 15.` matches the source words `Pay` and `15.`.
- **Leading text**: the text before the first word matches a source word without a start.
- **Unmatched part**: the writer builds a new word with the part text and the times of `Cue.Words`.

**Without words.** The writer keeps the source words when the stored times equal the cue times. When the stored text equals the cue text, it writes them unchanged. When the text changed but has as many words, each changed word gets the times of its source word and confidence 1.0. Else the cue gets no words.

**The typed values win.** A reused source word is written with the `End` and `Confidence` of its entry in `Cue.Words`, when they differ from the stored values. Equal values keep the stored text, so an unedited round trip stays the same. A run of 2 or more source words stays unchanged.

**A moved word keeps its confidence.** A new word whose start equals the start of a source word of the cue counts as edited and gets confidence 1.0. Any other new word gets the confidence of its entry in `Cue.Words`, and 1.0 when that is nil. A shifted file keeps its confidences, and a corrected word does not keep the confidence of the wrong word.

## Cost

| Operation | Cost | Benchmark |
|:--- |:--- |:--- |
| group words into cues | linear in the number of words | `TestWordgroupGrowth` |
| fill `Cue.Words` of a cue | one text search per word, plus one more for each distinct word that the text lacks | `TestWordgroupGrowth` |
| reuse source words | linear in the text and the source words, also when many source words share one start or have no start | `TestWordgroupGrowth`, and `BenchmarkReuse` with 1 MB of text in one cue, with distinct starts, one shared start and an untimed leading span |

`TestWordgroupGrowth` runs with `STBOX_GROWTH=1`. It fails when 10 times the words cost more than 30 times the time.

## Formats

| Format | Read | Write | Page section |
|:--- |:--- |:--- |:--- |
| AssemblyAI JSON | `ReadAssemblyAI` | `WriteAssemblyAI` | [AssemblyAI JSON](#assemblyai-json) |

## AssemblyAI JSON

AssemblyAI returns a transcript as JSON with times in milliseconds: a list of `words`, and with speaker labels a list of `utterances`. Example: `{"text": "Where", "start": 260, "end": 560, "confidence": 0.94655, "speaker": "A"}`. The reader makes one cue for each utterance, else it groups the words into cues. The writer writes `text`, `words` and `utterances` back, keeps all other fields, and writes `"utterances": null` when the words hold every cue, as AssemblyAI does.

| Item | Value |
|:--- |:--- |
| Format | `stbox.AssemblyAI`, name `assemblyai`, extension `.json` |
| Functions | `transcript.ReadAssemblyAI` and `WriteAssemblyAI` |
| Read options | `WordTimestamps` and `SpeakerVoices`. `KeepSegments` has no effect |
| Write options | none |
| Detection | none. `.json` belongs to the library JSON, so name the format with `--from assemblyai` or `ReadOptions.Format` |

### Read

- **Lists**: `utterances` counts when it is an array with items. Without utterances, `words` must be an array. Else the read fails with `The JSON has no "words" or "utterances" list.` in both modes. `"utterances": null` falls back to the words. An object with the keys `"0"` and `"1"` is no list ([subtitle-toolbox-php#1208](https://github.com/yama6a/subtitle-toolbox-php/issues/1208)).
- **Utterances**: one cue for each utterance, with the text of `text`. The cue data is the whole utterance.
- **Words only**: the words group into cues by the [grouping rules](#grouping-words-into-cues). The cue data is `{"words": [...]}`.
- **Language**: `language_code` when it is a string that is not empty. The part after the first `_` is upper-cased in ASCII and joins with `-`. So `en_us` gives `en-US`, `en_au` gives `en-AU` and `de` stays `de`.
- **File data**: every top-level member except `text`, `words` and `utterances`.

| Word field | Source |
|:--- |:--- |
| `Text` | `text`, a string. Trimmed |
| `Start` and `End` | `start` and `end` in milliseconds, numbers or numeric strings of 0 or more |
| `Speaker` | `speaker`, a string or an integer |
| `Confidence` | `confidence`, a number from 0 to 1 |

A field error fails the word or the utterance. Strict mode returns the error. Lenient mode skips it with a `Skipped` warning, and a skipped word leaves the rest of its cue.

| Field | Error |
|:--- |:--- |
| a word `start` or `end` that is no time | `The field words[0].start must be a time.`, or `utterances[0].words[0].start` inside an utterance |
| a word `text` that is no string | `The field words[0].text must be a string.` |
| an utterance with a bad `start`, `end` or `text` | `The field utterances[0].end must be a time.` |

### Write

The output is compact JSON without a BOM. The members are the stored file members, then `language_code` when the subtitle has a language and the file has none, then `text`, `words` and `utterances`. The reader removed these 3 members from the file data, so they come last.

- **Language**: the language lower-cased with `-` turned into `_`. So `en-US` gives `en_us`.
- **Utterance**: the stored utterance, or `{"confidence": 1.0}` for a grouped cue, with `end`, `speaker`, `start`, `text` and `words` replaced in this order. `start`, `end` and `text` keep the stored value when it equals the cue. Else they are the cue values, with times as whole milliseconds. `speaker` is the cue speaker, else the stored speaker, else null.
- **Words**: the writer applies the rules of [reuse of source words](#reuse-of-source-words). No result gives `[]`. A new word is `{"text", "start", "end", "confidence", "speaker"}` with `speaker` null when the cue has none.
- **Typed values**: a reused word gets `Word.End` in `end` and `Word.Confidence` in `confidence`, when it has the member and the value differs.
- **Speaker**: a cue speaker that differs from the stored `speaker` is written on every word.
- **Utterances are written** when the source had them, when an utterance has a speaker that is not null, or when a cue has no words. Else the writer writes `"utterances": null`.
- **Empty objects**: an empty JSON object in the file data stays `{}`. PHP writes `[]`.

A subtitle with the language `en-US` and one cue from another format, from 4 to 5 s, with the words `See` (4 to 4.5 s) and `you.` (4.5 to 5 s) writes:

```json
{"language_code":"en_us","text":"See you.","words":[{"text":"See","start":4000,"end":4500,"confidence":1.0,"speaker":null},{"text":"you.","start":4500,"end":5000,"confidence":1.0,"speaker":null}],"utterances":null}
```

### Cost

| Operation | Cost | Benchmark |
|:--- |:--- |:--- |
| read AssemblyAI JSON | linear in the file | `BenchmarkAssemblyAI`: 12,000 words, and 12,000 words in one utterance. Each input is about 1 MB |
| write AssemblyAI JSON | linear in the cues, the words and the stored words | the same inputs |
