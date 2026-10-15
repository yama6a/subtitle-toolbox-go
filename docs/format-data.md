# Format data

An ASS file has a `[V4+ Styles]` section and a `Format:` line in `[Events]`. No field of the model holds them, but the ASS writer needs them to write the file back as it was. The ASS reader keeps them as **format data**: data of one format under the key `ass`, on the subtitle for the whole file and on each cue for its `Dialogue` line. The ASS writer reads them back. Other writers ignore them. Library JSON carries format data, so a trip through JSON keeps it.

```go
var s model.Subtitle
err := s.SetFormatData("microdvd", model.NewData(model.Field{Key: "frameRate", Value: 23.976}))
fmt.Println(err) // <nil>

err = s.SetFormatData("scc", model.NewData(model.Field{Key: "dropFrame", Value: "yes"}))
fmt.Println(err) // stbox: invalid argument: The field formatData.scc.dropFrame must be a boolean.
```

## The Data type

- **Values**: `model.Data` is an ordered object. A value is `nil`, `bool`, `int64`, `float64`, `string`, `[]any` or `*Data`. `SetFormatData` rejects any other Go type, such as `int`, anywhere in the tree.
- **Order**: an object keeps its key order, because writers depend on it. The ASS writer writes the `[Script Info]` keys in stored order.
- **Integers**: always `int64`, so a 32-bit build reads the same values.
- **Copies**: `SetFormatData` stores a deep copy, and `FormatData` returns one. A change to a returned `*Data` changes nothing until you pass it back.
- **Keys**: the `Format` value of the format that owns the data, such as `ass`, `vtt` or `microdvd`. CSV and TSV share `csv`. The cue key `image` (`ImageKey`) holds image cue data. `FormatDataKeys` returns the keys sorted.
- **Why one ordered type**: it keeps unknown fields and key order without a codec per format, and the PHP read goldens match it as they are. A typed struct per format would need a codec per format and lose unknown fields.

## Type checks

Data enters only through `SetFormatData`, which checks the fields that writers read. It returns an error that matches `ErrInvalidArgument`, and keeps the old data. An empty `*Data` removes the key.

- **Message**: `The field PATH must be TYPE.` The path starts with `formatData.KEY`. An object key adds `.key`, a list index adds `[i]`. An object key that is a canonical decimal integer, such as `12` or `-5` but not `012`, also gets the `[12]` form, as in PHP.
- **First problem**: the check walks the schema fields in table order and the values in stored order. It returns the first problem.
- **Free data**: empty data passes. A key without a row passes, such as `whisper` or `pac`. A field outside its row passes.
- **All lines**: the tables hold the keys of formats that later lines add, such as `dcp` and `stl`. So data from PHP or from a later Go line passes the same checks in 1.x.

### Type language

| Type | Accepts | TYPE in the message |
|:--- |:--- |:--- |
| `string`, `int`, `bool` | a string, an int64, a bool | `a string`, `an integer`, `a boolean` |
| `number` | an int64 or a finite float64 | `a finite number` |
| `scalar` | a string, int64, float64, bool or nil | `a string, a number, a boolean or null` |
| `ttiBlock` | a string of exactly 256 hex digits | `a TTI block of 256 hexadecimal digits` |
| `timeBase` | a string such as `1/1000`, both parts without a leading 0 | `a time base such as "1/1000"` |
| `range(min, max)` | an int64 from min to max | `an integer from MIN to MAX` |
| `oneOf(values)` | one of the strings, exact | `one of "a", "b"` |
| `list(T)` | a list or an object, each value of type T | `a list or an object` |
| `strings` | `list(string)` | |
| `names(T)` | an object whose keys are names, each value of type T. An empty list passes | `an object`. A number key gives `The field PATH must be an object whose keys are names, not numbers.` |
| `keys(names, T)` | an object with only these keys, each value of type T | `an object`. Another key gives `The field PATH must hold only the keys a, b, c.` |
| `object(fields)` | a list or an object with these fields. `!` marks a required field | `an object`. A missing required field gives `The field PATH.NAME is missing.` |
| `?T` | T, or nil | as T |

### File schema

