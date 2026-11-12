# Video container subtitle tracks

A video file holds subtitle tracks next to the video and audio. `stbox.Tracks` lists the subtitle tracks of a file. `stbox.Load` and `stbox.Parse` read one track with `ReadOptions.Track`. The library reads only the bytes that it needs, so a 4 GB film never enters memory.

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

## MPEG-TS and M2TS

The package `container/mpegts` reads DVB recordings as MPEG transport streams (`.ts`) and M2TS files from Blu-ray discs and Japanese recorders. It lists the teletext subtitle pages and reads one page as SubRip cues. DVB bitmap subtitles and ARIB captions come with their own readers.

```text
tracks, err := stbox.Tracks(f, size)          // 888 dvb_teletext deu, 777 dvb_teletext deu "hearing impaired"
s, info, err := stbox.Load("news.ts", stbox.ReadOptions{Track: 888})
```

- **Packet size**: a transport stream has packets of 188 bytes. An M2TS file has packets of 192 bytes with a 4-byte time stamp first. The reader knows them by the sync byte `0x47` at bytes 0, 188 and 376, or at bytes 4, 196 and 388. The head must be longer than 2 packets. Anything else returns `The file is not an MPEG-TS file.`
- **Table scan**: the reader finds the PAT and the PMT of each program in the first 33,554,432 bytes (32 MiB). A packet that starts at or after that offset ends the scan. A recording that starts at a packet has its tables much earlier. A file with a PMT further in has no track.
- **Tracks**: `Track.Format` is `srt` and `Track.CodecID` is `dvb_teletext`. A track is a page in a teletext descriptor (tag `0x56`, or tag `0x46` for VBI teletext) of a stream of type `0x06`. Teletext type 2 is a subtitle page. Type 5 is a subtitle page for hearing impaired people, with the name `hearing impaired`. Other types are skipped. `Default` and `Forced` are false. The language is the 3 letters of the entry in lower case, or `und` when they are not 3 letters of a to z. The tracks of all programs come in the order of the PMTs, each program in descriptor order.
- **Track numbers**: the number is the page number, for example 888 for magazine 8 and page `88`. Magazine 0 is 8. A page that an earlier track has takes `PID * 1000 + number`. Page 150 on PID 513 gives 513150. When that number is taken too, the page is skipped. `ReadOptions.Track` takes this number.
- **Descriptor limits**: go-astits reads the page byte as the decimal number `tens * 10 + units`. So a page byte with a low nibble above 9, such as `0x1A`, reads as page 20. A page above 99 is skipped. Of each descriptor, the reader takes the first `length / 5` entries and ignores a short last entry.
- **Times**: a cue time is the PTS of the page minus the zero point. The zero point is the smallest first PTS of all streams of the program, as FFmpeg counts it. For each other stream the reader takes the PTS of its first packet that starts a PES packet. Example: the video of `teletext.ts` starts 0.5 s before the teletext, so the first cue of page 888 starts at 0.5 s. A PTS keeps counting past the wrap of the 33-bit clock after 26.5 hours. A PES packet without a PTS takes the last PTS. A cue time of 100,000 hours or more returns an error, as in the other containers.
- **Cues**: each state of the page that shows text is one cue. It ends when the next state starts. The last state ends at the last PTS of the page stream, or after `ReadOptions.LastCueDuration` when no later packet follows. A state with the same lines as the one before it does not start a new cue. The `language` metadata is the track language.
- **Teletext text**: the reader keeps the rows 1 to 23 of the page. A page with the subtitle or newsflash flag shows only the text between a Start Box and an End Box code. A colour code other than white gives `<font color="#rrggbb">` around the run. `&`, `<` and `>` are escaped. Double height, flash, background colours and mosaics are ignored.
- **Character sets**: the reader knows Latin with 13 national option subsets, Cyrillic 1, 2 and 3, and Greek. The set comes from, in this order, packet X/28 of the page, packet M/29 of the magazine, and the language of the track. The language `pol` gives the Polish set, `rus` and `ukr` the Cyrillic sets, and `gre` the Greek set, see the group table in ETS 300 706 table 33. A page in the Arabic or Hebrew set returns the error `Teletext page 888 uses the Arabic or Hebrew character set. The reader reads Latin, Cyrillic and Greek pages.`
- **Not read**: packet X/26 (single accented characters), the G2 set, X/27 links and page enhancement. `ReadOptions.Lenient` has no effect, and the reader adds no warning.
- **Errors**: a damaged container returns a `*model.ParseError` with an empty `Format`. A track number that the file does not have returns an `*model.InvalidArgumentError` with `The file has no subtitle track with the number N.` A read error of the `io.ReaderAt` returns `read mpegts: ` and the cause.

### Damaged files

The reader skips damage without a message. The result depends on the damage.

| Damage | Result |
|:--- |:--- |
| bytes between packets, such as 7 junk bytes | the reader skips to the next sync byte and reads the same cues as the clean file |
| a packet whose sync byte is damaged | the reader skips to the next sync byte, so it loses that packet. A PES packet that loses a packet is dropped whole |
| M2TS packets | the reader drops the time stamp and reads the same cues |
| a packet with the transport error flag | the reader drops it. A PES packet that spans 2 packets and loses one is dropped whole. PHP decodes the part that arrived |
| a continuity counter jump with no lost data | the reader keeps the data and reads the same cues |
| a packet that repeats the previous one of its PID with the same counter and the same payload, also inside a PES packet | the reader drops the repeat, as ISO/IEC 13818-1 section 2.4.3.3 says. A packet with the same counter and another payload is data, as after a discontinuity |
| a wrong CRC32 in the PAT or a PMT | the reader skips that table. A later copy of the table counts. Without one the file has no track |
| a PMT with a descriptor that go-astits cannot parse | the program has no tracks |
| an adaptation field that fills the packet, or a PES header length past the packet | the packet gives no data, or the data starts after the header length |
| a PES packet that does not start with `00 00 01` | the reader ignores it |

- **Spec**: ISO/IEC 13818-1 (packets, PAT, PMT, PES), [ETSI EN 300 468](https://www.etsi.org/deliver/etsi_en/300400_300499/300468/01.17.01_60/en_300468v011701p.pdf) (descriptors), [ETSI EN 300 472](https://www.etsi.org/deliver/etsi_en/300400_300499/300472/01.04.01_60/en_300472v010401p.pdf) (teletext in PES packets), [ETS 300 706](https://www.etsi.org/deliver/etsi_i_ets/300700_300799/300706/01_60/ets_300706e01p.pdf) (teletext).
