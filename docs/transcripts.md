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

**Decision D1: the typed values win.** A reused source word is written with the `End` and `Confidence` of its entry in `Cue.Words`, when they differ from the stored values. Equal values keep the stored text, so an unedited round trip stays the same. A run of 2 or more source words stays unchanged.

**Decision D2: a moved word keeps its confidence.** A new word whose start equals the start of a source word of the cue counts as edited and gets confidence 1.0. Any other new word gets the confidence of its entry in `Cue.Words`, and 1.0 when that is nil. A shifted file keeps its confidences, and a corrected word does not keep the confidence of the wrong word.

## Cost

| Operation | Cost | Benchmark |
|:--- |:--- |:--- |
| group words into cues | linear in the number of words | `TestWordgroupGrowth` |
| fill `Cue.Words` of a cue | one text search per word, plus one more for each distinct word that the text lacks | `TestWordgroupGrowth` |
| reuse source words | linear in the text and the source words, plus one try for each source word that shares a start | `BenchmarkReuse`, 1 MB of text in one cue |
| read Whisper JSON | linear in the file. A segment takes the top-level words with one pass over them. A word of whisper.cpp tokens joins its token texts once | `BenchmarkReadWhisper`: 12,000 words in one segment, 12,000 top-level words in 2,000 segments, 24,000 tokens in one word. Each input is about 1 MB |
| write Whisper JSON | linear in the cues, the words and the stored words | `BenchmarkWriteWhisper`: the first 2 inputs above |

`TestWordgroupGrowth` runs with `STBOX_GROWTH=1`. It fails when 10 times the words cost more than 30 times the time.

## Formats

