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
| Deepgram JSON | `ReadDeepgram` | `WriteDeepgram` | [Deepgram JSON](#deepgram-json) |

## Deepgram JSON

Deepgram returns the transcript of a recording as JSON. Each audio channel has a list of words. On request the response also has utterances, paragraphs with sentences, speakers and a detected language. Example: `{"word": "der", "start": 0.4, "end": 0.82, "confidence": 0.9087655, "punctuated_word": "Der"}`. The reader makes one cue for each utterance, else one for each paragraph sentence, else it groups the words into cues. The writer writes the response back for each channel, with words, paragraphs and utterances in the shape of the source.

| Item | Value |
|:--- |:--- |
| Format | `stbox.Deepgram`, name `deepgram`, extension `.json` |
| Functions | `transcript.ReadDeepgram` and `WriteDeepgram` |
| Read options | `WordTimestamps` and `SpeakerVoices`. `KeepSegments` has no effect |
| Write options | none |
| Detection | none. `.json` belongs to the library JSON, so name the format with `--from deepgram` or `ReadOptions.Format` |
| Known loss | the paragraphs of a file with utterances. The reader reads only the utterances, so the writer cannot rebuild the paragraphs |

### Read

- **Lists**: `results` must be an object and `results.channels` a JSON array. Else the read fails with `The JSON has no "results.channels" list.` in both modes. An object with the keys `"0"` and `"1"` is no list ([subtitle-toolbox-php#1208](https://github.com/yama6a/subtitle-toolbox-php/issues/1208)).
- **Utterances**: when `results.utterances` is an array with items, each utterance gives one cue. The channels are not read. The cue data is the whole utterance.
- **Paragraphs**: a channel whose first alternative has `paragraphs.paragraphs` gives one cue for each sentence. The speaker is the `speaker` of the paragraph. The words of a cue are the words whose middle lies between the start and the end of the sentence. The cue data is the sentence members, then `channel`, `speaker`, `paragraph` without `sentences`, and `words`.
- **Words only**: the words of a channel group into cues by the [grouping rules](#grouping-words-into-cues). The cue data is `{"channel": N, "words": [...]}`.
- **Channels**: only a channel whose `alternatives[0]` is an object gives cues. The cues of all channels are sorted by start, so channel 1 can come before channel 0.
- **Language**: `results.channels[0].detected_language`.
- **File data**: the members outside `results`, then `results` with the other members except `utterances`. Each channel keeps its members and the members of its first alternative except `transcript`, `words` and `paragraphs`. So `detected_language`, `language_confidence` and the alternative `confidence` stay.

| Word field | Source |
|:--- |:--- |
| `Text` | `punctuated_word` when it is not null, else `word`. It must be a string. Trimmed |
| `Start` and `End` | `start` and `end`, numbers or numeric strings of 0 or more |
| `Speaker` | `speaker`, a string or an integer |
| `Confidence` | `confidence`, a number from 0 to 1 |

A field error fails the word, the utterance or the sentence. Strict mode returns the error. Lenient mode skips it with a `Skipped` warning and keeps the rest of the cue. The block index of a word is its index in its list. The block index of a sentence is its index inside its paragraph, so 2 warnings can both have block 0.

| Field | Error |
|:--- |:--- |
| a word `start` or `end` that is no time | `The field results.channels[0].alternatives[0].words[0].end must be a time.` |
| a word text that is no string | `The field results.channels[0].alternatives[0].words[0].punctuated_word must be a string.` |
| an utterance or sentence with a bad `start`, `end` or text | `The field results.utterances[0].end must be a time.` |

### Write

The output is compact JSON without a BOM. `results` has `channels`, then `utterances` when they are written, then the stored members. Then the other stored top-level members such as `metadata`.

- **Channels**: a cue goes to the channel in its stored data, when that is an integer below the number of stored channels plus the number of cues. Else it goes to channel 0 ([subtitle-toolbox-php#1274](https://github.com/yama6a/subtitle-toolbox-php/issues/1274)). The count is the larger of the stored channels and the highest cue channel plus 1, at least 1. The first channel gets `detected_language` from the subtitle language when it has none.
- **Alternative**: `transcript` is the cue texts of the channel joined with one space. Then the stored members, then `words` with the words of all its cues. `paragraphs` follows when a cue has a stored paragraph.
- **Cue values**: `start`, `end` and the text keep the stored value when it equals the cue. Else the writer writes the cue value.
- **Words**: the writer applies the rules of [reuse of source words](#reuse-of-source-words). No result gives no words. A new word is `{"word", "start", "end", "confidence", "speaker"?, "punctuated_word"}`. `word` is the text in lower case without the characters `.,?!;:"'()[]` and white space at both ends, or the text when nothing is left.
- **Typed values**: a reused word gets `Word.End` in `end` and `Word.Confidence` in `confidence`, when it has the member and the value differs.
- **Speaker**: the name of the cue. A name of the digits 0 to 9 only becomes an integer, so `<v 0>` writes `0`. A name too large for an integer stays a string ([subtitle-toolbox-php#1274](https://github.com/yama6a/subtitle-toolbox-php/issues/1274)). When it differs from the stored `speaker`, each word gets it.
- **Utterance**: written when a cue has a stored `transcript`, or when a cue has no words and no paragraph. It is `{"start": null, "end": null, "confidence": 1.0}` replaced by the stored utterance, then `start`, `end`, `channel`, `transcript`, `words` and `speaker`.
- **Paragraphs**: a cue with a stored paragraph joins the paragraph before it when the stored paragraphs are equal. A cue without one joins it when the speaker is equal. Each sentence is the stored sentence members with `text`, `start` and `end`. A paragraph gets `num_words`, `start` and `end`. `transcript` is one line for each paragraph with `Speaker N: ` before it when it has a speaker, and starts with a line break as Deepgram writes it.

A subtitle with the language `de` and the cue `<v 1>Guten` / `Morgen.` from 1.5 to 3 s writes:

```json
{"results":{"channels":[{"alternatives":[{"transcript":"Guten Morgen.","words":[]}],"detected_language":"de"}],"utterances":[{"start":1.5,"end":3.0,"confidence":1.0,"channel":0,"transcript":"Guten Morgen.","words":[],"speaker":1}]}}
```

### Cost

| Operation | Cost | Benchmark |
|:--- |:--- |:--- |
| read Deepgram JSON | linear in the file. The sentences of a channel take their words with one pass over the words | `BenchmarkDeepgram`: 9,000 words in one channel, 9,000 words in one utterance, 9,000 sentences in one paragraph. Each input is about 1 MB |
| write Deepgram JSON | linear in the cues, the words and the stored words | the same inputs |
