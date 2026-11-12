# Video containers

`stbox.Load`, `stbox.Parse` and `stbox.Tracks` read the subtitle tracks of video files. The [detection page](detection.md) says how the library finds a container. This page holds the rules of each container reader.

## MP4 and MOV

Package `container/mp4` reads MP4 and QuickTime MOV files. The root calls it for a file with `ftyp` at bytes 4 to 7. `ReadOptions.Track` is the track ID of the `tkhd` box. `ReadOptions.SCC` and `ReadOptions.MCC` pass on to `c608` and `c708` tracks.

The reader reads the first `moov` box, and then only the samples of the wanted track. A 4 GB video file costs the size of its `moov` box, which the reader holds in memory.

### Tracks

A track is a subtitle track when it has the boxes `tkhd`, `mdia/hdlr` and `mdia/mdhd`, and the handler type is `text`, `sbtl`, `subt` or `clcp`. Other tracks are not listed. When a box holds 2 children of one type, the first one counts.

| `Track` field | Source |
|:--- |:--- |
| `Number` | the track ID of `tkhd` |
| `CodecID` | the type of the first sample entry in `stsd`, such as `tx3g`. A type with bytes that are not printable ASCII is `0x` and 8 hex digits |
| `Format` | `srt` for `tx3g`, `scc` for `c608`, `mcc` for `c708`. An encrypted track and any other codec give `""` |
| `Language` | the `elng` string, else the 3 letter code of `mdhd`. A code of 0 or `0x7FFF` is `und` |
| `Name` | the first `name` box in `udta`. The box may hold plain text, version and flags before the text, or a QuickTime text list |
| `Default` | the enabled flag of `tkhd` |
| `Forced` | for `tx3g`: the display flag `0x80000000` of the sample entry |

A string stops at the first NUL byte. A string that is not valid UTF-8 becomes `""`.

### Codecs

| Codec | Reads as | Rules |
|:--- |:--- |:--- |
| `tx3g` | SubRip cues | A sample holds a 2-byte text length, the text and modifier boxes. The text is UTF-8 or UTF-16 with a byte order mark. The reader escapes `&`, `<` and `>`, then puts `<b>`, `<i>` and `<u>` around the runs of the first `styl` box. An empty sample is a gap and gives no cue. A forced track sets `Forced` on every cue |
| `c608` | SCC cues | The `cdat` boxes hold CEA-608 byte pairs of field 1. The reader ignores `cdt2` boxes, which hold field 2. `ReadOptions.SCC` picks the channel and the roll-up mode |
| `c708` | MCC cues | Each `ccdp` box is a SMPTE 334-2 caption distribution packet. The reader decodes CEA-708 service 1, else CEA-608 field 1. `ReadOptions.MCC` picks the service |

Other codecs such as `wvtt` and `stpp` fail with `Track 1 has the codec wvtt. The reader extracts only tx3g, c608, c708.` An encrypted track fails with `Track 1 has the codec tx3g and is encrypted.` A track is encrypted when its sample entry type is `encv`, `enca`, `enct` or `encs`, or when a `tx3g` entry has a `sinf` box.

### Times

The edit list moves the samples. The reader adds the length of the empty edits at the start, and subtracts the media time of the first edit after them. Later edits do not count. A sample that ends at or before 0 s is dropped. For example, `ffmpeg_c608.mov` has an empty edit of 1.396 s, so each sample starts 1396 ms later.

Each time is an exact fraction, rounded half away from zero to whole milliseconds. Caption frames round to whole nanoseconds first, as SCC and MCC frame times do.

A sample that ends at 100000 hours or later fails with `The sample of track 1 at byte 40 ends at 360000000 s. Times must be below 100000 hours.`

### Damage

- **Not MP4**: the file has no `ftyp` box at the start. The message is `The file is not an MP4 file.`
- **Box sizes**: a box that does not fit its parent fails with `The box "moov" at byte 28 does not fit into its parent box.` Size 0 means to the end of the file. Size 1 means that a 64-bit size follows.
- **No `moov`**: `The MP4 file has no moov box.`
- **Sample tables**: the reader checks each count against the size of the box or the file before it allocates. A file that claims 4294967295 samples fails at once.
- **Sample outside the file**: `A sample of track 1 at byte 1073741824 lies outside the file.`
- **mp4ff checks**: the module github.com/Eyevinn/mp4ff decodes the `moov` box and checks the exact size of each table box. A damaged table in any track, also a video track, fails the open with `The MP4 file is not valid:` and the message of mp4ff.

All of these are a `*ParseError` with no format and no line.

### Fragmented files

The reader does not read fragmented MP4 files, such as CMAF, DASH and HLS segments. A `moov` box with a `mvex` box, and a `moof` box before the `moov` box, fail with `The MP4 file is fragmented. The reader reads only samples in the moov box.`
