# Transcripts

Package `format/transcript` reads and writes the JSON and HTML transcripts of speech-to-text tools and podcasts. This page holds the settings and the rules that every transcript format shares. The format list below names the fields of each format.

## Read options

`ReadOptions.Transcript` holds `transcript.ReadOptions`. The zero value reads no word times and no speakers. All fields are bools, so the struct is comparable and has no `Validate` rule.

| Field | Zero value | Meaning | Read by |
|:--- |:--- |:--- |:--- |
| `WordTimestamps` | false: cues have no `Words` | fill `Cue.Words` from the word times of the source | the formats with word times, and joined Podcasting 2.0 cues |
| `SpeakerVoices` | false: `Cue.Speaker` stays empty | set `Cue.Speaker` from the speaker of the source | the formats with speakers. The Podcasting 2.0 and HTML transcripts always set it |
| `KeepSegments` | false: join one-word segments | one cue per segment | the Podcasting 2.0 transcript only |

`WriteOptions.Transcript` holds `transcript.WriteOptions`. It holds one nested struct for each format with write settings. So far that is `Podcast`, see [Podcasting 2.0 transcript JSON](#podcasting-20-transcript-json), and `HTML`, see [HTML transcript](#html-transcript).

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
| read Whisper JSON | linear in the file. A segment takes the top-level words with one pass over them. A word of whisper.cpp tokens joins its token texts once | `BenchmarkReadWhisper`: 12,000 words in one segment, 12,000 top-level words in 2,000 segments, 24,000 tokens in one word. Each input is about 1 MB |
| write Whisper JSON | linear in the cues, the words and the stored words | `BenchmarkWriteWhisper`: the first 2 inputs above |
| read Podcasting 2.0 JSON | linear in the file. One pass from the back finds the first later start of all segments that have no end | `BenchmarkReadPodcast`: 16,000 one-word segments joined and kept, and 16,000 segments without an end whose starts fall. Each input is about 1 MB |
| write Podcasting 2.0 JSON | linear in the cue text and the words | `BenchmarkWritePodcast`: one cue of 16,000 words, with and without `WordSegments` |

`TestWordgroupGrowth` runs with `STBOX_GROWTH=1`. It fails when 10 times the words cost more than 30 times the time.

## Formats

| Format | Read | Write | Page section |
|:--- |:--- |:--- |:--- |
| Google Speech-to-Text JSON | `ReadGoogleSpeech` | `WriteGoogleSpeech` | [Google Speech-to-Text JSON](#google-speech-to-text-json) |
| Whisper JSON | `ReadWhisper` | `WriteWhisper` | [Whisper JSON](#whisper-json) |
| YouTube timed text | `youtube.Read` | `youtube.Write` | [YouTube timed text](#youtube-timed-text) |
| Podcasting 2.0 transcript JSON | `ReadPodcast` | `WritePodcast` | [Podcasting 2.0 transcript JSON](#podcasting-20-transcript-json) |
| HTML transcript | `ReadHTML` | `WriteHTML` | [HTML transcript](#html-transcript) |

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

## Podcasting 2.0 transcript JSON

A podcast feed links a transcript per episode with the `<podcast:transcript>` tag, and Apple Podcasts, Podverse and Fountain read its JSON form. Example: `{"version": "1.0.0", "segments": [{"speaker": "Anna", "startTime": 0.5, "endTime": 0.75, "body": "The"}, {"speaker": "Anna", "startTime": 1, "endTime": 1.25, "body": "bakery"}]}`. Many tools write one segment per word, so the reader joins the one-word segments of a speaker into cues. The writer writes one segment per cue and speaker, or one segment per word for the word highlight of the apps. The format follows the [Podcasting 2.0 transcript spec](https://github.com/Podcastindex-org/podcast-namespace/blob/c0ff5caa3729610362ee93f8034454fa41f3c493/docs/examples/transcripts/transcripts.md).

| Item | Value |
|:--- |:--- |
| Format | `stbox.PodcastTranscript`, name `podcast-transcript`, extension `.json` |
| Functions | `transcript.ReadPodcast`, `WritePodcast`, `DetectPodcast` and `DetectPodcastJSON` |
| Read options | `WordTimestamps` and `KeepSegments`. `SpeakerVoices` has no effect, because the speaker always goes to `Cue.Speaker` |
| Write options | `transcript.PodcastWriteOptions` in `WriteOptions.Podcast`: `WordSegments` and `PrettyPrint` |
| Detection | [JSON check 2](detection.md#json-checks): `segments` is an array whose first item is an object with the keys `startTime` and `body`. `.json` belongs to the library JSON in `FormatFromPath`, so output needs `--to podcast-transcript` |

### Podcast read

- **Segments**: `segments` must be an array. Else the read fails with `The JSON has no "segments" list.` in both modes.
- **File data**: the top-level members except `segments`, such as `version`, under the key `podcast-transcript`.
- **Segment fields**: a field error fails the segment. Strict mode returns the error. Lenient mode skips the segment with a `Skipped` warning without a line, with the segment as block text. The checks run in the order of the table.

| Field | Rule | Error |
|:--- |:--- |:--- |
| the segment | an object | `The field segments[0] must be an object.` |
| `startTime` | a finite number of 0 or more. A string such as `"0:01"` fails | `The field segments[0].startTime must be a number.` |
| `endTime` | missing, null, or the same rule | `The field segments[0].endTime must be a number.` |
| `speaker`, `body` | missing, null, or a string | `The field segments[0].speaker must be a string.` |
| times | below 100000 hours after the rounding to whole milliseconds | `The field segments[0].startTime is not below 100000 hours.` |
| order | a segment with an end must not end before it starts. Lenient mode swaps the times when the segment then lasts 30 s or less | `The cue ends at 1.5 s, before it starts at 2 s.` |

- **Text**: the speaker is trimmed. In the body, each run of space, tab, CR and LF becomes one space, then the body is trimmed.
- **Missing end**: a segment without `endTime` ends at the first later start among the segments after it in the file. Without one, it ends at its start plus the last cue duration, 5 s by default. A segment with an empty body counts for this rule. Then the reader drops each segment with an empty body, without a warning.

**Joining.** Unless `KeepSegments` is set, a segment is a **word** when its body holds no space. A word joins the open group when 2 things hold. The first segment of the group has the same speaker. The gap from the end of the last segment of the group to the start of the word is below 1 s. After each segment the group is open when that segment is a word whose body does not end a sentence. A body ends a sentence when it ends with `.`, `?`, `!` or U+2026, not counting the closing characters `"`, `'`, U+201D, U+2019, `)` and `]` at its end. A segment that is not a word never joins and closes the group. With `KeepSegments` every segment is its own cue.

Example: the words `Hello` (Anna, 0 to 0.4 s), `there.` (Anna, 0.5 to 0.9 s), `Come`, `in!`, `Thanks` (Ben), `Sit`, `"down."` and `Tea` give the cues `Hello there.`, `Come in!`, `Thanks`, `Sit "down."` and `Tea`.

**Cue.** A group becomes one cue.

| Part | Value |
|:--- |:--- |
| Times | the start of the first segment and the end of the last segment. A group that ends before it starts, because the file lists segments out of order, ends at its start and gets a `Repaired` warning |
| Lines | the bodies, escaped, joined with one space |
| Speaker | the speaker of the first segment, when it is not empty |
| Words | with `WordTimestamps` and a group of 2 or more segments, one `Word` for each segment: the body, the segment start and the segment end. No confidence |
| Cue data | for a group of one segment with other members, those members under `podcast-transcript` |

### Podcast write

- **Pieces**: the writer splits the text of a cue at each speaker change, and with `WordSegments` at each word timestamp. A speaker change is a `<v Name>` tag or a `</v>` tag. A piece with an empty body gives no segment.
- **Without `WordSegments`**: each piece has the cue start and the cue end. So a cue with 2 speakers gives 2 segments with the full cue times.
- **With `WordSegments`**: a piece starts at its word start, clamped into the cue times. It ends at the start of the next piece, or at the cue end. A piece that starts at a word ends earlier when `Word.End` is earlier. It never ends before its start.
- **Segment**: `{"speaker"?, "startTime", "endTime", "body"}`. `speaker` is there only when the piece has one. A time below 0 counts as 0. `endTime` is never before `startTime`.
- **Other members**: without `WordSegments`, a cue that gives exactly one segment adds its stored `podcast-transcript` members after `body`.
- **Document**: `{"version": V, "segments": [...]}`, then the stored file members. `V` is the stored `version` when it is set and not null, else `"1.0.0"`.
- **Output**: compact JSON without a BOM. With `PrettyPrint` it has a 4-space indent and a final LF, which the line ending option turns into CR LF.

| Option | Zero value | Meaning |
|:--- |:--- |:--- |
| `WordSegments` | false: one segment for each cue and speaker | one segment for each word of `Cue.Words` |
| `PrettyPrint` | false: one compact line, no final LF | 4-space indent and a final LF |

Cue `<v Ben>Home &amp; bed.` and `<v Anna>Why?` from 3 to 5 s, with the word `bed.` at 4 s, writes `{"speaker":"Ben","startTime":3.0,"endTime":5.0,"body":"Home & bed."},{"speaker":"Anna","startTime":3.0,"endTime":5.0,"body":"Why?"}`. With `WordSegments` it writes `Home &` from 3 to 4 s, `bed.` from 4 to 4 s and `Why?` from 4 to 5 s.

## YouTube timed text

YouTube serves captions in 4 variants: json3, srv3, srv2 and srv1, which is also the transcript XML. yt-dlp and youtube-transcript-api download them. Example: the srv3 paragraph `<p t="1200" d="2300">Hello<s t="400"> world</s></p>` is a cue from 1.2 to 3.5 s, and the word `world` starts at 1.6 s. Automatic captions hold the time of every word, rolling windows and append events that only add a line break. The reader turns each caption into a cue. The writer writes the variant of the source and keeps the source bytes of every caption that the user did not edit.

| Item | Value |
|:--- |:--- |
| Format | `stbox.YouTubeTimedText`, name `youtube`, extensions `.srv3` (first), `.json3`, `.srv1` and `.ytt`. `.srv2` is not registered. YTSubConverter saves srv3 as `.ytt` |
| Package | `format/youtube`: `Read`, `Write`, `Detect` and `DetectJSON` |
| Read options | `youtube.ReadOptions.WordTimestamps`, in `stbox.ReadOptions.YouTube`. `LastCueDuration` applies to a last caption without an end |
| Write options | none. `youtube.WriteOptions` is empty |
| Detection | [JSON check 4](detection.md#json-checks) and [signature 18](detection.md#signatures) |
| Lossy | srv1 and srv2 have no styles, word times or positions. In every variant the colors other than the pen color, the fonts and the speakers are lost |

### YouTube read rules

The first character after the BOM and white space decides the variant. `{` is json3. Anything else is XML.

| Root | Variant | Times |
|:--- |:--- |:--- |
| `<transcript>` | `srv1` | `<text start="s" dur="s">` in seconds |
| `<timedtext>` with a `<body>` child | `srv3` | `<p t="ms" d="ms">` |
| `<timedtext>` without `<body>` | `srv2` | `<text t="ms" d="ms">` |
| another root | error | `The root element must be <timedtext> or <transcript>, not <NAME>.` |

- **XML text before the root**: strict mode returns `The file has text before the XML: "LINE".` with its line. Lenient mode skips it with a `Repaired` warning. XML that does not parse returns `The content is not well-formed XML.` in both modes.
- **Bad UTF-8**: the XML variants follow TTML, see [encodings.md](encodings.md). json3 reads each bad byte as U+FFFD.
- **Times**: a time attribute is a number or a numeric string, finite and 0 or more, and the cue time must be below 100000 hours. Else `The <p> element has no valid "t" attribute.` or `The time "T" is not below 100000 hours.` A json3 field must be a JSON number: `The field events[0].tStartMs must be a number.` or `The field events[0].tStartMs is not below 100000 hours.` The same rules hold for `dDurationMs`, `tOffsetMs`, `d` and the `t` of an `<s>` span.
- **Captions**: srv1 and srv2 give one caption per child element of the root, of any name. srv3 gives one per `<p>`. json3 gives one per event. A json3 event without `segs` and with an `id` is a window event, and a srv3 `<w>` is a window. Both go to the file format data and give no caption.
- **Text**: srv1, srv2 and srv3 decode the text once more with the HTML5 entity table, so `&amp;#39;` gives `'`. json3 text stays as it is. A json3 `utf8` that is no string reads as empty text. A comment or a processing instruction inside a `<p>` gives no text. Each line of a segment is written with its style, see below.
- **Style**: a srv3 span takes the pen of its `p` attribute, else the pen of its paragraph. A json3 segment takes `pPenId`, else the pen of its event. The pen gives the color as `#rrggbb`, bold, italic and underline. Other pen fields are lost.
- **Alignment**: the window position of the caption gives an anchor point from 0 to 8, row by row from the top left. The cue gets the alignment of that point, and bottom center without a position.
- **Appends**: an append caption (`a="1"`, `aAppend` set) gives no cue. Its object goes to the list `appends` of the format data of the next caption that is not an append, and to `appends` of the file data after the last caption.
- **Ends**: a json3 event without `dDurationMs` lasts 0 s. A caption without an end ends at the first later start that is greater than its own, else it lasts the last cue duration. A caption in a window ends at most at the first later start of that window that is greater than its own. A window is the srv3 attribute `w` or the json3 field `wWinId`, and the integer 1 and the string `"1"` are 2 windows. Captions outside windows may overlap. The reader finds all ends in one backward pass.
- **Word times**: with `WordTimestamps` and at least one segment with an offset in the caption, each segment with text gives a `Word`. It starts at the caption start plus the offset, or at the caption start for a segment without an offset. The end is the start of the next word, or the cue end. The words have no confidence, and `acAsrConf` stays in the format data.
- **Format data**: the file data `youtube` holds `format`, then the other top-level members of json3, or the head elements `pen`, `ws` and `wp` and the windows `w` of srv3, as lists of their attributes. The cue data holds the attributes of the element or the members of the event, and `segments` with the object of each segment. srv1 and srv2 keep `text` when the second decode changed it.

### YouTube write rules

The `format` field of the file data names the variant. A cue from another format gets `srv3`. A caller writes another variant by setting `format`, for example to `json3`.

- **srv1**: the declaration, `<transcript>`, one `<text start="s" dur="s">` per cue and `</transcript>`, on one line with no final LF. Times are seconds with up to 3 decimals, such as `4.4` and `6`.
- **srv2**: the declaration and LF, `<timedtext>`, one `<text t="ms" d="ms">` and LF per cue, `</timedtext>` and LF.
- **srv3**: the declaration and `<timedtext format="3">`, the head with the pens, window styles and window positions, then the body with the windows, the append paragraphs and the paragraphs.
- **json3**: compact JSON in the order of YouTube, without the indent. `wireMagic` first, then the stored members, the lists `pens`, `wsWinStyles` and `wpWinPositions`, then `events`. An empty list item is `{}`.
- **Times**: an unedited start keeps its stored text. A duration stays when a reader would read the same cue end again, and a duration that the source lacked stays missing. A moved cue moves its append captions by the same shift. The writer uses the constant `writerLastCueDuration` of 5 s to find that end, not `ReadOptions`.
- **Unchanged cue**: when the lines and the words are the ones that the reader builds from the stored segments, the writer writes the stored segments as they are.
- **Changed cue**: the segments come from the tagged lines. A word timestamp sets the offset of the next text. The tags `b`, `i`, `u` and `font` set the style. The white space at the end of a segment moves to the start of the next timed segment. A cue without word times and styles whose words equal the stored one-word segments in number keeps the stored offsets, and a changed word loses only its own fields. Else each new segment takes the fields of the next unused stored segment with the same text and offset.
- **Pens and positions**: a style takes the preferred pen when it has the style, else the first pen with the style, else a new pen. A plain style needs none. An alignment other than bottom center takes the first window position with that anchor point, else a new one: `ah` and `av` are 0, 50 or 100 from the anchor point. A new id is the largest numeric id plus 1.
- **Escapes**: XML text escapes `&`, `<` and `>`, and attribute values also `"`. The writer drops the characters that XML cannot hold. A text that is not valid UTF-8 returns an `*UnwritableError`, as in [TTML](formats.md#ttml). A segment is escaped once, where PHP escapes some twice.

### YouTube cost

| Operation | Cost | Benchmark |
|:--- |:--- |:--- |
| read | linear in the file. The ends come from one backward pass with one stack for all captions and one for each window | `BenchmarkRead` in `format/youtube`: 20,000 captions without a duration, 20,000 captions in 50 windows and 20,000 segments in one caption, as srv1, srv3 and json3. Each input is about 1 MB |
| write | linear in the cues and the segments. A changed cue matches its segments to the stored ones with one list for each text and offset | `BenchmarkWrite` and `BenchmarkWriteChanged` with the same inputs, and `BenchmarkWriteJoinedTokens`: one changed cue of about 2 MB with 160,000 text tokens |

## HTML transcript

The Podcasting 2.0 spec allows a transcript as simple HTML. A `<cite>` holds the speaker, a `<time>` holds the start and each `<p>` holds text. Example: `<cite>Anna:</cite>` `<time>0:05</time>` `<p>Hello<br>there.</p>`. Podcast hosts publish it next to the audio, and readers can show it on a web page.

| Item | Value |
|:--- |:--- |
| Format | `stbox.HTMLTranscript`, name `html`, extensions `.html` and `.htm` |
| Functions | `transcript.ReadHTML`, `WriteHTML` and `DetectHTML` |
| Read options | none. The speaker always goes to `Cue.Speaker`. The last cue duration applies |
| Write options | `transcript.HTMLWriteOptions` in `WriteOptions.HTML`: `ParagraphGap` |
| Detection | [signature 21](detection.md#signatures): `<cite` and `<time` anywhere. A transcript without speakers has no `<cite>`, so `Load` finds it only by its extension |

### HTML read

The reader has no HTML parser. It scans the content for `<cite>`, `<time>` and `<p>` elements anywhere, in any case. Other markup is ignored and no `<html>` element is needed.

- **Elements**: a start tag is `<`, the name, then `>` or white space and any characters up to `>`. The element ends at the first end tag of the same name after it: `</`, the name, optional white space and `>`. So `<pre>` and `<timeline>` are no elements, and `</p >` ends a `<p>`. A start tag without an end tag is no element, and the scan goes on one character later. The scan skips the inner markup of a match, so a `<p>` inside a `<cite>` is part of the `<cite>`.
- **Cost**: the scanner remembers where the next `>` is and, for each name, the position after which no end tag exists. So a file of 1 MB with `<p>` and no `</p>` costs one pass, not one pass for each tag.
- **Paragraphs**: a paragraph starts at the first element, at a `<cite>` when the paragraph has a speaker or lines, and at a `<time>` when it has a time or lines. Its line is the line of the element that starts it. A paragraph without lines gives no cue.
- **Text**: tags go, entities decode, and each run of space, tab, CR and LF becomes one space. The `<cite>` text loses its trailing colons, so `Anna :` gives `Anna`. A `<p>` splits at `<br>`, `<br/>` and `<br />`, and each non-empty part becomes one escaped line.
- **Times**: a cue starts at its `<time>` and ends at the first later start among the cues after it. The last cue lasts the last cue duration.

| Check | Error | Line |
|:--- |:--- |:--- |
| the paragraph has a time | `The paragraph has no <time>.` | the paragraph line |
| the time is `h:mm:ss`, `m:ss` or `mm:ss`, with an optional fraction after `.` or `,`. Seconds go up to 59, minutes have any digits | `The time "soon" is not valid.` | the line of the `<time>` |
| the time, rounded to milliseconds, is below 100000 hours | `The time "99999999999999999999:00:04" is not below 100000 hours.` | the line of the `<time>` |

A failure follows the strict and lenient rules of [lenient-parsing.md](lenient-parsing.md). Lenient mode skips the paragraph with a `Skipped` warning. The block is the paragraph index and the block text is the time text, or the first line when there is no time.

### HTML write

The writer takes the segments of the [Podcasting 2.0 write](#podcast-write) without word segments, one for each cue and speaker. It groups them into paragraphs. A paragraph starts at a speaker that differs from the speaker of the segment before, and at a gap of `ParagraphGap` or more between its start and the latest end so far. A missing speaker differs from every name.

Each paragraph is `<cite>NAME:</cite>`, when its first segment has a speaker, then `<time>CLOCK</time>` and `<p>BODIES</p>`, each on its own line. The bodies join with one space. Names and bodies are escaped. `CLOCK` is the paragraph start floored to whole seconds, a negative start as 0: `m:ss` below 1 hour and `h:mm:ss` from 1 hour. 9 s gives `0:09` and 3725 s gives `1:02:05`. The output has no `<html>` element, no `<br>` and no BOM by default.

| Option | Zero value | Meaning |
|:--- |:--- |:--- |
| `ParagraphGap` | nil: 2 s | a gap of this length or more starts a new paragraph. 0 starts one at every segment. `transcript.NoParagraphBreaks` never starts one. A negative gap fails with `The paragraph gap must be 0 or more seconds, got -1.` |

The cues `<v Anna><b>Hello</b>` and `there.` (1 to 2 s), `<v Anna>Where are you going?` (2.5 to 4 s), `<v Ben>Home &amp; bed.` and `<v Anna>Why?` (4 to 5 s), `<v Anna>Fish &lt;3` (9 to 10 s) and `No speaker.` (3725 to 3727 s) write:

```html
<cite>Anna:</cite>
<time>0:01</time>
<p>Hello there. Where are you going?</p>
<cite>Ben:</cite>
<time>0:04</time>
<p>Home &amp; bed.</p>
<cite>Anna:</cite>
<time>0:04</time>
<p>Why?</p>
<cite>Anna:</cite>
<time>0:09</time>
<p>Fish &lt;3</p>
<time>1:02:05</time>
<p>No speaker.</p>
```

With a gap of 0.5 s, `Hello there.` and `Where are you going?` become 2 paragraphs, at `0:01` and `0:02`.

**Round trip.** Fractions of seconds, line breaks, styles and end times are lost. Writer output read again and written again gives the same bytes. A transcript without speakers writes no `<cite>`, so `Detect` does not find it. `Load` falls back to the extension `.html` or `.htm`. `Parse` needs `ReadOptions.Format` set to `html`.

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
