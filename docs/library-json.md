# Library JSON

Library JSON is the JSON shape of this library. It holds every field of the model, the comments, the metadata and the format data included. So `stbox convert show.ass show.json` followed by `stbox convert show.json show.ass` gives back the ASS file. Format name `json`, extension `.json`, package [format/stboxjson](../format/stboxjson).

Library JSON is also the conformance format of the port. Each PHP read golden holds the library JSON of a parse result, and the Go result must match it.

```json
{
    "version": 1,
    "metadata": {"title": "Morning train"},
    "comments": [{"text": "Scene 2", "beforeCueIndex": 1}],
    "formatData": {"vtt": {"header": "Kind: captions"}},
    "cues": [
        {"start": 1.0, "end": 3.5, "lines": ["<i>The train leaves at nine.</i>"], "identifier": null, "alignment": 8, "forced": true, "formatData": {}}
    ]
}
```

## Shape

| Field | Type | Read | Write |
|:--- |:--- |:--- |:--- |
| `version` | integer | required, exactly `1`. `1.0` fails | always `1` |
| `metadata` | object of strings | optional | always, keys sorted |
| `comments` | list of `{text, beforeCueIndex}` | optional. `beforeCueIndex` is an integer of 0 or more | always |
| `formatData` | object of objects, by format key | optional | keys sorted. Left out with `OmitFormatData` |
| `cues` | list | required | in start order |
| `cues[].start`, `.end` | number | required. The decimal value rounds to whole milliseconds, half away from zero: `1.0005` gives 1001 ms | seconds with at least 1 fraction digit: `4.0`, `1.001` |
| `cues[].lines` | list of strings | required. The reader cleans each line as `Cue.AddLine` does | core markup |
| `cues[].identifier` | string or null | optional | always. `null` for no identifier |
| `cues[].alignment` | integer 1 to 9, or null | optional. Missing and `null` mean 2 | `null` for bottom center, else the numpad value |
| `cues[].forced` | boolean | optional | only when true |
| `cues[].formatData` | object of objects | optional | always. Left out with `OmitFormatData` |

- **Key order**: the writer uses the order of the table. The fields inside one format's data keep their stored order, because writers such as ASS depend on it.
- **Lists for objects**: `metadata`, `formatData`, the data of one format and `cues[].formatData` may be JSON lists, as PHP writes an empty or numbered array. A list reads as an object with the keys `"0"`, `"1"` and so on. `cues`, `lines` and `comments` must be JSON lists.
- **Numbers in format data**: a literal without `.`, `e` and `E` that fits an int64 reads as an integer. Every other literal reads as a float. The writer adds `.0` to a float without a fraction, so `4.0` stays a float and `4` an integer.
- **Times**: a time of 100000 hours or more fails, as in every reader. A time of -100000 hours or less fails too, because `time.Duration` cannot hold it. Negative times above that bound read and write as they are.
- **Errors**: a read error names the path of the bad field, such as `The field cues[3].start must be a number.` Lenient mode skips a bad cue, metadata value, format data entry or comment. Invalid JSON and a bad `version`, `metadata`, `formatData`, `cues` or `comments` field fail in both modes.
- **Depth**: 511 lists and objects inside each other read. One more fails, as in PHP `json_decode`.

## Warnings

A lenient read gives one warning per skipped or repaired entry.

| Field | Holds |
|:--- |:--- |
| `Line` | always `NoLine`. The messages name the path of the field |
| `Block` | the cue index for a cue, else `NoBlock` |
| `Lines` | one line: the compact JSON of the entry, such as `{"start":4,"lines":["It leaves from platform two."]}`, or `{"title":5}` for a metadata or format data entry |

- **Order**: a stable sort by `Block`, where `NoBlock` counts as block 0, as PHP sorts them. So a metadata warning comes before the warnings of cue 1.
- **Numbers in `Lines`**: written as the writer writes them, so `1.0` stays `1.0`. A number outside the float64 range, such as `1e400`, keeps its text.

## Binary values

The writer writes each format data string that is not valid UTF-8 as `{"base64": "..."}`, with the standard alphabet and padding, at any depth. The PNG bytes of an image cue are the main case. The reader decodes each object whose only key is `base64` with a string value, as PHP `base64_decode` in strict mode does: it accepts missing padding and skips spaces, tabs, CR and LF.

Text outside format data must be valid UTF-8. The writer returns an `*UnwritableError` for a cue line, an identifier, a metadata key or value, a comment or a format data key that is not. `encoding/json` would replace the bad bytes without an error, so the writer checks first.

## Version

This package reads and writes version 1, the constant `stboxjson.Version`. Version 1 is API for the whole 1.x line, see [compatibility.md](compatibility.md) C11: a minor release may add fields, and readers ignore unknown fields. Version 2 arrives with typed words and speakers in 4.0 ([#92](https://github.com/yama6a/subtitle-toolbox-go/issues/92)). The 4.x reader still reads version 1.
