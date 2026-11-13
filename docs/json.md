# Library JSON

Library JSON is the JSON shape of this library. It holds every field of the model, the comments, the metadata and the format data included. So `stbox convert show.ass show.json` followed by `stbox convert show.json show.ass` gives back the ASS file. Format name `json`, extension `.json`, package [format/stboxjson](../format/stboxjson).

Library JSON is also the conformance format of the port. Each PHP read golden holds the library JSON of a parse result, and the Go result must match it.

The writer writes version 2 and the reader reads versions 1 and 2. Version 2 holds the speaker and the timed words of a cue as fields, not as tags in the lines.

```json
{
    "version": 2,
    "metadata": {"title": "Morning train"},
    "comments": [{"text": "Scene 2", "beforeCueIndex": 1}],
    "formatData": {"vtt": {"header": "Kind: captions"}},
    "cues": [
        {"start": 1.0, "end": 3.5, "lines": ["<i>The train leaves at nine.</i>"], "identifier": null, "alignment": 8, "forced": true, "speaker": "Anna", "words": [{"text": "The", "start": 1.0, "end": 1.2}, {"text": "train", "start": 1.2, "end": 1.8, "confidence": 0.93}], "formatData": {}}
    ]
}
```

## Shape

| Field | Type | Read | Write |
|:--- |:--- |:--- |:--- |
| `version` | integer | required, `1` or `2`. `1.0` fails | `2`, or `1` with `WriteOptions.Version: 1` |
| `metadata` | object of strings | optional | always, keys sorted |
| `comments` | list of `{text, beforeCueIndex}` | optional. `beforeCueIndex` is an integer of 0 or more | always |
| `formatData` | object of objects, by format key | optional | keys sorted. Left out with `OmitFormatData` |
| `cues` | list | required | in start order |
| `cues[].start`, `.end` | number | required. The decimal value rounds to whole milliseconds, half away from zero: `1.0005` gives 1001 ms | seconds with at least 1 fraction digit: `4.0`, `1.001` |
| `cues[].lines` | list of strings | required. Version 1: the reader cleans each line as `Cue.AddLine` does, then moves word timestamps and a whole-cue voice into `Words` and `Speaker` with `ExtractWordTags`. Version 2: the lines are as stored | core markup. Version 2 writes `Cue.Lines`, without word timestamp tags and without the whole-cue voice tag. Version 1 writes `Cue.TaggedLines()`, with both, and its vtt cue data leaves out `voiceClass` and `voiceEnd`, because the tags hold them |
| `cues[].identifier` | string or null | optional | always. `null` for no identifier |
| `cues[].alignment` | integer 1 to 9, or null | optional. Missing and `null` mean 2 | `null` for bottom center, else the numpad value. `alignment` stays the only place of the alignment |
| `cues[].position` | object or null | optional. A bad value fails with its path, such as `The field cues[3].position.row is out of range.` The checks are those of `Position.Validate` | only when the cue has a row, a column or an offset, and the position is valid |
| `cues[].position.row`, `.column` | integer or null | optional. 1-based, 0 or missing means none | `row`, `column` and `grid` together, when `Row` or `Column` is set |
| `cues[].position.grid` | object `{rows, columns}` or null | optional. Each value is an integer | as above |
| `cues[].position.offset` | object `{x, y}` or null | optional. Both are numbers from 0 to 100, in percent of the video | only when set |
| `cues[].forced` | boolean | optional | only when true, and never for an image cue, which holds the flag in `formatData.image.forced` |
| `cues[].speaker` | string | version 2, optional. A missing key and `null` mean no speaker | version 2, only when not empty |
| `cues[].words` | list of objects | version 2, optional. A missing key and `null` mean no words | version 2, only when not empty |
| `cues[].words[].text` | string | required, not empty, no white space at either end | the word text |
| `cues[].words[].start`, `.end` | number | required, from 0 to below 100000 hours, `end` not before `start`. Rounded as `cues[].start` | seconds, as `cues[].start` |
| `cues[].words[].confidence` | number | optional, from 0 to 1. A missing key and `null` mean none | only when set |
| `cues[].formatData` | object of objects | optional | always. Left out with `OmitFormatData`. Version 2 keeps `voiceClass` and `voiceEnd` of the vtt cue data, because the lines hold no voice tag |

