# Video container subtitle tracks

A video file holds subtitle tracks next to the video and audio. `stbox.Tracks` lists the subtitle tracks of a file. `stbox.Load` and `stbox.Parse` read one track with `ReadOptions.Track`. The library reads only the bytes that it needs, so a 4 GB film never enters memory.

This page covers MKV and WebM. The MP4, MPEG-TS, DVB and ARIB sections come with their readers.

```text
tracks, err := stbox.Tracks(f, size)          // 2 S_TEXT/UTF8 en English, 3 S_TEXT/ASS de
s, info, err := stbox.Load("movie.mkv", stbox.ReadOptions{Track: 2})
```

`stbox.Load("movie.mkv", stbox.ReadOptions{})` reads the only subtitle track. A file with no subtitle track or with 2 or more returns a `*stbox.TrackError`, see [errors.md](errors.md). The package `container/mkv` holds the reader. It returns `model.Track` values, which the root package re-exports as `stbox.Track`.

## MKV and WebM

| Codec | `Track.Format` | Becomes |
|:--- |:--- |:--- |
| `S_TEXT/UTF8` | `srt` | SubRip cues |
| `S_TEXT/ASS`, `S_TEXT/SSA` | `ass` | an ASS or SSA subtitle, as `mkvextract` writes it |
| `S_TEXT/WEBVTT` | `vtt` | a WebVTT subtitle with its header, cue settings, identifiers and comments |
| `D_WEBVTT/SUBTITLES`, `D_WEBVTT/CAPTIONS`, `D_WEBVTT/DESCRIPTIONS` | `vtt` | a WebVTT subtitle with cue settings and identifiers, as ffmpeg writes WebVTT into WebM. `D_WEBVTT/METADATA` holds no text to display |

Any other codec has the format "". `Load` and `Extract` return a `*model.ParseError` for it. PGS (`S_HDMV/PGS`), VobSub (`S_VOBSUB`) and DVB (`S_DVBSUB`) tracks are listed with the format "" until their readers land.

- **Tracks**: `Tracks` lists only tracks of type subtitle. The fields are `Number` (`TrackNumber`), `CodecID`, `Format`, `Language`, `Name` ("" without one), `Default` and `Forced`.
- **Track numbers**: the reader skips a subtitle track whose `TrackNumber` is above 2147483647, because `Track.Number` is an `int` that every platform holds.
- **Detection**: `Load` and `Parse` know an MKV or WebM file by its first 4 bytes `1A 45 DF A3`, not by its extension.
- **Language**: the reader takes `LanguageBCP47`. Without it, the reader takes `Language`. Without either, the language is `eng`, as the Matroska specification defines. The read puts it into the `language` metadata.
- **Forced**: on a track with the forced flag, the read sets `Cue.Forced` on every cue.
- **Rebuilt files**: the reader rebuilds a SubRip, ASS or WebVTT file from the blocks and reads it with the parser of that format. So the options `Lenient` and `LastCueDuration` apply as they do for a file, and a damaged rebuilt file returns the error of its format.
- **End times**: a text block without `BlockDuration` ends after `DefaultDuration` when the track has one. Else it ends at the start of the next block of the track. The last such block lasts `ReadOptions.LastCueDuration`.
- **Compression**: the reader reads zlib compression and header stripping. It returns an error for bzlib, LZO and encrypted tracks. One block may inflate to 64 MB, and one track to 1 GiB in total.
- **Live recordings**: the reader accepts a Segment or a Cluster of unknown size.
- **Memory**: the reader skips video and audio data. A block of another track costs at most 11 bytes of reads. Memory grows with the subtitle track, not with the file.
- **Speed**: in a file from mkvmerge, the reader reads only the blocks that the `Cues` element lists for the track. It does so only when their number equals the `NUMBER_OF_FRAMES` tag that mkvmerge writes for the track. For a 2-hour, 1.3 GB film with 2,000 cues, this takes 0.24 s in place of 1.1 s. In other files, and when the numbers differ, the reader walks the whole file.
- **Time limit**: a block that starts or ends at 100,000 hours or more returns an error. This is the [time limit](formats.md) of the format readers.
- **Errors**: a damaged container returns a `*model.ParseError` with an empty `Format`. The CLI prints it as `stbox: <message>`. Lenient mode does not change container errors. It only applies to the rebuilt text file. Damage in these cases returns an error:
  - A codec without a reader.
  - Laced subtitle blocks.
  - An unsigned integer element, such as a cluster `Timestamp`, of more than 8 bytes or of 2^63 or more.
  - An element whose size reaches past the end of the file. The reader checks the size before it reads, so a damaged size cannot exhaust memory.
  - Zlib data that is not valid, that inflates to more than 64 MB in one block, or that inflates to more than 1 GiB in one track.
  - A file that is not Matroska or WebM.
- **Spec**: [Matroska elements](https://www.matroska.org/technical/elements.html), [Matroska subtitles](https://www.matroska.org/technical/subtitles.html), [EBML, RFC 8794](https://datatracker.ietf.org/doc/html/rfc8794).
