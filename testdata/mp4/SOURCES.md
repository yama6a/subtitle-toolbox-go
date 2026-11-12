# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/mp4, MIT. In the rows below, "this repository" means subtitle-toolbox-php. The generator `generator/Mp4Fixtures.php` stays there. `ffmpeg_mov_text.mp4` comes from ffmpeg 7.0.2, as the files of `tests/files/mkv/` in subtitle-toolbox-php do. `ffmpeg_c608.mov` and `ffmpeg_c608_rollup.mov` come from the same ffmpeg 7.0.2 build, with the commands in the table. The read golden `expected/ffmpeg_mov_text.mp4.2.srt` holds the extracted track.

A PHP script wrote the other files with a small box writer that follows ISO/IEC 14496-12 and 3GPP TS 26.245.

| File | Source | License |
|:--- |:--- |:--- |
| `text_tracks.mp4` | Written for this repository. The moov box follows the mdat box. A video track and four subtitle tracks: `tx3g` in English with a name, gap samples, a `styl` box, UTF-16 text and CR LF, 2 samples per chunk. `tx3g` with `elng` `fr-CA`, the forced display flag, version 1 `tkhd` and `mdhd`, `co64` and `stz2`. A `c608` track with a `cdat` and a `cdt2` box and an edit list with an empty edit and a media time. An `enct` track with a `sinf` box | MIT |
| `one_track.mp4` | Written for this repository. The moov box comes first. A video track and one `tx3g` track | MIT |
| `ffmpeg_mov_text.mp4` | Muxed with `ffmpeg -c:s mov_text` from `tests/files/mkv/muxed/harbour.srt` of subtitle-toolbox-php. The moov box follows the mdat box. An H.264 track and a `tx3g` track with the handler `sbtl`, a font table with Arial, a `styl` box for italics and an empty sample for each gap | MIT |
| `ffmpeg_c608.mov` | Muxed with `ffmpeg -i captionmaker_popon_df.scc -c:s copy -fflags +bitexact -map_metadata -1` from `testdata/scc/real/`. A `c608` track with the handler `clcp`, one `cdat` box per SCC line, and an empty edit of 1.396 s before the first sample | MIT |
| `ffmpeg_c608_rollup.mov` | Muxed with `ffmpeg -i rollup_news_ndf.scc -c:s copy -fflags +bitexact` from `testdata/scc/real/`. Roll-up captions in a `c608` track | MIT |
| `expected/*` | Written by the library from the ffmpeg files. `ffmpeg_c608.mov.1.scc` is the `c608` track written as SCC | MIT |
| `c708_track.mov` | Written for this repository. A `c708` track at 29.97 fps with one `ccdp` box per sample, from the caption distribution packets of `testdata/mcc/real/captionmaker_608_708.mcc`, and an empty edit of 1001 ms before the first sample | MIT |
| `huge_sample_count.mp4` | Written for this repository. A copy of `one_track.mp4` with 3 changed fields in track 2: the `stsz` sample size is 1 and its sample count is 4294967295, and the first `stts` and `stsc` entries count 4294967295 samples. A reader must reject it at once | MIT |
