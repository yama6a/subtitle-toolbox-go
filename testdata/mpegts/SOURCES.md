# Sources

Copied from https://github.com/yama6a/subtitle-toolbox-php/tree/c0adecd5375820d5313f1583fc7e67942d514600/tests/files/mpegts, MIT. In the rows below, "this repository" means subtitle-toolbox-php. A PHP script wrote both files with a small packet writer that follows ISO/IEC 13818-1, ETSI EN 300 472 and ETS 300 706.

ffmpeg 7.0.2 with `libzvbi_teletextdec` reads the same text from each page, with `-txt_format ass -txt_page N`. The one difference is page 802. libzvbi takes the Latin set of Western Europe, because the file sends no X/28 or M/29 packet. So it shows `$` and `]` where the reader shows `ń` and `Ł` from the Polish language of the PMT.

| File | Source | License |
|:--- |:--- |:--- |
| `teletext.ts` | Written for this repository. A PAT, a PMT, a video stream that starts 0.5 s before the teletext stream, and a teletext stream with the pages 888 and 777 in German, 801, 802 and 301. Double height rows, boxes, colors, a corrected Hamming error, a page update without the erase flag, a repeated page, parallel magazines, X/28 with the Russian Cyrillic set, M/29 with the Greek set, and the initial page 100 | MIT |
| `one_page.ts` | Written for this repository. A video stream and the English teletext page 888 | MIT |
| `teletext_charsets.txt` | Written for this Go repository. The expected text of the pages 801, 802 and 301 of `teletext.ts`, one page per line: the Russian Cyrillic, Polish and Greek sets | MIT |
