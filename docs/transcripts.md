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
| Amazon Transcribe JSON | `ReadAWSTranscribe` | `WriteAWSTranscribe` | [Amazon Transcribe JSON](#amazon-transcribe-json) |

## Amazon Transcribe JSON

Amazon Transcribe returns a batch transcript as JSON with one item for each word and each punctuation mark. Times and confidences are strings. Example: `{"type": "pronunciation", "start_time": "0.52", "end_time": "0.94", "speaker_label": "spk_0", "alternatives": [{"confidence": "0.91", "content": "The"}]}` and then `{"type": "punctuation", "alternatives": [{"confidence": "0.0", "content": "."}]}`. Newer transcripts add `audio_segments`, one for each spoken phrase. The reader makes one cue for each audio segment, else it groups the words into cues. The writer writes the transcript back with renumbered item ids and new speaker segments.

| Item | Value |
|:--- |:--- |
| Format | `stbox.AWSTranscribe`, name `aws-transcribe`, extension `.json` |
| Functions | `transcript.ReadAWSTranscribe` and `WriteAWSTranscribe` |
| Read options | `WordTimestamps` and `SpeakerVoices`. `KeepSegments` has no effect |
| Write options | none |
| Detection | none. `.json` belongs to the library JSON, so name the format with `--from aws-transcribe` or `ReadOptions.Format` |
| Not read | `results.channel_labels`. The writer does not write it |

### Read

- **Lists**: `results` must be an object and `results.items` a JSON array. Else the read fails with `The JSON has no "results.items" list.` in both modes. `audio_segments` counts when it is an array with items. An object with the keys `"0"` and `"1"` is no list ([subtitle-toolbox-php#1208](https://github.com/yama6a/subtitle-toolbox-php/issues/1208)).
- **Words**: a pronunciation item makes a word. A punctuation item joins the word before it without a space. Punctuation before the first word waits and joins the first word in front. So the item U+00BF, the word `Vienes` and the item `?` give the word U+00BF followed by `Vienes?`.
- **Cues with audio segments**: one cue for each segment. Its words are the words of the item ids in the segment `items` list. The integer 5 and the string `"5"` name the same item, and `"05"` is another id. An id that names no word is ignored. A word that the list names twice in a row counts once.
- **Cues without audio segments**: the words group into cues by the [grouping rules](#grouping-words-into-cues).
- **Language**: `results.language_code`.
- **Speaker**: with `SpeakerVoices`, the `speaker_label` of the segment, or of the first item of a grouped cue. A string or an integer.
- **File data**: the top-level members except `results`, then `results` without `transcripts`, `items`, `audio_segments` and `channel_labels`. `results.speaker_labels` loses its `segments`.
- **Cue data**: for a segment, the segment object with `items` replaced by the raw items of its words. Else `{"items": [...]}` with the raw items of the group.

A field error fails the item or the segment. Strict mode returns the error. Lenient mode skips it with a `Skipped` warning without a line, with the item as block text.

| Field | Error |
|:--- |:--- |
| an item without a string `alternatives[0].content`, also an item that is no object | `The field results.items[0].alternatives[0].content must be a string.` |
| a pronunciation item with a `start_time` or `end_time` that is no number or numeric string of 0 or more | `The field results.items[0].start_time must be a time.` |
| a time of 100000 hours or more | `The field results.items[0].start_time is not below 100000 hours.` |
| a segment with a bad `start_time`, `end_time` or `transcript` | `The field results.audio_segments[0].transcript must be a string.` |

**Words.** Each word gives one entry of `Cue.Words`. `Text` is the content of its items joined. `Start` and `End` come from the pronunciation item. `Confidence` is `alternatives[0].confidence` of the pronunciation item, when it is a number or numeric string from 0 to 1.

### Write

The output is compact JSON without a BOM, in this order.

| Member | Content |
|:--- |:--- |
| `results.transcripts` | one entry with the segment texts joined with one space |
| `results.speaker_labels` | only when an item has a start, an end and a speaker label. `speakers` is the number of distinct labels. `segments` are the runs of items with the same label, each with its `items` |
| `results.items` | one item for each word and punctuation mark, with the ids 0, 1 and so on |
| `results.audio_segments` | one for each cue, written when a cue has a stored `transcript`, or when a cue has no items. The items alone cannot hold a cue without words |
| `results` | then the stored members, and `language_code` from the subtitle language when the file has none |
| top level | then the stored members such as `jobName` |

- **Words**: the writer applies the rules of [reuse of source words](#reuse-of-source-words). A source word is a pronunciation item with the punctuation items after it. No result gives no items for the cue.
- **New word**: a pronunciation item with the text before the trailing punctuation, and a punctuation item with the confidence `"0.0"` for the punctuation. The confidence follows the rule for new words in reuse of source words. `speaker_label` is set only when the cue has a speaker.
- **Typed values**: a reused word gets `Word.End` in the `end_time` of its pronunciation item and `Word.Confidence` in `alternatives[0].confidence`, when they differ from the stored values.
- **Speaker**: a cue with a speaker gives each item that speaker as `speaker_label`, when it differs from the stored segment label or the item has none.
- **Ids**: each item gets the next id as its first member. The segment `items` list holds the new ids. Each segment gets its index as `id`.
- **Numbers**: a time is a string with at most 3 decimals and at least 1, such as `"2.0"` and `"0.52"`. A confidence is the shortest decimal with at least 1 decimal, such as `"1.0"` and `"0.9989"`.

A cue from another format, `<v Ben>` with the words `See` (4 to 4.5 s) and `you.` (4.5 to 5 s), writes the items:

```json
{"id":0,"start_time":"4.0","speaker_label":"Ben","end_time":"4.5","alternatives":[{"confidence":"1.0","content":"See"}],"type":"pronunciation"},
{"id":1,"start_time":"4.5","speaker_label":"Ben","end_time":"5.0","alternatives":[{"confidence":"1.0","content":"you"}],"type":"pronunciation"},
{"id":2,"speaker_label":"Ben","alternatives":[{"confidence":"0.0","content":"."}],"type":"punctuation"}
```

### Cost

| Operation | Cost | Benchmark |
|:--- |:--- |:--- |
| read Amazon Transcribe JSON | linear in the file. A segment looks up each item id once. A word with many punctuation items joins their texts once | `BenchmarkAWSTranscribe`: 10,000 items, 10,000 items in one segment, 20,000 punctuation items on one word. Each input is about 1 MB |
| write Amazon Transcribe JSON | linear in the cues, the items and the stored items | the same inputs |
