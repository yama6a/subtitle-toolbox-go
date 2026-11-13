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
| YouTube timed text | `youtube.Read` | `youtube.Write` | [YouTube timed text](#youtube-timed-text) |

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
- **Text**: srv1, srv2 and srv3 decode the text once more with the HTML5 entity table, so `&amp;#39;` gives `'`. json3 text stays as it is. Each line of a segment is written with its style, see below.
- **Style**: a srv3 span takes the pen of its `p` attribute, else the pen of its paragraph. A json3 segment takes `pPenId`, else the pen of its event. The pen gives the color as `#rrggbb`, bold, italic and underline. Other pen fields are lost.
- **Alignment**: the window position of the caption gives an anchor point from 0 to 8, row by row from the top left. The cue gets the alignment of that point, and bottom center without a position.
- **Appends**: an append caption (`a="1"`, `aAppend` set) gives no cue. Its object goes to the list `appends` of the format data of the next caption that is not an append, and to `appends` of the file data after the last caption.
- **Ends**: a caption without an end ends at the first later start that is greater than its own, else it lasts the last cue duration. A caption in a window ends at most at the first later start of that window that is greater than its own. A window is the srv3 attribute `w` or the json3 field `wWinId`, and the integer 1 and the string `"1"` are 2 windows. Captions outside windows may overlap. The reader finds all ends in one backward pass.
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
| write | linear in the cues and the segments. A changed cue matches its segments to the stored ones with one list for each text and offset | `BenchmarkWrite` and `BenchmarkWriteChanged` with the same inputs |