| Key | Fields |
|:--- |:--- |
| `ass` | `sectionOrder` strings, `scriptInfoComments` strings, `scriptInfo` strings, `stylesSection` ?string, `styleFormat` ?strings, `styles` list(strings), `eventFormat` strings, `commentEvents` list(names(string)), `sections` list(strings) |
| `csv` | `delimiter` oneOf(`,`, `;`, tab), `!header` ?strings, `!roles` keys(identifier, start, end, duration, speaker, text; range(0, 999)), `!width` range(0, 1000), `timeFormat` oneOf(`seconds`, `hh:mm:ss.mmm`, `hh:mm:ss,mmm`, `hh:mm:ss:ff`, `milliseconds`), `frameRate` ?number, `quotedColumns` list(range(0, 999)) |
| `lrc` | `idTags` strings, `spacedWordTimes` bool |
| `microdvd` | `frameRate` number, `frameRateLine` bool |
| `mpsub` | the whole data is strings |
| `qttext` | `header` strings, `pairedTimestamps` bool |
| `sami` | `style` ?string, `class` ?string, `samiParam` ?string |
| `subviewer` | `header` strings, `style` ?string |
| `ttml` | `namespace` string, `namespaces` names(string), `head` string, `attributes` names(string), `body` names(string) |
| `vtt` | `header` string, `headerLines` strings, `regions` list(strings), `styles` strings |
| `dcp` | `dialect` oneOf(`interop`, `smpte`), `namespace` string, `prefix` string, `timeCodeRate` range(1, 1000), `attributes` names(string), `header` names(string), `loadFonts` list(object(attributes names(string), text string)), `font` names(string) |
| `itt` | `frameRate` string, `frameRateMultiplier` string |
| `mcc` | `version` oneOf(`1.0`, `2.0`), `frameRate` oneOf(`23.976`, `24`, `25`, `29.97`, `30`, `50`, `59.94`, `60`), `dropFrame` bool, `header` strings |
| `scc` | `dropFrame` bool |
| `spruce-stl` | `frameRate` number, `header` strings, `separator` string, `lineBreak` string |
| `stl` | `gsi` strings, `startOfProgrammeSubtracted` bool, `firstSubtitleNumber` ?int, `comments` list(object(!text string, !blocks list(ttiBlock))) |
| `ffmeta-chapters` | `tags` list(?string), `streams` list(strings) |

### Cue schema

| Key | Fields |
|:--- |:--- |
| `ass` | `fields` names(string), `text` string |
| `csv` | `columns` list(scalar) |
| `lrc` | `endLine` bool |
| `microdvd` | `lines` list(object(!codes string, !color ?string, !tags strings, otherCodes string)) |
| `qttext` | `source` strings |
| `sami` | `paragraphs` list(object(!attributes names(string), !html string)) |
| `srt` | `coordinates` object(!x1 int, !x2 int, !y1 int, !y2 int) |
| `ttml` | `attributes` names(string), `div` names(string) |
| `vtt` | `vertical`, `line`, `position`, `size`, `align`, `region`, each string |
| `image` | `png` string of raw PNG bytes, `x`, `y`, `width`, `height`, `screenWidth`, `screenHeight` int, `forced` bool |
| `dcp` | `subtitle` names(string), `texts` list(names(string)) |
| `spruce-stl` | `commands` strings |
| `stl` | `subtitleGroupNumber`, `cumulativeStatus`, `verticalPosition`, `justificationCode` int, `blocks` list(ttiBlock) |
| `ffmeta-chapters` | `timeBase` timeBase, `tags` strings |

- **Unchecked fields that writers read**: `alignment` on the `ass`, `vtt` and `ttml` cue data. `lines` on the `ass` cue data, the cue lines at read time, so the writer can tell whether the text changed. Writers read them with the accessors, such as `Int`, and treat a wrong type as missing.
- **Compatibility**: the keys and value types are API, see [compatibility.md](compatibility.md) C11. A minor release may add a checked field.

## An explicit alignment wins over a stored position

A reader stores the alignment it read as a numpad int in the field `alignment` of its cue data. A writer writes the stored exact position only while `Cue.AlignmentUnchanged(key)` holds: `alignment`, 2 when missing, equals `Cue.Alignment.Numpad()`.

```go
c := model.Cue{Alignment: model.TopLeft}
_ = c.SetFormatData("vtt", model.NewData(
	model.Field{Key: "line", Value: "0"},
	model.Field{Key: "position", Value: "30%"},
	model.Field{Key: "align", Value: "left"},
	model.Field{Key: "alignment", Value: int64(7)},
))
fmt.Println(c.AlignmentUnchanged("vtt")) // true

c.Alignment = model.BottomRight
fmt.Println(c.AlignmentUnchanged("vtt")) // false
```

| Format | Alignment unchanged | Alignment changed |
|:--- |:--- |:--- |
| WebVTT | writes the stored `line`, `position` and `align` settings | drops them and writes the settings of the new alignment |

## Defaults

An option wins over stored format data, and stored format data wins over the format default. Example: MicroDVD output takes `WriteOptions.MicroDVD.FrameRate`, else the file data `microdvd.frameRate`, else it returns an error.