| Format | Read | Write | Page section |
|:--- |:--- |:--- |:--- |
| Whisper JSON | `ReadWhisper` | `WriteWhisper` | [Whisper JSON](#whisper-json) |

## Whisper JSON

Whisper is the speech-to-text model of OpenAI. 6 tools write its result as JSON in 4 shapes. Example: `whisper lecture.mp3 --output_format json --word_timestamps True` writes `{"text": "...", "segments": [{"id": 0, "start": 0.0, "end": 2.2, "text": " The bakery opens at seven.", "words": [{"word": " The", "start": 0.0, "end": 0.24, "probability": 0.99}]}], "language": "en"}`. The reader makes one cue per segment. The writer writes the shape of the tool that wrote the source, and keeps every member that the cue did not change, so other tools still read the file.

| Item | Value |
|:--- |:--- |
| Format | `stbox.Whisper`, name `whisper`, extension `.json` |
| Functions | `transcript.ReadWhisper`, `WriteWhisper`, `DetectWhisper` and `DetectWhisperJSON` |
| Read options | `WordTimestamps` and `SpeakerVoices`. `KeepSegments` has no effect |
| Write options | none |
| Detection | [JSON check 3](detection.md#json-checks): `segments` or `transcription` is an array. `.json` belongs to the library JSON in `FormatFromPath`, so name the format with `--from whisper` or `ReadOptions.Format` when the content does not decide |

### Shapes

The reader stores the shape as the `format` member of the file format data `whisper`. It checks the rows in this order:

| `format` | Found when | Layout |
|:--- |:--- |:--- |
| `whisper.cpp` | `segments` is not an array, so `transcription` is | `{"systeminfo", "model", "params", "result": {"language"}, "transcription": [{"timestamps": {"from", "to"}, "offsets": {"from", "to"}, "text", "tokens"?}]}`. Offsets are milliseconds. `-ojf` adds `tokens` |
| `whisperx` | `word_segments` is not null | `{"segments": [{"start", "end", "text", "words": [{"word", "start", "end", "score", "speaker"}], "speaker"}], "word_segments"}` |
| `openai-api` | `task` is not null, or `words` is an array | `{"task", "language", "duration", "text", "words"?, "segments"}`. The words are at the top level |
| `openai-whisper` | every other file, also faster-whisper and whisper-ctranslate2 | `{"text", "segments": [{"id", "seek", "start", "end", "text", "tokens", "temperature", "avg_logprob", "compression_ratio", "no_speech_prob", "words"?}], "language"}` |

### Read

- **Lists**: `segments` that is an array takes the segment path. Else `transcription` that is an array takes the whisper.cpp path. Else the read fails with `The JSON has no "segments" or "transcription" list.` in both modes. A list must be a JSON array. An object with the keys `"0"` and `"1"` is no list ([subtitle-toolbox-php#1208](https://github.com/yama6a/subtitle-toolbox-php/issues/1208)).
- **Cues**: a segment whose trimmed text is empty gives no cue and no warning. A long segment stays one cue.
- **Language**: `language`, else `result.language`. A string is looked up in lower case ASCII in the 112 names of `whisper/tokenizer.py`. A hit gives the code, so `english` gives `en` and `castilian` gives `es`. A miss keeps the string.
- **File data**: `{"format": SHAPE}`, then every top-level member except `segments`, `transcription`, `words`, `word_segments`, `text` and `format`.
- **Cue data**: the whole segment object. With top-level words, the `words` member of the segment holds its own words, then the words that it took.
- **Speaker**: with `SpeakerVoices`, the `speaker` of the segment or the transcription item, when it is a string. WhisperX writes `SPEAKER_00` after diarization. whisper.cpp writes `0`, `1` or `?` with `-di`. A number gives no speaker.

A field error fails the segment. Strict mode returns the error. Lenient mode skips the segment with a `Skipped` warning without a line, with the segment as block text.

| Field | Error |
|:--- |:--- |
| a `start` or `end` that is no finite number, also a segment that is no object | `The field segments[0].start must be a number.` A numeric string is no number |
| a time below 0 | `The field segments[0].start must be a time of 0 or more.` |
| a time of 100000 hours or more after the rounding to whole milliseconds | `The field segments[0].start is not below 100000 hours.` |
| a `text` that is no string | `The field segments[0].text must be a string.` |
| the same checks for `offsets.from` and `offsets.to` of a whisper.cpp item, in milliseconds | `The field transcription[0].offsets.to must be a number.` |
| a word `start` of 100000 hours or more | `The field segments[0].words[1].start is not below 100000 hours.`, or `The field words[3].start ...` for a top-level word. It fails the whole segment |
| a token start of 100000 hours or more | `The field transcription[0].tokens[4].offsets.from is not below 100000 hours.` |

**Top-level words.** The OpenAI API lists the words of all segments in `words`. The last segment takes all the words that are left. Any other segment takes the next words while their middle is at most its end. The middle of a word is `(start + end) / 2` in seconds. A word without times has the middle minus infinity, so it goes to the current segment. A segment that fails before this step takes no words, so the next segment gets them.

**Words.** Each item of `words` gives one word for `Cue.Words`.

| `Word` field | Source |
|:--- |:--- |
| `Text` | `word`, trimmed. A value that is no string gives `""`, and the word gets no entry |
| `Start` | `start`, when it is a finite number of 0 or more. Else the word has no start and no entry |
| `End` | `end` under the same rule, else the start of the next word, else the cue end. An end of 100000 hours or more counts as no end |
| `Confidence` | `probability`, else `score`, when it is a number from 0 to 1 |

**whisper.cpp tokens.** With `-ojf`, each item has `tokens`. The reader skips a token without a string `text` and a token whose text starts with `[_`, such as `[_BEG_]`. A token whose text starts with a space starts a new word, as `should_split_on_word()` of whisper.cpp does. The token texts of a word join without a separator, then the word is trimmed. So ` mor` and `ning` give `morning`. The start is `offsets.from` of the first token and the end is `offsets.to` of the last token. whisper.cpp gives a probability per token and not per word, so the words have no confidence.

### Write

The shape is the `format` member of the file data when it names one of the 4 shapes. Else it is `openai-whisper`. The writer does not write `format`. Cue times below 0 count as 0. The output is compact JSON without a BOM.

| Shape | Members in order |
|:--- |:--- |
| `openai-whisper` | `text`: the segment texts joined. `segments`. Then the stored file members |
| `openai-api` | the stored file members, `text` with the joined texts trimmed, `words` with the words of all segments when there is one, `segments` without their `words` |
| `whisperx` | `segments`, `word_segments` with the words of all segments, then the stored file members |
| `whisper.cpp` | the stored file members, `transcription` |

- **Language**: the subtitle language becomes `language`, unless the file data has a `language` that is not null. Not in the whisper.cpp shape.
- **Segment**: the stored segment keeps its members in order. A segment without `id` gets the `id` of the segment before it plus 1, else 0. The id after 9223372036854775807 is the float `9.223372036854776e+18`, as in PHP. WhisperX segments get none. `start`, `end` and `text` keep the stored value when it equals the cue. Else the writer writes the cue value, with a space before the text unless the stored text is a string without one. A changed text removes `tokens`, because the writer cannot make token ids.
- **Words**: the writer applies the rules of [reuse of source words](#reuse-of-source-words). No result removes a stored `words` array. A stored `"words": null` stays. A new word is `{"word": " text", "start", "end", "probability"}` in openai-whisper, `{"word": "text", "start", "end"}` in openai-api and `{"word", "start", "end", "score", "speaker"}` in WhisperX, with `speaker` only when the cue has one.
- **Speaker**: a cue speaker that differs from the stored `speaker` sets `speaker`, and each word with a `speaker` that is not null gets the new name. A cue without a speaker keeps the stored speaker, so the writer cannot write the removal of a speaker.
- **whisper.cpp entry**: `offsets` is always `{"from": MS, "to": MS}` in whole milliseconds. `timestamps` keeps the stored value when the offsets are the same, else it is `{"from": "00:01:02,500", "to": ...}`. `text` follows the segment rule. `tokens` stay only when the offsets, the text and the words of the cue equal the stored ones. `speaker` follows the segment rule.

A subtitle from another format with the language `de` writes this, for a cue `<v Anna>Guten` and `Morgen.` from 1.5 to 3 s and a cue `Bis bald.` with 2 words:

```json
{"text":" Guten Morgen. Bis bald.","segments":[{"id":0,"start":1.5,"end":3.0,"text":" Guten Morgen.","speaker":"Anna"},{"id":1,"start":4.0,"end":5.0,"text":" Bis bald.","words":[{"word":" Bis","start":4.0,"end":4.5,"probability":1.0},{"word":" bald.","start":4.5,"end":5.0,"probability":1.0}]}],"language":"de"}
```
