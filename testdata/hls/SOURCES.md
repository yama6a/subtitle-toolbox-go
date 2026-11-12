# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/hls, MIT. In the rows below, "this repository" means subtitle-toolbox-php.

| File | Source | License |
|:--- |:--- |:--- |
| `node-webvtt-subs1.vtt` | [osk/node-webvtt `test/data/subs1.vtt`](https://github.com/osk/node-webvtt/blob/af2dbb541ae29f9ccada05db9b9250111cb4848c/test/data/subs1.vtt), cut to the first 30 cues. The cue texts are numbers. | MIT |
| `shaka-playlist-vtt.m3u8` | [shaka-player `hls-text-offset/playlist-vtt.m3u8`](https://github.com/shaka-project/shaka-player/blob/2238782f3c255aa5f40d682a2055da01a3b21649/test/test/assets/hls-text-offset/playlist-vtt.m3u8) | Apache-2.0 |
| `shaka-vtt-071.vtt`, `shaka-vtt-072.vtt` | [shaka-player `hls-text-offset/vtt-071.vtt` and `vtt-072.vtt`](https://github.com/shaka-project/shaka-player/tree/2238782f3c255aa5f40d682a2055da01a3b21649/test/test/assets/hls-text-offset). Segments without cues and without a final line break, with LOCAL before MPEGTS. | Apache-2.0 |
| `own-fileSequence[0-3].webvtt` | Written for this repository in the shape of Apple `mediasubtitlesegmenter` output: MPEGTS before LOCAL, cue settings, a cue repeated with its full times in both segments it overlaps, and an empty segment. | MIT |
| `own-prog_index.m3u8` | Written for this repository. The playlist of the four `own-fileSequence` segments, in the same shape. | MIT |
| `expected/node-webvtt-subs1/*` | Generated for this repository from `node-webvtt-subs1.vtt` (osk/node-webvtt, MIT) by `hls.Segment` with the segment duration 10 s, MPEGTS 126000, the pattern `part%03d.vtt` and the media duration 150 s. It equals the output of `stbox hls node-webvtt-subs1.vtt --segment 00:10 --mpegts 126000 --pattern part%03d.vtt --media-duration 00:02:30 --playlist index.m3u8`. The files are byte for byte the same as the files of the same name in subtitle-toolbox-php. | MIT |
| `expected/node-webvtt-subs1-local/*` | Generated like `expected/node-webvtt-subs1/*`, with the LOCAL time 10 s. It equals the output of the same command with `--local 10`. | MIT |
