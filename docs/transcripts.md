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
| Google Speech-to-Text JSON | `ReadGoogleSpeech` | `WriteGoogleSpeech` | [Google Speech-to-Text JSON](#google-speech-to-text-json) |

## Google Speech-to-Text JSON

Google Cloud Speech-to-Text returns a list of `results`. Each result has alternatives with a transcript and timed words. Times are protobuf durations such as `"1.300s"`. V1 and V2 of the API name the time fields differently, and a long-running operation wraps the response. The reader makes one cue for each result, or, with speaker diarization, groups the words of the last result into cues. The writer writes the field names of the source and keeps the operation around the response.

| Item | Value |
|:--- |:--- |
| Format | `stbox.GoogleSpeech`, name `google-speech`, extension `.json` |
| Functions | `transcript.ReadGoogleSpeech` and `WriteGoogleSpeech` |
| Read options | `WordTimestamps` and `SpeakerVoices`. `KeepSegments` has no effect |
| Write options | none |
| Detection | none. `.json` belongs to the library JSON, so name the format with `--from google-speech` or `ReadOptions.Format` |
| Limit | a diarized result holds its cues only through their words. The writer fails for a cue without words |

### Durations

| Input | Reader | Writer, for a stored value |
|:--- |:--- |:--- |
| a string `-?D(.D)?s`, such as `"1.300s"` | the decimal seconds, exact. A negative value is no time | the same, but a minus sign makes it no duration |
| an object with `seconds` or `nanos`, numbers or numeric strings, a missing one counting 0 | `seconds` plus `nanos` divided by 1e9, in whole milliseconds | the same |
| a plain number | seconds | no duration |
| any other value | no time | no duration |

A plain number is a time for the reader and no duration for the writer, as in PHP. So a result end that is a plain number never counts as the same time, and the writer writes it again as a string. The writer prints a duration from whole milliseconds: `"6s"` when the milliseconds are a multiple of 1000, else `"1.500s"`.

### Read

- **Operation**: a top-level `response` that is an object is unwrapped. The other top-level members, such as `name`, `metadata` and `done`, go first into the file data.
- **Lists**: `results` must be a JSON array. Else the read fails with `The JSON has no "results" list.` in both modes. An object with the keys `"0"` and `"1"` is no list ([subtitle-toolbox-php#1208](https://github.com/yama6a/subtitle-toolbox-php/issues/1208)).
- **Diarized**: the last result is diarized when a word of its first alternative has a `speakerLabel` that is not null, or a `speakerTag` that is not 0, `""`, `"0"`, false or null. Each diarized result repeats the words of the results before it, so the last one holds all words. The words group into cues by the [grouping rules](#grouping-words-into-cues). The file data then holds `results` with the last result without its transcript and words.
- **Other files**: one cue for each result. A result without words starts where the result before it ended, and ends at its result end. A result with words runs from its first word to its last word. The cue data is the result without `alternatives`, then the members of the first alternative.
- **Language**: the first `languageCode` of the results that is a string and not empty.
- **File data**: the operation members, then `results` for a diarized file, then the response members except `results`, such as `totalBilledTime`, `requestId` or `@type`.

| Word field | Source |
|:--- |:--- |
| `Text` | `word`, a string. Trimmed |
| `Start` and `End` | `startTime`, else `startOffset`, and `endTime`, else `endOffset`, as durations |
| `Speaker` | `speakerLabel`, else `speakerTag` when it is not the integer 0 |
| `Confidence` | `confidence`, a number from 0 to 1 |

A field error fails the word or the result. Strict mode returns the error. Lenient mode skips it with a `Skipped` warning. A skipped word leaves the rest of its result. The path of a V2 word time still ends in `.startTime` and `.endTime`.

| Field | Error |
|:--- |:--- |
| a word time that is no duration or no time | `The field results[0].alternatives[0].words[0].startTime must be a time.` |
| a time of 100000 hours or more | `The field results[0].alternatives[0].words[0].startTime is not below 100000 hours.` |
| a result end that is no time, for a result without words | `The field results[0].resultEndTime must be a time.` |
| a first alternative that is no object | `The field results[0].alternatives must be a list of objects.` |
| a transcript that is no string | `The field results[0].alternatives[0].transcript must be a string.` |

### Write

The writer uses V2 names when a stored result or a cue has a `resultEndOffset`, or a first word with a `startOffset`. Else it uses V1 names, also without any stored data. The output is compact JSON without a BOM.

- **Operation**: when the file data has `done` or `name`, the document is the stored `name`, `metadata`, `done` and `error` members in stored order, then `response`. The response leaves out those 4 members.
- **Result for each cue**: `alternatives` with the stored transcript, confidence and words, the transcript set to the cue text, and `words` when the cue has words or the stored data had a `words` member. Then the stored result members, then the result end. A cue without stored data also gets `languageCode` from the subtitle language.
- **Result end**: the stored value when the times are the same and the data has the end key. Else the larger of the end of the result before it and the cue end, because results follow each other. So 2 cues `Long cue` from 1 to 6 s and `Short cue` from 2 to 4 s both write `"resultEndTime":"6s"`, and the file reads back as 2 cues.
- **Text**: the stored transcript when it equals the cue text. A leading space of the stored transcript stays.
- **Words**: the writer applies the rules of [reuse of source words](#reuse-of-source-words). No result gives `[]`. A new word is `{"startTime", "endTime", "word"}`, with `startOffset` and `endOffset` in V2, and `confidence` when a stored word of the cue has one.
- **Typed values**: a reused word gets `Word.End` in its end member and `Word.Confidence` in `confidence`, when it has the member and the value differs.
- **Diarized source**: one result with all words of all cues. Each word gets `speakerLabel` with the cue speaker in V2, or `speakerTag` in V1. A V1 speaker of the digits 0 to 9 only is an integer, unless it is too large for one ([subtitle-toolbox-php#1274](https://github.com/yama6a/subtitle-toolbox-php/issues/1274)). A cue without words fails with an `UnwritableError` that names the cue, such as `Cue #2 at 8.24 s has no words.`

2 cues from another format with the language `en-US`, `<v Anna>Good` / `morning.` from 1.5 to 3 s, and `See you.` from 4 to 5 s with 2 words, write:

```json
{"results":[{"alternatives":[{"transcript":"Good morning."}],"resultEndTime":"3s","languageCode":"en-US"},{"alternatives":[{"transcript":"See you.","words":[{"startTime":"4s","endTime":"4.500s","word":"See"},{"startTime":"4.500s","endTime":"5s","word":"you."}]}],"resultEndTime":"5s","languageCode":"en-US"}]}
```

### Cost

| Operation | Cost | Benchmark |
|:--- |:--- |:--- |
| read Google Speech-to-Text JSON | linear in the file. A duration string is read once with the bounded decimal parser, so `"1e1000000000s"` is rejected without work | `BenchmarkGoogleSpeech`: 12,000 words in results of 10 words, and 12,000 words in one diarized result. Each input is about 1 MB |
| write Google Speech-to-Text JSON | linear in the cues, the words and the stored words | the same inputs |
