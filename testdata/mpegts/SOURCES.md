# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/mpegts, MIT. In the rows below, "this repository" means subtitle-toolbox-php. A PHP script wrote the teletext files with a small packet writer that follows ISO/IEC 13818-1, ETSI EN 300 472 and ETS 300 706.

ffmpeg 7.0.2 with `libzvbi_teletextdec` reads the same text from each page, with `-txt_format ass -txt_page N`. The one difference is page 802. libzvbi takes the Latin set of Western Europe, because the file sends no X/28 or M/29 packet. So it shows `$` and `]` where the reader shows `ń` and `Ł` from the Polish language of the PMT.

ffmpeg 8.1 with `libaribcaption` reads the same text from both ARIB files, `-map 0:s -c:s srt`, with the furigana as separate small text. Its times count from the first PES packet of the caption stream, not from the video.

| File | Source | License |
|:--- |:--- |:--- |
| `teletext.ts` | Written for this repository. A PAT, a PMT, a video stream that starts 0.5 s before the teletext stream, and a teletext stream with the pages 888 and 777 in German, 801, 802 and 301. Double height rows, boxes, colors, a corrected Hamming error, a page update without the erase flag, a repeated page, parallel magazines, X/28 with the Russian Cyrillic set, M/29 with the Greek set, and the initial page 100 | MIT |
| `one_page.ts` | Written for this repository. A video stream and the English teletext page 888 | MIT |
| `teletext_charsets.txt` | Written for this Go repository. The expected text of the pages 801, 802 and 301 of `teletext.ts`, one page per line: the Russian Cyrillic, Polish and Greek sets | MIT |
| `arib_japanese.ts` | Written for this repository by `generator/AribFixtures.php` of subtitle-toolbox-php. A video stream that starts 1 s before an ARIB caption stream in Japanese: kanji, kana, furigana, a yellow row, 3 known DRCS glyphs and 1 unknown one, middle and normal size alphanumerics, TIME and CS. The 3 known glyph patterns come from xqq/libaribcaption at `37dde0f`, `test/sample_data/include/sample_data.h`, MIT, see [THIRD_PARTY_NOTICES.md](../../THIRD_PARTY_NOTICES.md) | MIT |
| `arib_brazilian.m2ts` | Written for this repository by `generator/AribFixtures.php` of subtitle-toolbox-php. An M2TS file with 192-byte packets: a video stream and an ARIB caption stream in Portuguese with the Latin sets of ABNT NBR 15606-1 | MIT |
| `expected/arib_*` | Written for this repository by the library with `stbox convert`. Checked against ffmpeg 8.1 with libaribcaption | MIT |
| `arib_colors.txt` | Written for this Go repository. The 128 colors of STD-B24 table 9-1 as the PHP `AribCharacters::color()` gives them, one per line, `-` for transparent | MIT |
| `dvb_subtitles.ts` | Copied from the PHP repository. Encoded with ffmpeg 7.0.2 `-c:s dvbsub`. A gray 64x36 H.264 video, `pgs/shapes_1080p.sup` as a DVB subtitle stream on PID 257 in German with 8-bit pixel codes on a 1920x1080 display, and the `en` track of `vobsub/two-tracks-pal.idx` as a stream on PID 258 in English for hearing impaired people with 2-bit pixel codes on 720x576. ffmpeg ignores the PGS cropping, drops the last PGS cue and moves some VobSub times, so the goldens differ from the sources there | MIT |
| `expected/dvb_subtitles.ts.*` | Copied from the PHP repository, unchanged. The pixel goldens of the two DVB subtitle tracks | MIT |