- **Tagged lines**: the WebVTT reader stores `formatData.vtt.taggedLines` for a cue whose word timestamp tags and voice tags stand where the placement rule of `Cue.TaggedLines` would not write them. Example: the YouTube line `the<00:05:04.199><c> train</c>`. Version 2 writes the key while the lines still match the cue. The version 2 reader restores the layout when the lines match the `lines`, `speaker`, voice form and word texts and starts of the cue. Else it drops the key and adds a `Repaired` warning, also in strict mode. So a round trip through version 2 gives the bytes of the file. Version 1 never writes the key. The layout also keeps a timestamp that belongs to no word, such as the trailing `<00:00:24.000>` of a line. The ASS reader stores its tagged lines in `formatData.ass.lines`, and the version 2 reader restores the layout from them the same way, without a warning when they no longer match.
- **Words**: the starts do not decrease from word to word. Each `text` occurs in the plain text of the cue, after the end of the word before it. `Cue.ValidateWords` holds the rules. The reader checks the order rule with one pass over the text of each cue.
- **Key order**: the writer uses the order of the table. The fields inside one format's data keep their stored order, because writers such as ASS depend on it.
- **Lists for objects**: `metadata`, `formatData`, the data of one format and `cues[].formatData` may be JSON lists, as PHP writes an empty or numbered array. A list reads as an object with the keys `"0"`, `"1"` and so on. `cues`, `lines` and `comments` must be JSON lists.
- **Numbers in format data**: a literal without `.`, `e` and `E` that fits an int64 reads as an integer. Every other literal reads as a float. The writer adds `.0` to a float without a fraction, so `4.0` stays a float and `4` an integer.
- **Times**: a time that rounds to 100000 hours or more fails, as in every reader. A time that rounds to -100000 hours or less fails too, because `time.Duration` cannot hold it. Negative times above that bound read and write as they are. A cue that ends before it starts fails, also with negative times, and lenient mode swaps it as in [lenient-parsing.md](lenient-parsing.md).
- **Errors**: a read error names the path of the bad field, such as `The field cues[3].start must be a number.` A bad word names its path, such as `The field cues[3].words[0].confidence must be a number from 0 to 1.` and `The field cues[3].words[2] is not in the text of the cue after the word before it.` Lenient mode skips a bad cue, metadata value, format data entry or comment. Invalid JSON and a bad `version`, `metadata`, `formatData`, `cues` or `comments` field fail in both modes.
- **Depth**: 511 lists and objects inside each other read. One more fails, as in PHP `json_decode`.

## Warnings

A lenient read gives one warning per skipped or repaired entry. A strict read gives the `Repaired` warning of a dropped `formatData.vtt.taggedLines`, and fails on the other problems.

| Field | Holds |
|:--- |:--- |
| `Line` | always `NoLine`. The messages name the path of the field |
| `Block` | the cue index for a cue, else `NoBlock` |
| `Lines` | one line: the compact JSON of the entry, such as `{"start":4,"lines":["It leaves from platform two."]}`, or `{"title":5}` for a metadata or format data entry |

- **Order**: a stable sort by `Block`, where `NoBlock` counts as block 0, as PHP sorts them. So a metadata warning comes before the warnings of cue 1.
- **Numbers in `Lines`**: written as the writer writes them, so `1.0` stays `1.0`. A number outside the float64 range, such as `1e400`, keeps its text.

## Binary values

The writer writes each format data string that is not valid UTF-8 as `{"base64": "..."}`, at any depth. It uses the standard alphabet with padding. The PNG bytes of an image cue are the main case.

The key `formatData.image` of a cue maps to `Cue.Image`. It holds `png`, `x`, `y`, `width`, `height`, `screenWidth`, `screenHeight` and `forced`. The reader sets `Cue.Forced` when `cues[].forced` or `image.forced` is true. The reader checks the image with `Image.Validate`, and an error names the field, for example `The field cues[3].formatData.image.width must be an integer.` An empty `image` object is no image, and the reader ignores keys of `image` that it does not know. With `OmitFormatData`, the image is not written.

The reader decodes each object whose only key is `base64` with a string value. It decodes as PHP `base64_decode` in strict mode does. So it accepts missing padding and skips spaces, tabs, CR and LF.

Text outside format data must be valid UTF-8. That text is the cue lines and identifiers, the metadata keys and values, the comments and the format data keys. The writer returns an `*UnwritableError` for a string there that is not valid UTF-8. `encoding/json` would replace the bad bytes without an error, so the writer checks first.

## Version

`stboxjson.Version` is 2, the newest version. The reader reads versions 1 and 2. Any other `version` fails with `The field version must be 1 or 2.`, so a reader of 4.0 rejects version 3.

`WriteOptions.Version` picks the version to write. 0 and 2 write version 2. 1 writes version 1, with the word timestamps and the speaker as tags in the lines. Version 1 loses the end and the confidence of each word. The writer returns an `*InvalidArgumentError` for another value.

Versions 1 and 2 are API for the whole 4.x line, see [compatibility.md](compatibility.md#changes-a-minor-release-can-make): a minor release may add fields, and readers ignore unknown fields. [upgrade-4.0.md](upgrade-4.0.md) lists the change from version 1 to version 2.
