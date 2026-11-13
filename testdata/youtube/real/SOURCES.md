# Sources

The captions of real videos belong to their authors. So every file here is written for this repository in the exact shape that YouTube returns and yt-dlp saves with `--sub-format json3`, `srv3` or `srv1`. The text is new neutral sample text.

| File | Source | License |
|:--- |:--- |:--- |
| `manual.en.json3` | Written for this repository in the shape of uploaded captions as json3: `wireMagic`, empty `pens`, `wsWinStyles` and `wpWinPositions`, one segment per event, a line break inside a segment, and `&`, `<` and `>` as plain text | MIT |
| `auto.en.json3` | Written for this repository in the shape of automatic captions as json3: a window event with `id`, word segments with `tOffsetMs` and `acAsrConf`, `aAppend` events with a line break, and events that overlap the next event | MIT |
| `auto.en.srv3` | Written for this repository in the shape of automatic captions as srv3: the `ws` and `wp` head elements, a `w` window, `<s>` words with `t` and `ac`, and `<p a="1">` append paragraphs, one without `d`. Same text and times as `auto.en.json3` | MIT |
| `styled.en.srv3` | Written for this repository in the shape of styled uploaded captions as srv3: `pen` elements with `fc`, `b` and `i`, `wp` window positions with `ap`, `<s p>` spans, a paragraph with two lines, and the double-escaped apostrophe `&amp;#39;` | MIT |
| `blank-lines.en.srv3` | Written for this repository in the shape of srv3 XML that an editor saved with two empty lines and an indent before the XML declaration | MIT |
| `transcript.en.srv1` | Written for this repository in the shape of the `<transcript>` XML that yt-dlp saves as srv1 and youtube-transcript-api reads: one line, no final line break, `start` and `dur` in seconds, overlapping automatic captions and double-escaped entities | MIT |
| `missing-dur.en.srv1` | Written for this repository in the shape of the `<transcript>` XML of issue #393: a middle and a last `<text>` without `dur`, and one with `dur="0"` | MIT |
| `missing-d.en.srv2` | Written for this repository in the shape of srv2 `<timedtext>` XML with `t` and `d` in milliseconds: a middle and a last `<text>` without `d`, and one with `d="0"` | MIT |
| `missing-d.en.srv3` | Written for this repository in the shape of srv3 `<timedtext format="3">` XML with `t` and `d` in milliseconds: a middle and a last `<p>` without `d`, and one with `d="0"` | MIT |
| `expected/*.json` | Generated for this repository. Expected parse result of the file with the same name, written by `tests/files/generate-read-goldens.php` | MIT |
| `expected/*.to.*` | Generated for this repository. Expected output of the file with the same name in another format, written by `tests/files/generate-write-goldens.php` | MIT |
| `expected/auto.en.json3.out.json3`, `expected/manual.en.json3.out.json3` | Generated for this repository. Expected `YouTubeTimedTextFormatter` output for the file with the same name: the same JSON data and key order, without the indents of YouTube | MIT |
| `expected/blank-lines.en.srv3.out.srv3` | Generated for this repository. Expected `YouTubeTimedTextFormatter` output for `blank-lines.en.srv3`: the same XML without the empty lines and the indent before the XML declaration | MIT |
